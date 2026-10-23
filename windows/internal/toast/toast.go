package toast

import (
	"net/url"
	"strings"
)

type Notification struct {
	Title   string `json:"title"`
	Message string `json:"message"`
	URL     string `json:"url,omitempty"`
	Silent  bool   `json:"silent,omitempty"`
}

// SafeURL returns the URL to open for a notification, or false if it should
// not be opened. Notifications can arrive over MQTT from anyone allowed to
// publish on the broker, so only plain http(s) links are accepted, never
// file paths, other schemes, or text a shell could interpret.
func SafeURL(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}
	for _, r := range raw {
		if r <= ' ' || r == 0x7f || strings.ContainsRune(`"<>^&|%`+"`", r) {
			return "", false
		}
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil {
		return "", false
	}
	return u.String(), true
}
