package webui

import (
	"encoding/json"
	"net/http"
	"reflect"
	"testing"
	"time"

	"satellite/internal/config"
	"satellite/internal/privacy"
	"satellite/internal/screenthumb"
)

func TestTrayPrivacyPauseHidesPreviewsAndPreservesSettings(t *testing.T) {
	t.Cleanup(privacy.Default.Resume)
	cfg := configWith(allExpose())
	srv, base, token := startTestServer(t, cfg)
	srv.SetCapabilities(allCaps())
	store := screenthumb.NewStore()
	store.Set(testJPEG)
	srv.SetThumbnailStore(store)
	before := config.Get()
	srv.PausePrivacy(5 * time.Minute)
	if !reflect.DeepEqual(before, config.Get()) {
		t.Fatal("privacy pause changed saved configuration")
	}
	resp, body := apiRequest(t, http.MethodGet, base+"/api/status?thumb=1", token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status response: %d %s", resp.StatusCode, body)
	}
	var status struct {
		Pause   privacy.Status `json:"privacy_pause"`
		Sensors []Sensor       `json:"sensors"`
	}
	if err := json.Unmarshal(body, &status); err != nil {
		t.Fatal(err)
	}
	if !status.Pause.Paused || status.Pause.RemainingSeconds < 299 || status.Pause.RemainingSeconds > 300 {
		t.Fatalf("pause status: %+v", status.Pause)
	}
	for _, sensor := range status.Sensors {
		switch sensor.ID {
		case "user_presence", "session_lock", "media_control", "active_window", "window_title", "microphone", "webcam", "screen_thumbnail":
			t.Errorf("paused sensor was listed: %s", sensor.ID)
		}
	}
	if resp, _ := apiRequest(t, http.MethodGet, base+"/api/screen-thumbnail", token, nil); resp.StatusCode != http.StatusNotFound {
		t.Fatalf("paused preview returned %d", resp.StatusCode)
	}
	srv.PausePrivacy(0)
	if privacy.Default.Status().Paused {
		t.Fatal("early resume did not clear pause")
	}
	if _, _, ok := store.Latest(); ok {
		t.Fatal("resume restored a stale preview")
	}
	store.Set(testJPEG)
	if resp, _ := apiRequest(t, http.MethodGet, base+"/api/screen-thumbnail", token, nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("fresh preview after resume returned %d", resp.StatusCode)
	}
}
