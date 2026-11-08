package config

import (
	"encoding/json"
	"testing"
)

func TestLockCooldownDefaultsAndRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		raw  string
		want int
	}{
		{`{}`, 60},
		{`{"lock_cooldown_sec":0}`, 0},
		{`{"lock_cooldown_sec":120}`, 120},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			cfg := DefaultConfig()
			if err := json.Unmarshal([]byte(tc.raw), &cfg); err != nil {
				t.Fatal(err)
			}
			if cfg.LockCooldownSeconds() != tc.want {
				t.Fatal(cfg.LockCooldownSeconds())
			}
			for _, password := range []string{"", "test-password"} {
				raw, err := ExportConfig(cfg, password)
				if err != nil {
					t.Fatal(err)
				}
				result, err := ImportConfig(raw, ImportOptions{Passphrase: password})
				if err != nil {
					t.Fatal(err)
				}
				if result.Config.LockCooldownSeconds() != tc.want {
					t.Fatal(result.Config.LockCooldownSeconds())
				}
			}
		})
	}
}

func TestLockCooldownValidation(t *testing.T) {
	for _, value := range []int{-1, 2147483648} {
		cfg := DefaultConfig()
		cfg.LockCooldownSec = &value
		if cfg.ValidateLockCooldown() == nil {
			t.Fatalf("accepted %d", value)
		}
	}
}
