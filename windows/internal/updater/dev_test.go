package updater

import "testing"

func TestDevBuildDisablesAutomaticUpdates(t *testing.T) {
	c := NewChecker("", "0.99999", nil)
	if c.IsEnabled() {
		t.Fatal("dev checker enabled by default")
	}
	c.SetEnabled(true)
	c.SetAutoUpdate(true)
	if c.IsEnabled() || c.autoUpdate {
		t.Fatal("dev build allowed automatic updates")
	}
}
