//go:build windows

package tray

import (
	"testing"
	"time"

	"satellite/internal/mqtt"
	"satellite/internal/privacy"
	"satellite/internal/telemetry"
	"satellite/internal/webui"
)

func TestPrivacyCountdownTitle(t *testing.T) {
	for _, tc := range []struct {
		status privacy.Status
		want   string
	}{
		{privacy.Status{}, "Privacy Pause"},
		{privacy.Status{Paused: true, RemainingSeconds: 300}, "Privacy paused: 05:00 remaining"},
		{privacy.Status{Paused: true, RemainingSeconds: 61}, "Privacy paused: 01:01 remaining"},
		{privacy.Status{Paused: true, RemainingSeconds: 1}, "Privacy paused: 00:01 remaining"},
	} {
		if got := privacyTitle(tc.status); got != tc.want {
			t.Errorf("title = %q, want %q", got, tc.want)
		}
	}
}

func TestTrayPauseDurationChoicesAndEarlyResume(t *testing.T) {
	t.Cleanup(privacy.Default.Resume)
	client := mqtt.NewClient()
	tray := NewTray(webui.NewServer(telemetry.NewCollector(), client), client, nil, nil)
	tray.openItem, tray.toggleItem = newMockMenuItem(), newMockMenuItem()
	tray.updateItem, tray.checkUpdateItem = newMockMenuItem(), newMockMenuItem()
	tray.quitItem, tray.resumeItem = newMockMenuItem(), newMockMenuItem()
	for i := range tray.pauseItems {
		tray.pauseItems[i] = newMockMenuItem()
	}
	done := make(chan struct{})
	go func() {
		tray.eventLoop()
		close(done)
	}()
	t.Cleanup(func() {
		tray.quitItem.ClickedCh <- struct{}{}
		<-done
	})
	for i, minutes := range privacyMinutes {
		previous := privacy.Default.Status().Generation
		tray.pauseItems[i].ClickedCh <- struct{}{}
		waitPrivacy(t, func() bool { return privacy.Default.Status().Generation != previous })
		status := privacy.Default.Status()
		if !status.Paused || status.RemainingSeconds < minutes*60-1 || status.RemainingSeconds > minutes*60 {
			t.Fatalf("%d minute choice: %+v", minutes, status)
		}
	}
	tray.resumeItem.ClickedCh <- struct{}{}
	waitPrivacy(t, func() bool { return !privacy.Default.Status().Paused })
}

func waitPrivacy(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for privacy menu action")
		}
		time.Sleep(time.Millisecond)
	}
}
