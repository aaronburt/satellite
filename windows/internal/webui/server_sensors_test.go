package webui

import (
	"bytes"
	"encoding/json"
	"image"
	"net/http"
	"strings"
	"testing"
	"time"

	"satellite/internal/config"
	"satellite/internal/screenthumb"
)

var testJPEG = []byte{0xff, 0xd8, 0xff, 0xe0, 'f', 'a', 'k', 'e', 0xff, 0xd9}

func configWith(expose config.ExposeConfig) config.Config {
	cfg := config.DefaultConfig()
	cfg.Expose = expose
	return cfg
}

type statusBody struct {
	MQTTStatus string          `json:"mqtt_status"`
	Telemetry  json.RawMessage `json:"telemetry"`
	Sensors    []Sensor        `json:"sensors"`
}

func getStatus(t *testing.T, base, token, query string) statusBody {
	t.Helper()
	resp, body := apiRequest(t, http.MethodGet, base+"/api/status"+query, token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/status%s: status %d: %s", query, resp.StatusCode, body)
	}
	var out statusBody
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("status is not JSON: %v\n%s", err, body)
	}
	return out
}

func TestStatusListsTheEnabledSensors(t *testing.T) {
	srv, base, token := startTestServer(t, configWith(config.ExposeConfig{CPU: true, Memory: true, Uptime: true}))
	srv.SetCapabilities(allCaps())

	st := getStatus(t, base, token, "")
	if !reflectEqual(ids(st.Sensors), []string{"cpu", "memory", "uptime"}) {
		t.Fatalf("sensors = %v", ids(st.Sensors))
	}
	if len(st.Telemetry) == 0 {
		t.Error("the Performance tab still needs the raw telemetry")
	}
}

func TestStatusFollowsTheSavedConfig(t *testing.T) {
	srv, base, token := startTestServer(t, configWith(config.ExposeConfig{CPU: true}))
	srv.SetCapabilities(allCaps())

	next := configWith(config.ExposeConfig{CPU: true, Wifi: true})
	next.BindAddress = "127.0.0.1"
	if err := config.Save(next); err != nil {
		t.Fatal(err)
	}
	if got := ids(getStatus(t, base, token, "").Sensors); !reflectEqual(got, []string{"cpu", "wifi"}) {
		t.Fatalf("after enabling Wi-Fi sensors = %v", got)
	}
}

func TestStatusWithNothingEnabledSendsAnEmptyList(t *testing.T) {
	srv, base, token := startTestServer(t, configWith(config.ExposeConfig{}))
	srv.SetCapabilities(allCaps())
	_, body := apiRequest(t, http.MethodGet, base+"/api/status", token, nil)
	if !strings.Contains(string(body), `"sensors":[]`) {
		t.Fatalf("sensors must be an empty list, got %s", body)
	}
}

