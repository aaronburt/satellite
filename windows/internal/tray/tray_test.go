package tray

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/getlantern/systray"
	"satellite/internal/mqtt"
	"satellite/internal/telemetry"
	"satellite/internal/updater"
	"satellite/internal/webui"
)

func TestNewTrayAndExit(t *testing.T) {
	client := mqtt.NewClient()
	collector := telemetry.NewCollector()
	server := webui.NewServer(collector, client)

	exitCalled := false
	tray := NewTray(server, client, nil, func() {
		exitCalled = true
	})

	if tray == nil {
		t.Fatal("expected non-nil tray")
	}

	ctx, cancel := context.WithCancel(context.Background())
	tray.cancel = cancel
	tray.onSystrayExit()

	if !exitCalled {
		t.Fatal("expected exit callback to be invoked")
	}
	if ctx.Err() == nil {
		t.Fatal("expected context to be cancelled")
	}
}

func TestManualCheckUpdate(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"tag_name": "v99.9.9", "html_url": "https://example.com/release"}`))
	}))
	defer ts.Close()

	u := updater.NewChecker("test/repo", "0.1.0", func(status updater.CheckStatus) {})
	u.SetAPIBaseURL(ts.URL)

	tray := &Tray{updaterInstance: u}
	tray.manualCheckUpdate()

	trayNil := &Tray{}
	trayNil.manualCheckUpdate()

	errServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer errServer.Close()

	uErr := updater.NewChecker("test/repo", "0.1.0", func(status updater.CheckStatus) {})
	uErr.SetAPIBaseURL(errServer.URL)
	trayErr := &Tray{updaterInstance: uErr}
	trayErr.manualCheckUpdate()

	upToDateServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"tag_name": "v0.1.0", "html_url": "https://example.com/release"}`))
	}))
	defer upToDateServer.Close()

	uUpToDate := updater.NewChecker("test/repo", "0.1.0", func(status updater.CheckStatus) {})
	uUpToDate.SetAPIBaseURL(upToDateServer.URL)
	trayUpToDate := &Tray{updaterInstance: uUpToDate}
	trayUpToDate.manualCheckUpdate()
}

func newMockMenuItem() *systray.MenuItem {
	return &systray.MenuItem{
		ClickedCh: make(chan struct{}, 5),
	}
}

func TestStatusUpdater(t *testing.T) {
	client := mqtt.NewClient()
	collector := telemetry.NewCollector()
	server := webui.NewServer(collector, client)
	_, _ = server.Start(0)
	defer server.Stop()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"tag_name": "v99.9.9", "html_url": "https://example.com/release"}`))
	}))
	defer ts.Close()

	u := updater.NewChecker("test/repo", "0.1.0", func(status updater.CheckStatus) {})
	u.SetAPIBaseURL(ts.URL)
	_, _ = u.Check(context.Background())

	tray := &Tray{
		server:          server,
		mqttClient:      client,
		updaterInstance: u,
		statusItem:      newMockMenuItem(),
		openItem:        newMockMenuItem(),
		toggleItem:      newMockMenuItem(),
		updateItem:      newMockMenuItem(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2100*time.Millisecond)
	defer cancel()
	tray.statusUpdater(ctx)

	server.SetWebUIEnabled(false)
	ctx2, cancel2 := context.WithTimeout(context.Background(), 2100*time.Millisecond)
	defer cancel2()
	tray.statusUpdater(ctx2)
}

func TestEventLoop(t *testing.T) {
	client := mqtt.NewClient()
	collector := telemetry.NewCollector()
	server := webui.NewServer(collector, client)
	_, _ = server.Start(0)
	defer server.Stop()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"tag_name": "v99.9.9", "html_url": "https://example.com/release"}`))
	}))
	defer ts.Close()

	u := updater.NewChecker("test/repo", "0.1.0", func(status updater.CheckStatus) {})
	u.SetAPIBaseURL(ts.URL)
	_, _ = u.Check(context.Background())

	tray := &Tray{
		server:          server,
		mqttClient:      client,
		updaterInstance: u,
		statusItem:      newMockMenuItem(),
		openItem:        newMockMenuItem(),
		toggleItem:      newMockMenuItem(),
		updateItem:      newMockMenuItem(),
		checkUpdateItem: newMockMenuItem(),
		quitItem:        newMockMenuItem(),
	}

	done := make(chan struct{})
	go func() {
		tray.eventLoop()
		close(done)
	}()

	time.Sleep(10 * time.Millisecond)
	tray.updateItem.ClickedCh <- struct{}{}
	time.Sleep(10 * time.Millisecond)
	tray.checkUpdateItem.ClickedCh <- struct{}{}
	time.Sleep(10 * time.Millisecond)
	tray.openItem.ClickedCh <- struct{}{}
	time.Sleep(10 * time.Millisecond)
	tray.toggleItem.ClickedCh <- struct{}{}
	time.Sleep(10 * time.Millisecond)
	tray.toggleItem.ClickedCh <- struct{}{}
	time.Sleep(10 * time.Millisecond)
	tray.quitItem.ClickedCh <- struct{}{}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("expected eventLoop to terminate on quitItem click")
	}
}

