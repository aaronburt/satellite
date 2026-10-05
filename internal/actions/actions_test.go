package actions

import (
	"errors"
	"sync"
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

type mockExecutor struct {
	mu     sync.Mutex
	calls  map[string]int
	failOn map[string]bool
}

func newMockExecutor() *mockExecutor {
	return &mockExecutor{
		calls:  make(map[string]int),
		failOn: make(map[string]bool),
	}
}

func (m *mockExecutor) inc(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls[name]++
	if m.failOn[name] {
		return errors.New("mock failure")
	}
	return nil
}

func (m *mockExecutor) getCalls(name string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls[name]
}

func (m *mockExecutor) LockWorkstation() error { return m.inc("lock") }
func (m *mockExecutor) TogglePlayPause() error { return m.inc("play_pause") }
func (m *mockExecutor) NextTrack() error       { return m.inc("next") }
func (m *mockExecutor) PreviousTrack() error   { return m.inc("prev") }
func (m *mockExecutor) Stop() error            { return m.inc("stop") }
func (m *mockExecutor) ToggleMute() error      { return m.inc("mute") }
func (m *mockExecutor) VolumeUp() error        { return m.inc("vol_up") }
func (m *mockExecutor) VolumeDown() error      { return m.inc("vol_down") }
func (m *mockExecutor) SleepDisplays() error   { return m.inc("display_sleep") }

func TestBuiltinActions(t *testing.T) {
	mock := newMockExecutor()
	reg := NewRegistryWithExecutor(mock)
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
		"lock",
		"lock_workstation",
		"display_sleep",
		"sleep_displays",
		"monitor_off",
	}

	for _, act := range actionsToTest {
		if err := reg.Execute(act); err != nil {
			t.Errorf("unexpected error for action %s: %v", act, err)
		}
	}

	if mock.getCalls("lock") != 2 {
		t.Errorf("expected 2 lock calls, got %d", mock.getCalls("lock"))
	}
	if mock.getCalls("display_sleep") != 3 {
		t.Errorf("expected 3 display_sleep calls, got %d", mock.getCalls("display_sleep"))
	}

	reg.SetStatusProvider(func() string {
		return "playing"
	})
	_ = reg.Execute("media_play")
	_ = reg.Execute("media_pause")

	mock.failOn["lock"] = true
	if err := reg.Execute("lock"); err == nil {
		t.Errorf("expected error when executor fails")
	}

	fallbackReg := NewRegistryWithExecutor(nil)
	if fallbackReg.getExecutor() == nil {
		t.Errorf("expected non-nil executor for fallback")
	}

	emptyReg := &Registry{}
	if emptyReg.getExecutor() == nil {
		t.Errorf("expected non-nil executor for empty struct")
	}
}
