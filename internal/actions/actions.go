package actions

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"satellite/internal/logger"
	"satellite/internal/telemetry"
)

var ErrActionNotFound = errors.New("action not recognized")

type Handler func() error

type StatusProvider func() string

type Registry struct {
	mu             sync.RWMutex
	handlers       map[string]Handler
	statusProvider StatusProvider
}

type CommandMessage struct {
	Action  string `json:"action"`
	Command string `json:"command"`
}

func NewRegistry() *Registry {
	r := &Registry{
		handlers: make(map[string]Handler),
	}

	r.Register("lock", LockWorkstation)
	r.Register("lock_workstation", LockWorkstation)

	r.Register("media_play_pause", TogglePlayPause)
	r.Register("play_pause", TogglePlayPause)

	r.Register("media_play", func() error {
		if strings.ToLower(r.getStatus()) == "playing" {
			return nil
		}
		return TogglePlayPause()
	})
	r.Register("play", func() error {
		if strings.ToLower(r.getStatus()) == "playing" {
			return nil
		}
		return TogglePlayPause()
	})

	r.Register("media_pause", func() error {
		if strings.ToLower(r.getStatus()) != "playing" {
			return nil
		}
		return TogglePlayPause()
	})
	r.Register("pause", func() error {
		if strings.ToLower(r.getStatus()) != "playing" {
			return nil
		}
		return TogglePlayPause()
	})

	r.Register("media_next", NextTrack)
	r.Register("next_track", NextTrack)
	r.Register("next", NextTrack)

	r.Register("media_prev", PreviousTrack)
	r.Register("media_previous", PreviousTrack)
	r.Register("previous_track", PreviousTrack)
	r.Register("prev_track", PreviousTrack)
	r.Register("prev", PreviousTrack)
	r.Register("previous", PreviousTrack)

	r.Register("media_stop", Stop)
	r.Register("stop", Stop)

	r.Register("volume_mute", ToggleMute)
	r.Register("mute", ToggleMute)

	r.Register("volume_up", VolumeUp)
	r.Register("volume_down", VolumeDown)

	r.Register("display_sleep", telemetry.SleepDisplays)
	r.Register("sleep_displays", telemetry.SleepDisplays)
	r.Register("monitor_off", telemetry.SleepDisplays)

	return r
}

func (r *Registry) SetStatusProvider(sp StatusProvider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.statusProvider = sp
}

func (r *Registry) getStatus() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.statusProvider != nil {
		return r.statusProvider()
	}
	return ""
}

func (r *Registry) Register(name string, handler Handler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[strings.ToLower(strings.TrimSpace(name))] = handler
}

func (r *Registry) Execute(actionName string) error {
	key := strings.ToLower(strings.TrimSpace(actionName))
	r.mu.RLock()
	handler, exists := r.handlers[key]
	r.mu.RUnlock()

	if !exists {
		logger.Warn("action", fmt.Sprintf("Unrecognized action requested: %s", actionName))
		return fmt.Errorf("%w: %s", ErrActionNotFound, actionName)
	}

	logger.Info("action", fmt.Sprintf("Executing action: %s", key))
	err := handler()
	if err != nil {
		logger.Error("action", fmt.Sprintf("Action %s failed: %v", key, err))
	}
	return err
}

func (r *Registry) ExecutePayload(raw []byte) error {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return errors.New("empty command payload")
	}

	var msg CommandMessage
	if err := json.Unmarshal(raw, &msg); err == nil {
		action := msg.Action
		if action == "" {
			action = msg.Command
		}
		if action != "" {
			return r.Execute(action)
		}
	}

	return r.Execute(trimmed)
}
