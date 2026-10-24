// Package collectloop runs the periodic telemetry collection and lets the
// interval change while it runs, without starting a second timer.
package collectloop

import (
	"context"
	"sync"
	"time"
)

// DefaultInterval is used when the configured interval is below one second,
// matching the config normalization of interval_sec <= 0 to 5.
const DefaultInterval = 5 * time.Second

// Normalize turns an interval_sec value into the duration the loop uses.
func Normalize(sec int) time.Duration {
	if sec < 1 {
		return DefaultInterval
	}
	return time.Duration(sec) * time.Second
}

// Loop calls tick once per interval on a single goroutine.
type Loop struct {
	tick    func()
	updates chan time.Duration // size 1: only the latest pending change is kept

	mu       sync.Mutex
	interval time.Duration // last requested interval, for Interval()

	newTicker func(time.Duration) ticker
}

type ticker interface {
	C() <-chan time.Time
	Reset(time.Duration)
	Stop()
}

type realTicker struct{ t *time.Ticker }

func (r realTicker) C() <-chan time.Time   { return r.t.C }
func (r realTicker) Reset(d time.Duration) { r.t.Reset(d) }
func (r realTicker) Stop()                 { r.t.Stop() }

// New creates a loop that calls tick every intervalSec seconds once Run starts.
func New(intervalSec int, tick func()) *Loop {
	return &Loop{
		tick:      tick,
		updates:   make(chan time.Duration, 1),
		interval:  Normalize(intervalSec),
		newTicker: func(d time.Duration) ticker { return realTicker{time.NewTicker(d)} },
	}
}

// Interval reports the most recently requested interval.
func (l *Loop) Interval() time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.interval
}

// SetInterval changes the interval of a running loop. It never blocks; when
// several changes arrive before the loop sees them, only the last applies.
// The next collection happens one full new interval after the change.
func (l *Loop) SetInterval(intervalSec int) {
	d := Normalize(intervalSec)
	l.mu.Lock()
	defer l.mu.Unlock()
	if d == l.interval {
		return
	}
	l.interval = d
	select {
	case <-l.updates: // drop a stale pending change
	default:
	}
	l.updates <- d // cannot block: we hold mu and just drained the slot
}

// Run collects until ctx is cancelled. Call it once.
func (l *Loop) Run(ctx context.Context) {
	current := l.Interval()
	t := l.newTicker(current)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case d := <-l.updates:
			if d != current {
				current = d
				t.Reset(d)
			}
		case <-t.C():
			if ctx.Err() != nil {
				return
			}
			l.tick()
		}
	}
}
