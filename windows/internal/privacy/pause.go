package privacy

import (
	"sync"
	"time"
)

var Default = &Controller{}

type Status struct {
	Paused           bool      `json:"paused"`
	Until            time.Time `json:"until"`
	RemainingSeconds int       `json:"remaining_seconds"`
	Generation       uint64    `json:"-"`
}

type Controller struct {
	mu         sync.RWMutex
	until      time.Time
	generation uint64
	now        func() time.Time
}

func (c *Controller) clock() time.Time {
	if c.now != nil {
		return c.now().Round(0)
	}
	return time.Now().Round(0)
}

func (c *Controller) Pause(duration time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.generation++
	if duration <= 0 {
		c.until = time.Time{}
	} else {
		c.until = c.clock().Add(duration)
	}
}

func (c *Controller) Resume() { c.Pause(0) }

func (c *Controller) Status() Status {
	c.mu.RLock()
	defer c.mu.RUnlock()
	remaining := c.until.Sub(c.clock())
	s := Status{Until: c.until, Generation: c.generation + 1}
	if remaining > 0 {
		s.Paused = true
		s.RemainingSeconds = int((remaining + time.Second - 1) / time.Second)
	}
	return s
}

func (c *Controller) Allows(generation uint64) bool {
	s := c.Status()
	return !s.Paused && (generation == 0 || generation == s.Generation)
}
