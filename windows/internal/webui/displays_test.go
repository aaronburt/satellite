package webui

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"satellite/internal/config"
	"satellite/internal/screenthumb"
)

func TestDisplayInventoryRequiresWebUIAuthenticationAndGET(t *testing.T) {
	srv, base, token := startTestServer(t, config.DefaultConfig())
	resp, _ := apiRequest(t, http.MethodGet, base+"/api/displays", "", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("without token: %d", resp.StatusCode)
	}
	resp, _ = apiRequest(t, http.MethodPost, base+"/api/displays", token, nil)
	if resp.StatusCode != http.StatusMethodNotAllowed || resp.Header.Get("Allow") != "GET" {
		t.Fatalf("POST: %d", resp.StatusCode)
	}
	resp, body := apiRequest(t, http.MethodGet, base+"/api/displays", token, nil)
	var displays []screenthumb.Display
	if resp.StatusCode != http.StatusOK || json.Unmarshal(body, &displays) != nil {
		t.Fatalf("inventory: %d %s", resp.StatusCode, body)
	}
	srv.SetWebUIEnabled(false)
	resp, _ = apiRequest(t, http.MethodGet, base+"/api/displays", token, nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("disabled WebUI: %d", resp.StatusCode)
	}
}

func TestNumberedDisplayPreviewsStaySeparateAndHideDisconnectedFrames(t *testing.T) {
	cfg := configWith(config.ExposeConfig{ScreenThumbnail: true})
	cfg.ScreenThumbnailDisplays = []int{1, 3}
	srv, _, _ := startTestServer(t, cfg)
	srv.SetCapabilities(allCaps())
	displays := []screenthumb.Display{{ID: 1, Name: "Display 1", Primary: true}, {ID: 3, Name: "Display 3", X: -1920}}
	srv.listDisplays = func() ([]screenthumb.Display, error) { return displays, nil }
	store := screenthumb.NewStore()
	store.SetDisplay(1, []byte("display one"))
	store.SetDisplay(2, []byte("not selected"))
	store.SetDisplay(3, []byte("display three"))
	srv.SetThumbnailStore(store)
	request := func(path string) *httptest.ResponseRecorder {
		t.Helper()
		w := httptest.NewRecorder()
		srv.handleScreenThumbnail(w, httptest.NewRequest(http.MethodGet, path, nil))
		return w
	}
	for id, want := range map[string]string{"1": "display one", "3": "display three"} {
		w := request("/api/screen-thumbnail?display=" + id)
		if w.Code != http.StatusOK || w.Body.String() != want || w.Header().Get("Content-Type") != "image/jpeg" {
			t.Fatalf("display %s: %d %s", id, w.Code, w.Body.String())
		}
	}
	for _, path := range []string{"/api/screen-thumbnail", "/api/screen-thumbnail?display=2"} {
		if w := request(path); w.Code != http.StatusNotFound {
			t.Fatalf("unselected display: %d", w.Code)
		}
	}
	for _, raw := range []string{"-1", "bad", "999"} {
		if w := request("/api/screen-thumbnail?display=" + raw); w.Code != http.StatusBadRequest {
			t.Fatalf("invalid display %s: %d", raw, w.Code)
		}
	}
	w := httptest.NewRecorder()
	srv.handleStatus(w, httptest.NewRequest(http.MethodGet, "/api/status", nil))
	var status statusBody
	if err := json.Unmarshal(w.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if len(status.Sensors) != 2 || status.Sensors[0].Display != 1 || status.Sensors[1].Display != 3 || status.Sensors[0].FrameTS == 0 || status.Sensors[1].FrameTS == 0 {
		t.Fatalf("numbered sensors = %+v", status.Sensors)
	}
	displays = displays[:1]
	if w := request("/api/screen-thumbnail?display=3"); w.Code != http.StatusNotFound {
		t.Fatal("served the stale frame of a disconnected display")
	}
	frames := srv.displayFrames(cfg.ScreenThumbnailDisplays, store)
	sensors := BuildSensors(cfg.Expose, allCaps(), srv.collector.Last(), Frame{Displays: frames}, testNow)
	if sensors[1].Value != "Display unavailable" || sensors[1].FrameTS != 0 {
		t.Fatalf("disconnected sensor = %+v", sensors[1])
	}
}

func TestDisplaySelectionConfigSaveAndValidation(t *testing.T) {
	_, base, token := startTestServer(t, config.DefaultConfig())
	resp, body := apiRequest(t, http.MethodPost, base+"/api/config", token, json.RawMessage(`{"screen_thumbnail_displays":[3,1,3]}`))
	if resp.StatusCode != http.StatusOK || !slices.Equal(config.Get().ScreenThumbnailDisplays, []int{1, 3}) {
		t.Fatalf("save: %d %s", resp.StatusCode, body)
	}
	resp, body = apiRequest(t, http.MethodGet, base+"/api/config", token, nil)
	if resp.StatusCode != http.StatusOK || !bytes.Contains(body, []byte(`"screen_thumbnail_displays":[1,3]`)) {
		t.Fatalf("load: %d %s", resp.StatusCode, body)
	}
	resp, _ = apiRequest(t, http.MethodPost, base+"/api/config", token, json.RawMessage(`{"screen_thumbnail_displays":[-1]}`))
	if resp.StatusCode != http.StatusBadRequest || !slices.Equal(config.Get().ScreenThumbnailDisplays, []int{1, 3}) {
		t.Fatal("invalid selection changed the saved settings")
	}
	resp, _ = apiRequest(t, http.MethodPost, base+"/api/config", token, json.RawMessage(`{"screen_thumbnail_displays":[]}`))
	if resp.StatusCode != http.StatusOK || len(config.Get().ScreenThumbnailDisplays) != 0 {
		t.Fatal("could not restore primary-display compatibility mode")
	}
}
