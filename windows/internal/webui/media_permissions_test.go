package webui

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"satellite/internal/config"
	"satellite/internal/mqtt"
	"satellite/internal/telemetry"
	"satellite/internal/testutil"
	"testing"
)

func TestDisabledMediaAliasesCannotExecute(t *testing.T) {
	original := config.Get()
	testutil.IsolateConfigDir(t, t.TempDir())
	defer func() { _ = config.Save(original); _, _ = config.Load() }()
	cfg := original
	cfg.Expose.MediaControl = true
	cfg.Expose.MediaPermissions = &config.MediaPermissions{}
	if err := config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if _, err := config.Load(); err != nil {
		t.Fatal(err)
	}
	client := mqtt.NewClient()
	server := NewServer(telemetry.NewCollector(), client)
	called := false
	for _, action := range []string{"media_play", "media_next", "volume_mute"} {
		client.Registry().Register(action, func() error { called = true; return nil })
	}
	for _, action := range []string{"play", "next_track", "mute", "volume_up", "media_stop"} {
		response := httptest.NewRecorder()
		server.handleAction(response, httptest.NewRequest(http.MethodPost, "/api/action", bytes.NewBufferString(`{"action":"`+action+`"}`)))
		if response.Code != http.StatusForbidden {
			t.Errorf("%s: status %d", action, response.Code)
		}
	}
	if called {
		t.Fatal("disabled handler executed")
	}
}
