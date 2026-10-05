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

type Executor interface {
	LockWorkstation() error
	TogglePlayPause() error
	NextTrack() error
	PreviousTrack() error
	Stop() error
	ToggleMute() error
	VolumeUp() error
	VolumeDown() error
	SleepDisplays() error
}

type systemExecutor struct{}

func (systemExecutor) LockWorkstation() error {
	return LockWorkstation()
}

func (systemExecutor) TogglePlayPause() error {
	return TogglePlayPause()
}

func (systemExecutor) NextTrack() error {
	return NextTrack()
}

func (systemExecutor) PreviousTrack() error {
	return PreviousTrack()
}

func (systemExecutor) Stop() error {
	return Stop()
}

func (systemExecutor) ToggleMute() error {
	return ToggleMute()
}

func (systemExecutor) VolumeUp() error {
	return VolumeUp()
}

func (systemExecutor) VolumeDown() error {
	return VolumeDown()
}

func (systemExecutor) SleepDisplays() error {
	return telemetry.SleepDisplays()
}

type Registry struct {
	mu             sync.RWMutex
	handlers       map[string]Handler
	statusProvider StatusProvider
	executor       Executor
}

type CommandMessage struct {
	Action  string `json:"action"`
	Command string `json:"command"`
}

func NewRegistry() *Registry {
	return NewRegistryWithExecutor(systemExecutor{})
}

func NewRegistryWithExecutor(exec Executor) *Registry {
	if exec == nil {
		exec = systemExecutor{}
	}
	return &Registry{
		handlers: make(map[string]Handler),
		executor: exec,
	}
}

func (r *Registry) getExecutor() Executor {
	if r.executor != nil {
		return r.executor
	}
	return systemExecutor{}
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

func normalizeAction(name string) string {
	key := strings.ToLower(strings.TrimSpace(name))
	switch key {
	case "lock", "lock_workstation":
		return "lock"
	case "media_play_pause", "play_pause":
		return "media_play_pause"
	case "media_play", "play":
		return "media_play"
	case "media_pause", "pause":
		return "media_pause"
	case "media_next", "next_track", "next":
		return "media_next"
	case "media_prev", "media_previous", "previous_track", "prev_track", "prev", "previous":
		return "media_prev"
	case "media_stop", "stop":
		return "media_stop"
	case "volume_mute", "mute":
		return "volume_mute"
	case "volume_up":
		return "volume_up"
	case "volume_down":
		return "volume_down"
	case "display_sleep", "sleep_displays", "monitor_off":
		return "display_sleep"
	default:
		return key
	}
}

func (r *Registry) Register(name string, handler Handler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[normalizeAction(name)] = handler
}

func (r *Registry) Execute(actionName string) error {
	key := normalizeAction(actionName)

	r.mu.RLock()
	customHandler, hasCustom := r.handlers[key]
	r.mu.RUnlock()

	if hasCustom {
		return customHandler()
	}

	logger.Info("action", fmt.Sprintf("Executing action: %s", key))

	var err error
	exec := r.getExecutor()
	switch key {
	case "lock":
		err = exec.LockWorkstation()
	case "media_play_pause":
		err = exec.TogglePlayPause()
	case "media_play":
		if strings.ToLower(r.getStatus()) != "playing" {
			err = exec.TogglePlayPause()
		}
	case "media_pause":
		if strings.ToLower(r.getStatus()) == "playing" {
			err = exec.TogglePlayPause()
		}
	case "media_next":
		err = exec.NextTrack()
	case "media_prev":
		err = exec.PreviousTrack()
	case "media_stop":
		err = exec.Stop()
	case "volume_mute":
		err = exec.ToggleMute()
	case "volume_up":
		err = exec.VolumeUp()
	case "volume_down":
		err = exec.VolumeDown()
	case "display_sleep":
		err = exec.SleepDisplays()
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