func TestScreenThumbnailIsServedWhenEnabled(t *testing.T) {
	srv, base, token := startTestServer(t, configWith(config.ExposeConfig{ScreenThumbnail: true}))
	srv.SetCapabilities(allCaps())
	store := screenthumb.NewStore()
	srv.SetThumbnailStore(store)

	// Nothing captured yet.
	if resp, _ := apiRequest(t, http.MethodGet, base+"/api/screen-thumbnail", token, nil); resp.StatusCode != http.StatusNotFound {
		t.Fatalf("no frame: status %d, want 404", resp.StatusCode)
	}

	store.Set(testJPEG)
	resp, body := apiRequest(t, http.MethodGet, base+"/api/screen-thumbnail", token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "image/jpeg" {
		t.Errorf("Content-Type = %q", ct)
	}
	if cc := resp.Header.Get("Cache-Control"); !strings.Contains(cc, "no-store") {
		t.Errorf("Cache-Control = %q: a screenshot must never be cached", cc)
	}
	if !bytes.Equal(body, testJPEG) {
		t.Errorf("body = %v, want the stored frame", body)
	}
}

func TestScreenThumbnailAcceptsTheTokenInTheQuery(t *testing.T) {
	// An <img> tag cannot send headers, so the page puts the token in the URL.
	srv, base, token := startTestServer(t, configWith(config.ExposeConfig{ScreenThumbnail: true}))
	srv.SetCapabilities(allCaps())
	store := screenthumb.NewStore()
	store.Set(testJPEG)
	srv.SetThumbnailStore(store)

	resp, err := testClient.Get(base + "/api/screen-thumbnail?token=" + token)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func TestScreenThumbnailRefusesWithoutTheToken(t *testing.T) {
	srv, base, _ := startTestServer(t, configWith(config.ExposeConfig{ScreenThumbnail: true}))
	srv.SetCapabilities(allCaps())
	store := screenthumb.NewStore()
	store.Set(testJPEG)
	srv.SetThumbnailStore(store)

	for _, tok := range []string{"", "wrong"} {
		resp, body := apiRequest(t, http.MethodGet, base+"/api/screen-thumbnail", tok, nil)
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("token %q: status %d, want 401", tok, resp.StatusCode)
		}
		if bytes.Contains(body, testJPEG) {
			t.Error("the screenshot leaked to an unauthenticated request")
		}
	}
}

func TestScreenThumbnailIsHiddenWhenTheSensorIsOff(t *testing.T) {
	srv, base, token := startTestServer(t, configWith(config.ExposeConfig{CPU: true}))
	srv.SetCapabilities(allCaps())
	store := screenthumb.NewStore()
	store.Set(testJPEG) // a leftover frame must not be served
	srv.SetThumbnailStore(store)

	if resp, _ := apiRequest(t, http.MethodGet, base+"/api/screen-thumbnail", token, nil); resp.StatusCode != http.StatusNotFound {
		t.Fatalf("sensor off: status %d, want 404", resp.StatusCode)
	}
	if srv.ThumbnailWanted() {
		t.Error("asking for a disabled preview must not make the agent capture the screen")
	}
}

func TestScreenThumbnailIsHiddenOnPlatformsThatCannotCaptureIt(t *testing.T) {
	srv, base, token := startTestServer(t, configWith(config.ExposeConfig{ScreenThumbnail: true}))
	caps := allCaps()
	caps.ScreenThumbnail = false
	srv.SetCapabilities(caps)
	store := screenthumb.NewStore()
	store.Set(testJPEG)
	srv.SetThumbnailStore(store)

	if resp, _ := apiRequest(t, http.MethodGet, base+"/api/screen-thumbnail", token, nil); resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status %d, want 404", resp.StatusCode)
	}
}

func TestScreenThumbnailWithoutAStoreIs404(t *testing.T) {
	srv, base, token := startTestServer(t, configWith(config.ExposeConfig{ScreenThumbnail: true}))
	srv.SetCapabilities(allCaps())
	if resp, _ := apiRequest(t, http.MethodGet, base+"/api/screen-thumbnail", token, nil); resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status %d, want 404", resp.StatusCode)
	}
}

func TestScreenThumbnailEnforcesGETAndTheWebUISwitch(t *testing.T) {
	srv, base, token := startTestServer(t, configWith(config.ExposeConfig{ScreenThumbnail: true}))
	srv.SetCapabilities(allCaps())
	store := screenthumb.NewStore()
	store.Set(testJPEG)
	srv.SetThumbnailStore(store)

	for _, m := range []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodHead} {
		resp, _ := apiRequest(t, m, base+"/api/screen-thumbnail", token, nil)
		if resp.StatusCode != http.StatusMethodNotAllowed || resp.Header.Get("Allow") != http.MethodGet {
			t.Errorf("%s: status %d Allow %q, want 405 GET", m, resp.StatusCode, resp.Header.Get("Allow"))
		}
	}

	srv.SetWebUIEnabled(false)
	if resp, _ := apiRequest(t, http.MethodGet, base+"/api/screen-thumbnail", token, nil); resp.StatusCode != http.StatusNotFound {
		t.Fatalf("web UI off: status %d, want 404", resp.StatusCode)
	}
}

func TestStatusShowsTheScreenPreview(t *testing.T) {
	srv, base, token := startTestServer(t, configWith(config.ExposeConfig{ScreenThumbnail: true}))
	srv.SetCapabilities(allCaps())
	store := screenthumb.NewStore()
	srv.SetThumbnailStore(store)

	before := getStatus(t, base, token, "")
	if s := byID(t, before.Sensors, "screen_thumbnail"); s.FrameTS != 0 || s.State != StateWaiting {
		t.Fatalf("before a capture: %+v", s)
	}

	store.Set(testJPEG)
	after := getStatus(t, base, token, "")
	s := byID(t, after.Sensors, "screen_thumbnail")
	if s.FrameTS == 0 || s.State != StateActive || !s.Image {
		t.Fatalf("after a capture: %+v", s)
	}
}

