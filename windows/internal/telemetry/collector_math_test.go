package telemetry

import (
	"testing"
	"time"
)

func TestCPUPercent(t *testing.T) {
	cases := []struct {
		name                string
		pIdle, pKern, pUser uint64
		idle, kern, user    uint64
		want                float64
		wantOK              bool
	}{
		// kernel includes idle: 100 ticks elapsed, 75 idle -> 25% busy.
		{"quarter busy", 1000, 2000, 500, 1075, 2080, 520, 25.0, true},
		{"fully idle", 0, 100, 0, 100, 200, 0, 0.0, true},
		{"fully busy", 0, 100, 0, 0, 150, 50, 100.0, true},
		{"rounds to one decimal", 0, 0, 0, 2, 3, 0, 33.3, true},
		{"no time elapsed", 10, 20, 30, 10, 20, 30, 0, false},
		{"counter went backwards", 100, 200, 300, 50, 250, 350, 0, false},
		{"idle exceeds total", 0, 0, 0, 100, 10, 0, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := cpuPercent(tc.pIdle, tc.pKern, tc.pUser, tc.idle, tc.kern, tc.user)
			if ok != tc.wantOK || (ok && got != tc.want) {
				t.Fatalf("cpuPercent = %v, %v; want %v, %v", got, ok, tc.want, tc.wantOK)
			}
		})
	}
}

func TestKBPerSecond(t *testing.T) {
	cases := []struct {
		name       string
		prev, curr uint64
		elapsed    time.Duration
		want       float64
		wantOK     bool
	}{
		{"one kB per second", 0, 1024, time.Second, 1.0, true},
		{"averaged over interval", 1000, 1000 + 10*1024, 5 * time.Second, 2.0, true},
		{"rounds to one decimal", 0, 1100, time.Second, 1.1, true},
		{"no traffic", 500, 500, time.Second, 0, true},
		{"counter reset", 5000, 100, time.Second, 0, false},
		{"zero elapsed", 0, 1024, 0, 0, false},
		{"negative elapsed", 0, 1024, -time.Second, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := kbPerSecond(tc.prev, tc.curr, tc.elapsed)
			if ok != tc.wantOK || (ok && got != tc.want) {
				t.Fatalf("kbPerSecond = %v, %v; want %v, %v", got, ok, tc.want, tc.wantOK)
			}
		})
	}
}
