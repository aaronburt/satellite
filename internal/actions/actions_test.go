package actions

import (
	"errors"
	"testing"
)

func TestRegistryExecution(t *testing.T) {
	reg := NewRegistry()

	called := false
	reg.Register("custom_action", func() error {
		called = true
		return nil
	})

	if err := reg.Execute("custom_action"); err != nil {
		t.Fatalf("unexpected error executing custom_action: %v", err)
	}
	if !called {
		t.Errorf("expected custom_action handler to have been called")
	}

	err := reg.Execute("non_existent_action")
	if !errors.Is(err, ErrActionNotFound) {
		t.Errorf("expected ErrActionNotFound, got %v", err)
	}
}

func TestExecutePayload(t *testing.T) {
	reg := NewRegistry()

	actionTriggered := ""
	reg.Register("display_sleep", func() error {
		actionTriggered = "display_sleep"
		return nil
	})
	reg.Register("lock", func() error {
		actionTriggered = "lock"
		return nil
	})

	if err := reg.ExecutePayload([]byte("display_sleep")); err != nil {
		t.Fatalf("unexpected error executing raw string payload: %v", err)
	}
	if actionTriggered != "display_sleep" {
		t.Errorf("expected display_sleep, got %s", actionTriggered)
	}

	jsonPayload := []byte(`{"action": "lock"}`)
	if err := reg.ExecutePayload(jsonPayload); err != nil {
		t.Fatalf("unexpected error executing json payload: %v", err)
	}
	if actionTriggered != "lock" {
		t.Errorf("expected lock, got %s", actionTriggered)
	}

	cmdPayload := []byte(`{"command": "display_sleep"}`)
	if err := reg.ExecutePayload(cmdPayload); err != nil {
		t.Fatalf("unexpected error executing command json payload: %v", err)
	}
	if actionTriggered != "display_sleep" {
		t.Errorf("expected display_sleep, got %s", actionTriggered)
	}

	if err := reg.ExecutePayload([]byte("")); err == nil {
		t.Errorf("expected error for empty payload")
	}
	if err := reg.ExecutePayload([]byte("   ")); err == nil {
		t.Errorf("expected error for whitespace payload")
	}

	rawFallbackPayload := []byte(`{"other": "field"}`)
	_ = reg.ExecutePayload(rawFallbackPayload)
}

func TestStatusProvider(t *testing.T) {
	reg := NewRegistry()
	if reg.getStatus() != "" {
		t.Errorf("expected empty string when no provider is set")
	}

	reg.SetStatusProvider(func() string {
		return "playing"
	})

	if reg.getStatus() != "playing" {
		t.Errorf("expected 'playing', got %q", reg.getStatus())
	}
}

func TestBuiltinActions(t *testing.T) {
	reg := NewRegistry()
	reg.SetStatusProvider(func() string {
		return "paused"
	})

	actionsToTest := []string{
		"media_play_pause",
		"media_play",
		"media_pause",
		"media_next",
		"media_prev",
		"media_stop",
		"volume_mute",
		"volume_up",
		"volume_down",
	}

	for _, act := range actionsToTest {
		if err := reg.Execute(act); err != nil {
			t.Errorf("unexpected error for action %s: %v", act, err)
		}
	}

	reg.SetStatusProvider(func() string {
		return "playing"
	})
	_ = reg.Execute("media_play")
	_ = reg.Execute("media_pause")

	reg.Register("lock", func() error { return nil })
	_ = reg.Execute("lock")
	_ = reg.Execute("lock_workstation")

	reg.Register("display_sleep", func() error { return nil })
	_ = reg.Execute("display_sleep")
}
