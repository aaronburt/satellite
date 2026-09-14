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
}

func TestStatusProvider(t *testing.T) {
	reg := NewRegistry()
	reg.SetStatusProvider(func() string {
		return "playing"
	})

	if reg.getStatus() != "playing" {
		t.Errorf("expected 'playing', got %q", reg.getStatus())
	}
}
