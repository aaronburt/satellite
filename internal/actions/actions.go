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
	return &Registry{
		handlers: make(map[string]Handler),
	}
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
	customHandler, hasCustom := r.handlers[key]
	r.mu.RUnlock()

	if hasCustom {
		return customHandler()
	}

	logger.Info("action", fmt.Sprintf("Executing action: %s", key))

	var err error
	switch key {
	case "lock", "lock_workstation":
		err = LockWorkstation()
	case "media_play_pause", "play_pause":
		err = TogglePlayPause()
	case "media_play", "play":
		if strings.ToLower(r.getStatus()) != "playing" {
			err = TogglePlayPause()
		}
	case "media_pause", "pause":
		if strings.ToLower(r.getStatus()) == "playing" {
			err = TogglePlayPause()
		}
	case "media_next", "next_track", "next":
		err = NextTrack()
	case "media_prev", "media_previous", "previous_track", "prev_track", "prev", "previous":
		err = PreviousTrack()
	case "media_stop", "stop":
		err = Stop()
	case "volume_mute", "mute":
		err = ToggleMute()
	case "volume_up":
		err = VolumeUp()
	case "volume_down":
		err = VolumeDown()
	case "display_sleep", "sleep_displays", "monitor_off":
		err = telemetry.SleepDisplays()
	default:
		logger.Warn("action", fmt.Sprintf("Unrecognized action requested: %s", actionName))
		return fmt.Errorf("%w: %s", ErrActionNotFound, actionName)
	}

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
