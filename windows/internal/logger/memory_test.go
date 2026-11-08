package logger

import (
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"satellite/internal/config"
)

type memoryTransport struct {
	release   chan struct{}
	active    atomic.Int64
	completed atomic.Int64
}

func (m *memoryTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	m.active.Add(1)
	<-m.release
	m.active.Add(-1)
	m.completed.Add(1)
	return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
}

func TestLogBurstMemory(t *testing.T) {
	if os.Getenv("SATELLITE_MEMORY_BENCH") != "1" {
		t.Skip("set SATELLITE_MEMORY_BENCH=1 to measure a blocked webhook burst")
	}
	transport := &memoryTransport{release: make(chan struct{})}
	previous := httpClient
	httpClient = &http.Client{Transport: transport}
	var once sync.Once
	defer func() {
		once.Do(func() { close(transport.release) })
		UpdateConfig(config.WebhookConfig{}, "")
		httpClient = previous
	}()
	UpdateConfig(config.WebhookConfig{Enabled: true, URL: "http://memory.test"}, "bench")
	runtime.GC()
	var before, during runtime.MemStats
	runtime.ReadMemStats(&before)
	beforeG := runtime.NumGoroutine()
	for i := 0; i < 1000; i++ {
		Info("memory", strings.Repeat("x", 1024))
	}
	time.Sleep(250 * time.Millisecond)
	runtime.GC()
	runtime.ReadMemStats(&during)
	t.Logf("1000 events, 1024-byte messages: heap_delta=%d stack_delta=%d total_alloc_delta=%d goroutine_delta=%d active_requests=%d", int64(during.HeapAlloc)-int64(before.HeapAlloc), int64(during.StackInuse)-int64(before.StackInuse), during.TotalAlloc-before.TotalAlloc, runtime.NumGoroutine()-beforeG, transport.active.Load())
	once.Do(func() { close(transport.release) })
	deadline := time.Now().Add(3 * time.Second)
	last := int64(-1)
	for time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
		n := transport.completed.Load()
		if n == last && transport.active.Load() == 0 {
			return
		}
		last = n
	}
	t.Fatal("webhook requests did not drain")
}
