package actions

import (
	"errors"
	"sync"
	"testing"
)

type mockExecutor struct {
	mu     sync.Mutex
	calls  []string
	failOn map[string]bool
}

func newMockExecutor() *mockExecutor {
	return &mockExecutor{failOn: make(map[string]bool)}
}

func (m *mockExecutor) record(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, name)
	if m.failOn[name] {
		return errors.New("mock failure: " + name)
	}
	return nil
}

func (m *mockExecutor) Calls() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string(nil), m.calls...)
}

func (m *mockExecutor) LockWorkstation() error { return m.record("lock") }
func (m *mockExecutor) TogglePlayPause() error { return m.record("play_pause") }
func (m *mockExecutor) NextTrack() error       { return m.record("next") }
func (m *mockExecutor) PreviousTrack() error   { return m.record("prev") }
func (m *mockExecutor) Stop() error            { return m.record("stop") }
func (m *mockExecutor) ToggleMute() error      { return m.record("mute") }
func (m *mockExecutor) VolumeUp() error        { return m.record("vol_up") }
func (m *mockExecutor) VolumeDown() error      { return m.record("vol_down") }
func (m *mockExecutor) SleepDisplays() error   { return m.record("display_sleep") }

func equalCalls(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Every name Home Assistant (or a user's automation) may send must reach
// exactly one executor call. A broken alias silently does nothing in HA.
func TestExecuteMapsEveryAliasToExactlyOneExecutorCall(t *testing.T) {
	cases := map[string]string{
		"lock":             "lock",
		"lock_workstation": "lock",
		"media_play_pause": "play_pause",
		"play_pause":       "play_pause",
		"media_next":       "next",
		"next_track":       "next",
		"next":             "next",
		"media_prev":       "prev",
		"media_previous":   "prev",
		"previous_track":   "prev",
		"prev_track":       "prev",
		"prev":             "prev",
		"previous":         "prev",
		"media_stop":       "stop",
		"stop":             "stop",
		"volume_mute":      "mute",
		"mute":             "mute",
		"volume_up":        "vol_up",
		"volume_down":      "vol_down",
		"display_sleep":    "display_sleep",
		"sleep_displays":   "display_sleep",
		"monitor_off":      "display_sleep",
		// Case and surrounding whitespace are ignored.
		"  LOCK  ":    "lock",
		"Volume_Up\n": "vol_up",
	}
	for input, want := range cases {
		t.Run(input, func(t *testing.T) {
			mock := newMockExecutor()
			reg := NewRegistryWithExecutor(mock)
			if err := reg.Execute(input); err != nil {
				t.Fatalf("Execute(%q) error: %v", input, err)
			}
			if got := mock.Calls(); !equalCalls(got, []string{want}) {
				t.Fatalf("Execute(%q) calls = %v, want [%s]", input, got, want)
			}
		})
	}
}

// media_play and media_pause are idempotent: they only toggle when the
// player is not already in the requested state.
func TestPlayAndPauseRespectCurrentMediaStatus(t *testing.T) {
	cases := []struct {
		action     string
		status     string
		wantToggle bool
	}{
		{"media_play", "paused", true},
		{"media_play", "stopped", true},
		{"media_play", "", true},
		{"media_play", "playing", false},
		{"media_play", "Playing", false},
		{"play", "playing", false},
		{"media_pause", "playing", true},
		{"media_pause", "PLAYING", true},
		{"pause", "playing", true},
		{"media_pause", "paused", false},
		{"media_pause", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.action+"/"+tc.status, func(t *testing.T) {
			mock := newMockExecutor()
			reg := NewRegistryWithExecutor(mock)
			status := tc.status
			reg.SetStatusProvider(func() string { return status })
			if err := reg.Execute(tc.action); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			toggled := len(mock.Calls()) == 1 && mock.Calls()[0] == "play_pause"
			if toggled != tc.wantToggle {
				t.Fatalf("toggled = %v, want %v (calls %v)", toggled, tc.wantToggle, mock.Calls())
			}
		})
	}
}

func TestPlayWithoutStatusProviderToggles(t *testing.T) {
	mock := newMockExecutor()
	reg := NewRegistryWithExecutor(mock)
	if err := reg.Execute("media_play"); err != nil {
		t.Fatal(err)
	}
	if !equalCalls(mock.Calls(), []string{"play_pause"}) {
		t.Fatalf("calls = %v", mock.Calls())
	}
}

func TestExecuteUnknownActionReturnsErrActionNotFound(t *testing.T) {
	mock := newMockExecutor()
	reg := NewRegistryWithExecutor(mock)
	for _, name := range []string{"reboot", "shutdown", "", "lockk"} {
		err := reg.Execute(name)
		if !errors.Is(err, ErrActionNotFound) {
			t.Errorf("Execute(%q) = %v, want ErrActionNotFound", name, err)
		}
	}
	if calls := mock.Calls(); len(calls) != 0 {
		t.Fatalf("unknown actions must not touch the executor, got %v", calls)
	}
}

func TestExecutorErrorsArePropagated(t *testing.T) {
	mock := newMockExecutor()
	mock.failOn["lock"] = true
	reg := NewRegistryWithExecutor(mock)
	err := reg.Execute("lock")
	if err == nil || err.Error() != "mock failure: lock" {
		t.Fatalf("Execute error = %v, want executor error", err)
	}
}

func TestCustomHandlerOverridesBuiltinAndIsMatchedByAlias(t *testing.T) {
	mock := newMockExecutor()
	reg := NewRegistryWithExecutor(mock)
	called := 0
	reg.Register("lock_workstation", func() error { called++; return nil })

	if err := reg.Execute("LOCK"); err != nil {
		t.Fatal(err)
	}
	if called != 1 {
		t.Fatalf("custom handler called %d times, want 1", called)
	}
	if len(mock.Calls()) != 0 {
		t.Fatalf("builtin executor should not run when overridden, got %v", mock.Calls())
	}

	want := errors.New("custom failure")
	reg.Register("custom_action", func() error { return want })
	if err := reg.Execute("custom_action"); !errors.Is(err, want) {
		t.Fatalf("custom handler error = %v, want %v", err, want)
	}
}

func TestExecutePayloadFormats(t *testing.T) {
	cases := []struct {
		name    string
		payload string
		want    []string
	}{
		{"plain text", "volume_up", []string{"vol_up"}},
		{"plain text with whitespace", "  next \n", []string{"next"}},
		{"json action", `{"action":"lock"}`, []string{"lock"}},
		{"json command", `{"command":"mute"}`, []string{"mute"}},
		{"action wins over command", `{"action":"lock","command":"mute"}`, []string{"lock"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mock := newMockExecutor()
			reg := NewRegistryWithExecutor(mock)
			if err := reg.ExecutePayload([]byte(tc.payload)); err != nil {
				t.Fatalf("ExecutePayload(%q) error: %v", tc.payload, err)
			}
			if !equalCalls(mock.Calls(), tc.want) {
				t.Fatalf("calls = %v, want %v", mock.Calls(), tc.want)
			}
		})
	}
}

func TestExecutePayloadRejectsEmptyAndUnknown(t *testing.T) {
	mock := newMockExecutor()
	reg := NewRegistryWithExecutor(mock)

	for _, p := range []string{"", "   ", "\n\t"} {
		err := reg.ExecutePayload([]byte(p))
		if err == nil || errors.Is(err, ErrActionNotFound) {
			t.Errorf("ExecutePayload(%q) = %v, want empty-payload error", p, err)
		}
	}
	// JSON without an action falls back to treating the raw text as the
	// action name, which is not a known action.
	if err := reg.ExecutePayload([]byte(`{"other":"field"}`)); !errors.Is(err, ErrActionNotFound) {
		t.Errorf("JSON without action = %v, want ErrActionNotFound", err)
	}
	if len(mock.Calls()) != 0 {
		t.Fatalf("no executor calls expected, got %v", mock.Calls())
	}
}

func TestNilExecutorFallsBackToSystemExecutor(t *testing.T) {
	if _, ok := NewRegistryWithExecutor(nil).getExecutor().(systemExecutor); !ok {
		t.Fatal("nil executor should fall back to systemExecutor")
	}
	if _, ok := (&Registry{}).getExecutor().(systemExecutor); !ok {
		t.Fatal("zero Registry should fall back to systemExecutor")
	}
	if _, ok := NewRegistry().getExecutor().(systemExecutor); !ok {
		t.Fatal("NewRegistry should use systemExecutor")
	}
}

func TestRegistryIsSafeForConcurrentUse(t *testing.T) {
	mock := newMockExecutor()
	reg := NewRegistryWithExecutor(mock)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(3)
		go func() { defer wg.Done(); _ = reg.Execute("volume_up") }()
		go func() { defer wg.Done(); reg.SetStatusProvider(func() string { return "playing" }) }()
		go func() { defer wg.Done(); reg.Register("custom", func() error { return nil }) }()
	}
	wg.Wait()
	if n := len(mock.Calls()); n != 50 {
		t.Fatalf("expected 50 volume_up calls, got %d", n)
	}
}
