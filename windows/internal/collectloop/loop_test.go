package collectloop

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type fakeTicker struct {
	c       chan time.Time
	mu      sync.Mutex
	resets  []time.Duration
	stopped bool
}

func (f *fakeTicker) C() <-chan time.Time { return f.c }
func (f *fakeTicker) Reset(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.resets = append(f.resets, d)
}
func (f *fakeTicker) Stop() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stopped = true
}
func (f *fakeTicker) snapshot() ([]time.Duration, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]time.Duration(nil), f.resets...), f.stopped
}

// harness starts a loop on a fake ticker and counts tickers and ticks.
type harness struct {
	loop    *Loop
	ft      *fakeTicker
	created atomic.Int32
	initial atomic.Int64
	ticks   chan struct{}
	cancel  context.CancelFunc
	done    chan struct{}
}

func start(t *testing.T, sec int) *harness {
	t.Helper()
	h := &harness{ft: &fakeTicker{c: make(chan time.Time)}, ticks: make(chan struct{}, 100), done: make(chan struct{})}
	h.loop = New(sec, func() { h.ticks <- struct{}{} })
	h.loop.newTicker = func(d time.Duration) ticker {
		h.created.Add(1)
		h.initial.Store(int64(d))
		return h.ft
	}
	ctx, cancel := context.WithCancel(context.Background())
	h.cancel = cancel
	go func() { h.loop.Run(ctx); close(h.done) }()
	t.Cleanup(func() { cancel(); <-h.done })
	h.fire(t) // make sure Run is up before the test changes anything
	return h
}

// fire delivers one tick and waits for the collection it causes. Because the
// fake channel is unbuffered, a successful send also proves the loop goroutine
// has finished handling everything queued before it.
func (h *harness) fire(t *testing.T) {
	t.Helper()
	select {
	case h.ft.c <- time.Now():
	case <-time.After(2 * time.Second):
		t.Fatal("loop did not accept a tick")
	}
	select {
	case <-h.ticks:
	case <-time.After(2 * time.Second):
		t.Fatal("tick did not collect")
	}
}

func waitResets(t *testing.T, ft *fakeTicker, n int) []time.Duration {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if r, _ := ft.snapshot(); len(r) >= n {
			return r
		}
		time.Sleep(time.Millisecond)
	}
	r, _ := ft.snapshot()
	t.Fatalf("resets = %v, want at least %d", r, n)
	return nil
}

func TestNormalizeFollowsConfigContract(t *testing.T) {
	cases := map[int]time.Duration{-3: 5 * time.Second, 0: 5 * time.Second, 1: time.Second, 30: 30 * time.Second}
	for in, want := range cases {
		if got := Normalize(in); got != want {
			t.Errorf("Normalize(%d) = %v, want %v", in, got, want)
		}
	}
}

func TestRunStartsWithConfiguredInterval(t *testing.T) {
	h := start(t, 7)
	h.fire(t)
	if got := time.Duration(h.initial.Load()); got != 7*time.Second {
		t.Fatalf("initial interval = %v, want 7s", got)
	}
}

func TestSetIntervalResetsRunningTimer(t *testing.T) {
	h := start(t, 5)
	h.fire(t)
	h.loop.SetInterval(20)
	r := waitResets(t, h.ft, 1)
	if r[0] != 20*time.Second {
		t.Fatalf("reset to %v, want 20s", r[0])
	}
	if h.loop.Interval() != 20*time.Second {
		t.Fatalf("Interval() = %v", h.loop.Interval())
	}
	h.fire(t) // still collecting after the change
	if h.created.Load() != 1 {
		t.Fatalf("created %d tickers, want 1", h.created.Load())
	}
}

func TestSetIntervalNormalizesInvalidValues(t *testing.T) {
	h := start(t, 30)
	h.loop.SetInterval(0)
	if r := waitResets(t, h.ft, 1); r[0] != DefaultInterval {
		t.Fatalf("reset to %v, want %v", r[0], DefaultInterval)
	}
}

func TestSetIntervalSameValueDoesNotReset(t *testing.T) {
	h := start(t, 5)
	h.loop.SetInterval(5)
	h.loop.SetInterval(-1) // normalizes to 5s too
	h.fire(t)
	if r, _ := h.ft.snapshot(); len(r) != 0 {
		t.Fatalf("unexpected resets %v", r)
	}
}

func TestRapidChangesKeepOneTimerAndApplyLast(t *testing.T) {
	h := start(t, 5)
	var wg sync.WaitGroup
	for i := 1; i <= 50; i++ {
		wg.Add(1)
		go func(sec int) { defer wg.Done(); h.loop.SetInterval(sec) }(i + 100)
	}
	wg.Wait()
	h.loop.SetInterval(42)
	// After the dust settles the timer must end on the last value.
	deadline := time.Now().Add(2 * time.Second)
	for {
		r, _ := h.ft.snapshot()
		if len(r) > 0 && r[len(r)-1] == 42*time.Second {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("final reset never reached 42s: %v", r)
		}
		time.Sleep(time.Millisecond)
	}
	// One tick still means exactly one collection.
	h.fire(t)
	select {
	case <-h.ticks:
		t.Fatal("a single tick produced more than one collection")
	case <-time.After(20 * time.Millisecond):
	}
	if h.created.Load() != 1 {
		t.Fatalf("created %d tickers, want 1", h.created.Load())
	}
}

func TestShutdownStopsTimerAndCollection(t *testing.T) {
	h := start(t, 5)
	h.fire(t)
	h.cancel()
	select {
	case <-h.done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after cancel")
	}
	if _, stopped := h.ft.snapshot(); !stopped {
		t.Fatal("ticker not stopped on shutdown")
	}
	// Changes after shutdown must not block or panic.
	done := make(chan struct{})
	go func() { h.loop.SetInterval(9); h.loop.SetInterval(10); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("SetInterval blocked after shutdown")
	}
}

func TestConcurrentUpdatesDuringShutdown(t *testing.T) {
	for i := 0; i < 50; i++ {
		h := start(t, 5)
		var wg sync.WaitGroup
		for j := 0; j < 8; j++ {
			wg.Add(1)
			go func(n int) { defer wg.Done(); h.loop.SetInterval(n + 1) }(j)
		}
		h.cancel()
		wg.Wait()
		<-h.done
	}
}

// Real-timer check: a long interval changed to a short one collects promptly.
func TestRealTimerPicksUpShorterInterval(t *testing.T) {
	var n atomic.Int32
	l := New(3600, func() { n.Add(1) })
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { l.Run(ctx); close(done) }()
	defer func() { cancel(); <-done }()
	time.Sleep(10 * time.Millisecond)
	l.SetInterval(1)
	deadline := time.Now().Add(3 * time.Second)
	for n.Load() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("no collection within 3s after switching from 1h to 1s")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
