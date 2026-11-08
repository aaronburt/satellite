package logger

import "sync"

const eventQueueCapacity = 64

type eventDelivery struct {
	url    string
	secret string
	event  Event
}

type eventDispatcher struct {
	once  sync.Once
	queue chan eventDelivery
	send  func(string, string, Event) error
}

func newEventDispatcher(send func(string, string, Event) error) *eventDispatcher {
	return &eventDispatcher{queue: make(chan eventDelivery, eventQueueCapacity), send: send}
}

func (d *eventDispatcher) submit(delivery eventDelivery) bool {
	d.once.Do(func() {
		go func() {
			for delivery := range d.queue {
				_ = d.send(delivery.url, delivery.secret, delivery.event)
			}
		}()
	})
	select {
	case d.queue <- delivery:
		return true
	default:
		return false
	}
}
