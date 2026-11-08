package mqtt

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"satellite/internal/capabilities"
	"satellite/internal/config"
	"satellite/internal/icue"

	"github.com/eclipse/paho.golang/paho"
)

type rgbDiscovery struct {
	cfg          config.Config
	enabled      bool
	publish      func(context.Context, string, []byte) error
	mu           sync.Mutex
	known        map[string]string
	cleanupMu    sync.Mutex
	cleanup      map[string]struct{}
	cleanupReady chan struct{}
}

func newRGBDiscovery(cfg config.Config, caps capabilities.PlatformCapabilities, client *paho.Client) *rgbDiscovery {
	return &rgbDiscovery{
		cfg:          cfg,
		enabled:      cfg.Expose.ICUERGB && caps.ICUERGB,
		known:        make(map[string]string),
		cleanup:      make(map[string]struct{}),
		cleanupReady: make(chan struct{}, 1),
		publish: func(ctx context.Context, topic string, payload []byte) error {
			ack, err := client.Publish(ctx, &paho.Publish{Topic: topic, Payload: payload, QoS: 1, Retain: true})
			if err == nil && ack != nil && ack.ReasonCode >= 0x80 {
				return fmt.Errorf("RGB discovery publish rejected: MQTT reason %d", ack.ReasonCode)
			}
			return err
		},
	}
}

func rgbDiscoveryPrefix(nodeID string) string {
	return "homeassistant/sensor/" + discoveryID(nodeID) + "_icue_rgb/"
}

func rgbDiscoveryItem(cfg config.Config, d icue.Device) DiscoveryItem {
	prefix := cfg.MQTT.TopicPrefix
	if prefix == "" {
		prefix = "satellite"
	}
	base := prefix + "/" + cfg.NodeID
	key := "icue_rgb_" + d.ID
	id, _ := json.Marshal(d.ID)
	selectDevice := fmt.Sprintf("{%% set d = (value_json.rgb.devices | selectattr('id', 'equalto', %s) | first | default({})) if value_json.rgb is defined and value_json.rgb.status in ['connected', 'partial'] else {} %%}", id)
	name := strings.TrimSpace(d.Model)
	if name == "" {
		name = "iCUE RGB " + d.ID[:min(8, len(d.ID))]
	}
	payload := EntityDiscoveryPayload{
		Component:     "sensor",
		Name:          "Average colour",
		StateTopic:    base + "/state",
		ValueTemplate: selectDevice + "{{ '#%02X%02X%02X' | format(d.average_rgb[0], d.average_rgb[1], d.average_rgb[2]) if d else none }}",
		UniqueID:      cfg.NodeID + "_" + key,
		Device: DeviceInfo{
			Identifiers:  []string{"satellite_" + cfg.NodeID + "_" + key},
			Name:         name,
			Model:        d.Model,
			Manufacturer: "Corsair",
			ViaDevice:    "satellite_" + cfg.NodeID,
		},
		Availability: []EntityAvailability{
			{Topic: base + "/status"},
			{Topic: base + "/state", ValueTemplate: selectDevice + "{{ 'online' if d and d.leds | count > 0 and value_json.rgb.status in ['connected', 'partial'] else 'offline' }}"},
		},
		AvailabilityMode:       "all",
		JSONAttributesTopic:    base + "/state",
		JSONAttributesTemplate: selectDevice + "{{ dict(d, led_count=d.leds | count) | tojson if d else {'average_rgb': none, 'leds': [], 'led_count': 0} | tojson }}",
		Icon:                   "mdi:palette",
	}
	data, _ := json.Marshal(payload)
	return DiscoveryItem{Key: key, Topic: rgbDiscoveryPrefix(cfg.NodeID) + discoveryID(cfg.NodeID+"_"+key) + "/config", Payload: data, ShouldRun: true}
}

func (r *rgbDiscovery) sync(ctx context.Context, state *icue.State) error {
	if !r.enabled || state == nil || (state.Status != "connected" && state.Status != "partial") {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, d := range state.Devices {
		if d.ID == "" || len(d.LEDs) == 0 {
			continue
		}
		item := rgbDiscoveryItem(r.cfg, d)
		if r.known[item.Topic] == string(item.Payload) {
			continue
		}
		pubCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		err := r.publish(pubCtx, item.Topic, item.Payload)
		cancel()
		if err != nil {
			return err
		}
		r.known[item.Topic] = string(item.Payload)
	}
	return nil
}

func (r *rgbDiscovery) queueCleanup(p *paho.Publish) {
	if r.enabled || len(p.Payload) == 0 {
		return
	}
	r.cleanupMu.Lock()
	r.cleanup[p.Topic] = struct{}{}
	r.cleanupMu.Unlock()
	select {
	case r.cleanupReady <- struct{}{}:
	default:
	}
}

func (r *rgbDiscovery) clearRetained(ctx context.Context) error {
	r.cleanupMu.Lock()
	topics := r.cleanup
	r.cleanup = make(map[string]struct{})
	r.cleanupMu.Unlock()
	for topic := range topics {
		pubCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		err := r.publish(pubCtx, topic, nil)
		cancel()
		if err != nil {
			return err
		}
	}
	return nil
}
