package screenthumb

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"satellite/internal/privacy"
)

type displayRunner struct {
	display Display
	runner  *Runner
	close   func()
}

type Manager struct {
	Enabled   func() bool
	Selected  func() []int
	Displays  func() ([]Display, error)
	NewRunner func(Display) (*Runner, func())
	Store     *Store

	mu      sync.Mutex
	active  map[int]displayRunner
	closed  bool
	running atomic.Bool
}

func (m *Manager) Tick() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil
	}
	var ids []int
	if !privacy.Default.Status().Paused && m.Enabled != nil && m.Enabled() {
		ids = SelectedDisplays(m.Selected())
	}
	if m.Store != nil {
		m.Store.KeepDisplays(ids)
	}
	var displays []Display
	var err error
	if len(ids) > 0 {
		displays, err = m.Displays()
	}
	wanted := make(map[int]Display)
	if err == nil {
		for _, id := range ids {
			if d, ok := ResolveDisplay(displays, id); ok {
				wanted[id] = d
			} else if m.Store != nil {
				m.Store.ClearDisplay(id)
			}
		}
	}
	for id, active := range m.active {
		if d, ok := wanted[id]; !ok || d != active.display {
			if active.close != nil {
				active.close()
			}
			delete(m.active, id)
			if m.Store != nil {
				m.Store.ClearDisplay(id)
			}
		}
	}
	if err != nil {
		return err
	}
	if m.active == nil {
		m.active = make(map[int]displayRunner)
	}
	var failures []error
	for _, id := range ids {
		d, ok := wanted[id]
		if !ok {
			continue
		}
		active, ok := m.active[id]
		if !ok {
			runner, close := m.NewRunner(d)
			active = displayRunner{display: d, runner: runner, close: close}
			m.active[id] = active
		}
		if _, err := active.runner.Tick(); err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", d.Name, err))
		}
	}
	return errors.Join(failures...)
}

func (m *Manager) Trigger(report func(error)) {
	if !m.running.CompareAndSwap(false, true) {
		return
	}
	go func() {
		defer m.running.Store(false)
		err := m.Tick()
		if report != nil {
			report(err)
		}
	}()
}

func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	for _, active := range m.active {
		if active.close != nil {
			active.close()
		}
	}
	m.active = nil
	if m.Store != nil {
		m.Store.KeepDisplays(nil)
	}
}
