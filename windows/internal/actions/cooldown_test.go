package actions

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func TestLockCooldownAliasesAndExpiry(t *testing.T) {
	exec := newMockExecutor()
	r := NewRegistryWithExecutor(exec)
	now := time.Now()
	r.now = func() time.Time { return now }
	r.lockCooldown = func() time.Duration { return time.Minute }
	if err := r.Execute("lock"); err != nil {
		t.Fatal(err)
	}
	now = now.Add(59 * time.Second)
	if err := r.ExecutePayload([]byte(`{"action":"lock_workstation"}`)); !errors.Is(err, ErrLockCooldown) {
		t.Fatalf("got %v", err)
	}
	if err := r.Execute("volume_up"); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Second)
	if err := r.Execute("LOCK"); err != nil {
		t.Fatal(err)
	}
	if !equalCalls(exec.Calls(), []string{"lock", "vol_up", "lock"}) {
		t.Fatal(exec.Calls())
	}
}

func TestLockCooldownConcurrentRequests(t *testing.T) {
	exec := newMockExecutor()
	r := NewRegistryWithExecutor(exec)
	r.lockCooldown = func() time.Duration { return time.Minute }
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := r.Execute("lock"); err != nil && !errors.Is(err, ErrLockCooldown) {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if len(exec.Calls()) != 1 {
		t.Fatal(exec.Calls())
	}
}

func TestLockCooldownLiveChangesAndFailures(t *testing.T) {
	exec := newMockExecutor()
	exec.failOn["lock"] = true
	r := NewRegistryWithExecutor(exec)
	cooldown := time.Minute
	r.lockCooldown = func() time.Duration { return cooldown }
	if err := r.Execute("lock"); err == nil {
		t.Fatal("expected executor failure")
	}
	if err := r.Execute("lock"); !errors.Is(err, ErrLockCooldown) {
		t.Fatalf("got %v", err)
	}
	cooldown = 0
	delete(exec.failOn, "lock")
	for i := 0; i < 3; i++ {
		if err := r.Execute("lock"); err != nil {
			t.Fatal(err)
		}
	}
	cooldown = time.Minute
	if err := r.Execute("lock"); !errors.Is(err, ErrLockCooldown) {
		t.Fatalf("got %v", err)
	}
}
