package webui

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"satellite/internal/config"
	"satellite/internal/mqtt"
	"satellite/internal/telemetry"
	"satellite/internal/testutil"
)

func TestLockCooldownSharedWithHTTP(t *testing.T) {
	original := config.Get()
	testutil.IsolateConfigDir(t, t.TempDir())
	defer func() { _ = config.Save(original) }()
	cfg := config.DefaultConfig()
	if err := config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	client := mqtt.NewClient()
	server := NewServer(telemetry.NewCollector(), client)
	calls := 0
	client.Registry().Register("lock", func() error { calls++; return nil })
	if err := client.Registry().Execute("lock"); err != nil {
		t.Fatal(err)
	}
	request := func() int {
		response := httptest.NewRecorder()
		server.handleAction(response, httptest.NewRequest(http.MethodPost, "/api/action", bytes.NewBufferString(`{"action":"lock_workstation"}`)))
		return response.Code
	}
	if got := request(); got != http.StatusTooManyRequests {
		t.Fatalf("status %d", got)
	}
	zero := 0
	cfg.LockCooldownSec = &zero
	if err := config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if got := request(); got != http.StatusOK {
		t.Fatalf("status %d", got)
	}
	if calls != 2 {
		t.Fatalf("calls %d", calls)
	}
}
