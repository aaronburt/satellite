package config

import (
	"os"
	"path/filepath"
	"testing"

	"satellite/internal/testutil"
)

func TestOnChangeRunsAfterSuccessfulSaveWithSavedConfig(t *testing.T) {
	testutil.IsolateConfigDir(t, t.TempDir())
	var got []int
	remove := OnChange(func(c Config) {
		// The listener must see the new config through Get too, and must
		// not deadlock doing so.
		if Get().IntervalSec != c.IntervalSec {
			t.Errorf("Get() = %d during listener, want %d", Get().IntervalSec, c.IntervalSec)
		}
		got = append(got, c.IntervalSec)
	})
	defer remove()

	cfg := DefaultConfig()
	cfg.IntervalSec = 30
	if err := Save(cfg); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != 30 {
		t.Fatalf("listener calls = %v, want [30]", got)
	}

	remove()
	cfg.IntervalSec = 60
	if err := Save(cfg); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("removed listener still called: %v", got)
	}
}

func TestOnChangeNotCalledWhenSaveFails(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "file")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	testutil.IsolateConfigDir(t, blocker)
	called := false
	defer OnChange(func(Config) { called = true })()
	if err := Save(DefaultConfig()); err == nil {
		t.Fatal("expected Save to fail")
	}
	if called {
		t.Fatal("listener ran for a failed save")
	}
}

func TestOnChangeRemoveLeavesOtherListeners(t *testing.T) {
	testutil.IsolateConfigDir(t, t.TempDir())
	var a, b int
	removeA := OnChange(func(Config) { a++ })
	removeB := OnChange(func(Config) { b++ })
	defer removeB()
	removeA()
	removeA() // removing twice is harmless
	if err := Save(DefaultConfig()); err != nil {
		t.Fatal(err)
	}
	if a != 0 || b != 1 {
		t.Fatalf("a=%d b=%d, want a=0 b=1", a, b)
	}
}
