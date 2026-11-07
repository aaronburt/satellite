package webui

import (
	"encoding/json"
	"net/http"
	"reflect"
	"testing"

	"satellite/internal/config"
	"satellite/internal/icue"
)

func TestICUESDKCheckRequiresAuthenticationAndGET(t *testing.T) {
	s, base, token := startTestServer(t, config.DefaultConfig())
	calls := 0
	s.checkICUESDK = func() icue.SDKStatus { calls++; return icue.SDKStatus{Supported: true, Status: "missing"} }
	for _, test := range []struct {
		method, token string
		want          int
	}{
		{"GET", "", http.StatusUnauthorized},
		{"POST", token, http.StatusMethodNotAllowed},
		{"HEAD", token, http.StatusMethodNotAllowed},
	} {
		response, _ := apiRequest(t, test.method, base+"/api/icue-sdk", test.token, nil)
		if response.StatusCode != test.want {
			t.Fatalf("%s: got %d, want %d", test.method, response.StatusCode, test.want)
		}
	}
	if calls != 0 {
		t.Fatal("invalid requests probed the SDK")
	}
}

func TestICUESDKCheckDetectsInstallationWithoutChangingConfig(t *testing.T) {
	cfg := config.DefaultConfig()
	s, base, token := startTestServer(t, cfg)
	original := config.Get()
	for _, expected := range []icue.SDKStatus{
		{Supported: true, Status: "missing"},
		{Supported: true, Status: "unavailable"},
		{Supported: true, Available: true, Status: "ready"},
		{Status: "unsupported"},
	} {
		s.checkICUESDK = func() icue.SDKStatus { return expected }
		response, body := apiRequest(t, "GET", base+"/api/icue-sdk", token, nil)
		var got icue.SDKStatus
		if response.StatusCode != 200 {
			t.Fatalf("check failed: %d %s", response.StatusCode, body)
		}
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatal(err)
		}
		if got != expected {
			t.Fatalf("got %+v, want %+v", got, expected)
		}
		if !reflect.DeepEqual(config.Get(), original) {
			t.Fatal("SDK check changed settings")
		}
	}
}
