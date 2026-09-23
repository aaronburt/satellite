package toast

import (
	"testing"
	"time"
)

func TestNotificationFields(t *testing.T) {
	n := Notification{
		Title:   "Test Alert",
		Message: "Hello from test",
		URL:     "https://example.com",
		Silent:  true,
	}

	if n.Title != "Test Alert" {
		t.Fatalf("expected title 'Test Alert', got %s", n.Title)
	}
	if n.Message != "Hello from test" {
		t.Fatalf("expected message 'Hello from test', got %s", n.Message)
	}
	if n.URL != "https://example.com" {
		t.Fatalf("expected url 'https://example.com', got %s", n.URL)
	}
	if !n.Silent {
		t.Fatalf("expected silent to be true")
	}
}

func TestShow(t *testing.T) {
	n := Notification{
		Title:   "",
		Message: "Silent test toast",
		URL:     "",
	}
	_ = Show(n)
}

func TestShowWithURL(t *testing.T) {
	n := Notification{
		Title:   "Test",
		Message: "Toast with URL",
		URL:     "http://127.0.0.1:65535/nonexistent",
	}
	_ = Show(n)
	time.Sleep(50 * time.Millisecond)
}
