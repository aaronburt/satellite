package logger

import (
	"errors"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"satellite/internal/config"
)

func TestLogBurstBoundsDeliveryAndPreservesListeners(t *testing.T) {
	resetLogger(t)
	started := make(chan struct{}, eventQueueCapacity+1)
	release := make(chan struct{})
	delivered := make(chan eventDelivery, eventQueueCapacity+1)
	d := newEventDispatcher(func(url, secret string, event Event) error {
		started <- struct{}{}
		<-release
		delivered <- eventDelivery{url: url, secret: secret, event: event}
		return errors.New("unavailable")
	})
	previous := webhookDelivery
	webhookDelivery = d
	var releaseOnce sync.Once
	defer func() {
		webhookDelivery = previous
		releaseOnce.Do(func() { close(release) })
		close(d.queue)
	}()
	var localCount atomic.Int64
	unsubscribe := AddListener(func(Event) { localCount.Add(1) })
	defer unsubscribe()
	UpdateConfig(config.WebhookConfig{Enabled: true, URL: "http://original.test", Secret: "original"}, "desk")
	Info("burst", "0")
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("delivery worker did not start")
	}
	for i := 1; i <= eventQueueCapacity; i++ {
		Info("burst", strconv.Itoa(i))
	}
	UpdateConfig(config.WebhookConfig{Enabled: true, URL: "http://new.test", Secret: "new"}, "desk")
	var producers sync.WaitGroup
	for i := 0; i < 1000; i++ {
		producers.Go(func() { Info("burst", "overflow") })
	}
	done := make(chan struct{})
	go func() { producers.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("logging blocked on a full queue")
	}
	if got := localCount.Load(); got != 1001+eventQueueCapacity {
		t.Fatalf("local listeners received %d events", got)
	}
	if len(started) != 0 || len(d.queue) != eventQueueCapacity {
		t.Fatalf("unexpected concurrency or queue size: started=%d queued=%d", len(started), len(d.queue))
	}
	releaseOnce.Do(func() { close(release) })
	for i := 0; i <= eventQueueCapacity; i++ {
		select {
		case got := <-delivered:
			if got.event.Message != strconv.Itoa(i) || got.url != "http://original.test" || got.secret != "original" {
				t.Fatalf("delivery %d = %+v", i, got)
			}
		case <-time.After(time.Second):
			t.Fatal("queue stopped draining after a delivery error")
		}
	}
}
