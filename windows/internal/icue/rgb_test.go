package icue

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDeviceColorsAndIdentity(t *testing.T) {
	d := device("private-device-serial", "Keyboard", 1, []LED{{ID: 8, R: 255, A: 255}, {ID: 2, B: 255, A: 255}})
	if d.Average != [3]uint8{128, 0, 128} || d.LEDs[0].ID != 2 {
		t.Fatalf("unexpected colours: %+v", d)
	}
	if device("private-device-serial", "Keyboard", 1, nil).ID != d.ID {
		t.Fatal("device identity changed")
	}
	if device("another-device", "Keyboard", 1, nil).ID == d.ID {
		t.Fatal("device identities collided")
	}
	raw, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "private-device-serial") {
		t.Fatal("raw device identifier leaked")
	}
	if device("empty", "empty", 0, nil).Average != [3]uint8{} {
		t.Fatal("empty average must be black")
	}
}

type fakeBackend struct{ reads, closes int }

func (f *fakeBackend) read() State { f.reads++; return unavailable("connected") }
func (f *fakeBackend) close()      { f.closes++ }

func TestReadRequiresOptInAndClosesWhenDisabled(t *testing.T) {
	original := reader.backend
	t.Cleanup(func() { reader.backend = original })
	f := &fakeBackend{}
	reader.backend = f
	if Read(false) != nil || f.reads != 0 {
		t.Fatal("disabled sensor was read")
	}
	if state := Read(true); state == nil || state.Status != "connected" || f.reads != 1 {
		t.Fatal("enabled sensor was not read")
	}
	if Read(false) != nil || f.reads != 1 || f.closes != 2 {
		t.Fatal("disable did not close the reader")
	}
	if reader.backend != f {
		t.Fatal("backend should be reused across toggles")
	}
}
