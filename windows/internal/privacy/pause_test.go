package privacy

import (
	"sync"
	"testing"
	"time"
)

func TestCountdownExpiryAndReplacement(t *testing.T) {
	now := time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)
	c := &Controller{now: func() time.Time { return now }}
	c.Pause(5 * time.Minute)
	if s := c.Status(); !s.Paused || s.RemainingSeconds != 300 || !s.Until.Equal(now.Add(5*time.Minute)) {
		t.Fatalf("initial status: %+v", s)
	}
	now = now.Add(120*time.Second + time.Millisecond)
	if s := c.Status(); s.RemainingSeconds != 180 {
		t.Fatalf("countdown should round up: %+v", s)
	}
	c.Pause(15 * time.Minute)
	if s := c.Status(); s.RemainingSeconds != 900 {
		t.Fatalf("replacement should start from now: %+v", s)
	}
	now = now.Add(15 * time.Minute)
	if s := c.Status(); s.Paused || s.RemainingSeconds != 0 {
		t.Fatalf("must resume at the deadline: %+v", s)
	}
	c.Pause(time.Minute)
	now = now.Add(24 * time.Hour)
	if c.Status().Paused {
		t.Fatal("time asleep must count toward the pause")
	}
}

func TestResumeAndOldWorkInvalidation(t *testing.T) {
	c := &Controller{}
	before := c.Status().Generation
	if !c.Allows(before) {
		t.Fatal("fresh work should be allowed")
	}
	c.Pause(time.Hour)
	if c.Allows(before) || c.Allows(0) {
		t.Fatal("paused work must be rejected")
	}
	c.Resume()
	if c.Status().Paused || c.Allows(before) || !c.Allows(c.Status().Generation) {
		t.Fatal("resume must allow fresh work and reject old work")
	}
	c.Pause(-time.Second)
	if c.Status().Paused {
		t.Fatal("nonpositive duration must not pause")
	}
}

func TestConcurrentPauseAndStatus(t *testing.T) {
	c := &Controller{}
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 100 {
				c.Pause(time.Minute)
				c.Allows(c.Status().Generation)
				c.Resume()
			}
		})
	}
	wg.Wait()
	c.Resume()
	if c.Status().Paused {
		t.Fatal("final resume must take effect")
	}
}