// The agent only captures the screen for the page while the page is looking
// at the preview, so a forgotten tab costs nothing.
func TestPreviewCaptureIsOnlyWantedWhileAPageWatches(t *testing.T) {
	srv, base, token := startTestServer(t, configWith(config.ExposeConfig{ScreenThumbnail: true}))
	srv.SetCapabilities(allCaps())

	if srv.ThumbnailWanted() {
		t.Fatal("nobody has asked yet")
	}
	getStatus(t, base, token, "") // a poll from another tab does not count
	if srv.ThumbnailWanted() {
		t.Fatal("a poll without thumb=1 must not start captures")
	}
	getStatus(t, base, token, "?thumb=1")
	if !srv.ThumbnailWanted() {
		t.Fatal("thumb=1 should start captures")
	}

	srv.thumbSeen.Store(time.Now().Add(-thumbViewWindow - time.Second).UnixNano())
	if srv.ThumbnailWanted() {
		t.Fatal("captures must stop once the page stops asking")
	}
}

func TestPreviewRequestForADisabledSensorDoesNotStartCaptures(t *testing.T) {
	srv, base, token := startTestServer(t, configWith(config.ExposeConfig{CPU: true}))
	srv.SetCapabilities(allCaps())
	getStatus(t, base, token, "?thumb=1")
	if srv.ThumbnailWanted() {
		t.Fatal("thumb=1 must be ignored while the sensor is off")
	}
}

func TestFetchingTheImageCountsAsWatching(t *testing.T) {
	srv, base, token := startTestServer(t, configWith(config.ExposeConfig{ScreenThumbnail: true}))
	srv.SetCapabilities(allCaps())
	srv.SetThumbnailStore(screenthumb.NewStore())
	apiRequest(t, http.MethodGet, base+"/api/screen-thumbnail", token, nil)
	if !srv.ThumbnailWanted() {
		t.Fatal("an open preview that is polling the image keeps captures going")
	}
}

func reflectEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// The whole path the preview takes: the page asks, the agent captures into the
// store, the next poll shows the frame and the image URL serves it. No broker
// is involved, so this is also the "MQTT is down" case.
func TestPreviewWorksWithoutABroker(t *testing.T) {
	srv, base, token := startTestServer(t, configWith(config.ExposeConfig{ScreenThumbnail: true}))
	srv.SetCapabilities(allCaps())
	store := screenthumb.NewStore()
	srv.SetThumbnailStore(store)

	captures := 0
	runner := &screenthumb.Runner{
		Enabled: func() bool { return config.Get().Expose.ScreenThumbnail },
		Ready:   srv.ThumbnailWanted, // what main.go uses when MQTT is not connected
		Store:   store,
		Capture: func(int) (image.Image, error) {
			captures++
			return image.NewRGBA(image.Rect(0, 0, 160, 90)), nil
		},
		Publish: func([]byte) error { return nil },
	}

	if res, _ := runner.Tick(); res != screenthumb.Skipped || captures != 0 {
		t.Fatal("no page is watching, so the screen must not be captured")
	}

	getStatus(t, base, token, "?thumb=1") // the page opens the Sensors tab
	if res, err := runner.Tick(); res != screenthumb.Published || err != nil {
		t.Fatalf("Tick = %v, %v", res, err)
	}

	s := byID(t, getStatus(t, base, token, "?thumb=1").Sensors, "screen_thumbnail")
	if s.FrameTS == 0 {
		t.Fatalf("the poll after a capture must carry the frame: %+v", s)
	}
	resp, body := apiRequest(t, http.MethodGet, base+"/api/screen-thumbnail", token, nil)
	if resp.StatusCode != http.StatusOK || !bytes.HasPrefix(body, []byte{0xff, 0xd8}) {
		t.Fatalf("image: status %d, %d bytes", resp.StatusCode, len(body))
	}

	// Turning the sensor off drops the frame and stops serving it.
	off := configWith(config.ExposeConfig{CPU: true})
	off.BindAddress = "127.0.0.1"
	if err := config.Save(off); err != nil {
		t.Fatal(err)
	}
	runner.Tick()
	if _, _, ok := store.Latest(); ok {
		t.Error("the stored screenshot outlived the sensor")
	}
	if resp, _ := apiRequest(t, http.MethodGet, base+"/api/screen-thumbnail", token, nil); resp.StatusCode != http.StatusNotFound {
		t.Errorf("after turning it off: status %d, want 404", resp.StatusCode)
	}
}
