package mqtt

import (
	"encoding/json"
	"testing"
	"time"

	"satellite/internal/config"
)

func findItem(t *testing.T, items []DiscoveryItem, key string) DiscoveryItem {
	t.Helper()
	for _, it := range items {
		if it.Key == key {
			return it
		}
	}
	t.Fatalf("no discovery item %q", key)
	return DiscoveryItem{}
}

func TestScreenThumbnailIsOffByDefault(t *testing.T) {
	stubHardware(t, nil, nil)
	cfg := config.DefaultConfig()
	if cfg.Expose.ScreenThumbnail {
		t.Fatal("screen thumbnail must be off in the default config")
	}
	item := findItem(t, GetAllDiscoveryItems(cfg, false, allCaps()), "screen_thumbnail")
	if item.ShouldRun || len(item.Payload) != 0 {
		t.Fatalf("default config must clear the image entity, got %q", item.Payload)
	}
}

func TestScreenThumbnailDiscoveryIsAnImageEntity(t *testing.T) {
	stubHardware(t, nil, nil)
	cfg := config.DefaultConfig()
	cfg.NodeID = "desk"
	cfg.MQTT.TopicPrefix = "sat"
	cfg.Expose.ScreenThumbnail = true

	item := findItem(t, GetAllDiscoveryItems(cfg, false, allCaps()), "screen_thumbnail")
	if !item.ShouldRun {
		t.Fatal("enabled + supported should be discovered")
	}
	if item.Topic != "homeassistant/image/desk/desk_screen_thumbnail/config" {
		t.Errorf("topic = %s", item.Topic)
	}
	var p map[string]any
	if err := json.Unmarshal(item.Payload, &p); err != nil {
		t.Fatal(err)
	}
	if p["image_topic"] != "sat/desk/screen_thumbnail" {
		t.Errorf("image_topic = %v", p["image_topic"])
	}
	if p["content_type"] != "image/jpeg" {
		t.Errorf("content_type = %v", p["content_type"])
	}
	if _, ok := p["state_topic"]; ok {
		t.Error("image entities take image_topic, not state_topic")
	}
}

func TestScreenThumbnailNeedsPlatformSupport(t *testing.T) {
	stubHardware(t, nil, nil)
	cfg := config.DefaultConfig()
	cfg.Expose.ScreenThumbnail = true
	caps := allCaps()
	caps.ScreenThumbnail = false
	if findItem(t, GetAllDiscoveryItems(cfg, false, caps), "screen_thumbnail").ShouldRun {
		t.Fatal("must not be discovered where capture is unsupported")
	}
}

func TestPublishScreenThumbnailSendsRawBytesUnretained(t *testing.T) {
	b := startBroker(t, "", "")
	cfg := brokerConfig(b)
	cfg.Expose.ScreenThumbnail = true
	c := startClient(t, cfg)

	jpeg := "\xff\xd8\xff\xe0fake-jpeg\xff\xd9"
	if err := c.PublishScreenThumbnail([]byte(jpeg)); err != nil {
		t.Fatal(err)
	}
	m := b.waitForMessage(t, "sat/node1/screen_thumbnail", jpeg)
	if m.retain {
		t.Error("thumbnails must not be retained by the broker")
	}
}

func TestPublishScreenThumbnailDropsWhenDisabled(t *testing.T) {
	b := startBroker(t, "", "")
	c := startClient(t, brokerConfig(b))
	if err := c.PublishScreenThumbnail([]byte("img")); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	if n := len(b.messages("sat/node1/screen_thumbnail")); n != 0 {
		t.Fatalf("published %d thumbnails while disabled", n)
	}
}

func TestPublishScreenThumbnailWhileDisconnectedIsANoop(t *testing.T) {
	c := NewClient()
	if err := c.PublishScreenThumbnail([]byte("img")); err != nil {
		t.Fatalf("disconnected publish should be dropped quietly, got %v", err)
	}
}
