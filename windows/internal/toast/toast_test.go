package toast

import "testing"

func TestSafeURL(t *testing.T) {
	allowed := map[string]string{
		"https://github.com/aaronburt/satellite/releases": "https://github.com/aaronburt/satellite/releases",
		"  http://homeassistant.local:8123/lovelace  ":    "http://homeassistant.local:8123/lovelace",
		"https://example.com/path?q=1#frag":               "https://example.com/path?q=1#frag",
	}
	for in, want := range allowed {
		got, ok := SafeURL(in)
		if !ok || got != want {
			t.Errorf("SafeURL(%q) = %q, %v; want %q", in, got, ok, want)
		}
	}

	rejected := []string{
		"",
		"   ",
		"file:///C:/Windows/System32/calc.exe",
		"C:\\Windows\\System32\\calc.exe",
		"javascript:alert(1)",
		"ms-settings:privacy",
		"https://example.com&calc.exe",
		"https://example.com/|calc",
		"https://example.com/^&calc",
		"https://example.com/%COMSPEC%",
		"https://example.com/\" & calc",
		"https://exa mple.com",
		"https://example.com/\ncalc",
		"https://user:pass@example.com",
		"https://",
		"//example.com",
	}
	for _, in := range rejected {
		if got, ok := SafeURL(in); ok {
			t.Errorf("SafeURL(%q) accepted as %q, want rejected", in, got)
		}
	}
}
