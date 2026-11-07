package icue

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"sync"
)

type LED struct {
	ID uint32 `json:"id"`
	R  uint8  `json:"r"`
	G  uint8  `json:"g"`
	B  uint8  `json:"b"`
	A  uint8  `json:"a"`
}

type Device struct {
	ID      string   `json:"id"`
	Model   string   `json:"model"`
	Type    uint32   `json:"type"`
	Average [3]uint8 `json:"average_rgb"`
	LEDs    []LED    `json:"leds"`
}

type State struct {
	Status  string   `json:"status"`
	Devices []Device `json:"devices"`
}

type backend interface {
	read() State
	close()
}

var reader struct {
	sync.Mutex
	backend backend
}

func Read(enabled bool) *State {
	reader.Lock()
	defer reader.Unlock()
	if !enabled {
		if reader.backend != nil {
			reader.backend.close()
		}
		return nil
	}
	if reader.backend == nil {
		reader.backend = newBackend()
	}
	state := reader.backend.read()
	return &state
}

func device(id, model string, kind uint32, leds []LED) Device {
	sort.Slice(leds, func(i, j int) bool { return leds[i].ID < leds[j].ID })
	d := Device{ID: fmt.Sprintf("%x", sha256.Sum256([]byte(id))), Model: model, Type: kind, LEDs: leds}
	var r, g, b uint64
	for _, led := range leds {
		r += uint64(led.R)
		g += uint64(led.G)
		b += uint64(led.B)
	}
	if n := uint64(len(leds)); n > 0 {
		d.Average = [3]uint8{uint8((r + n/2) / n), uint8((g + n/2) / n), uint8((b + n/2) / n)}
	}
	return d
}

func unavailable(status string) State {
	return State{Status: status, Devices: []Device{}}
}
