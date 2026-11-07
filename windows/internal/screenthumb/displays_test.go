package screenthumb

import (
	"errors"
	"image"
	"slices"
	"testing"
)

func TestDisplaySelectionPreservesPrimaryDefault(t *testing.T) {
	if !slices.Equal(SelectedDisplays(nil), []int{0}) || !slices.Equal(SelectedDisplays([]int{3, 1, 3}), []int{1, 3}) {
		t.Fatal("display selection was not normalized")
	}
	if DisplaySelected([]int{2}, 0) || DisplaySelected([]int{2}, 1) || !DisplaySelected([]int{2}, 2) {
		t.Fatal("capture must stay limited to selected displays")
	}
	for _, ids := range [][]int{{0}, {-1}, {MaxDisplays + 1}} {
		if ValidateDisplays(ids) == nil {
			t.Fatalf("accepted invalid display selection %v", ids)
		}
	}
	displays := []Display{{ID: 1}, {ID: 3, Primary: true}}
	if d, ok := ResolveDisplay(displays, 0); !ok || d.ID != 0 || !d.Primary {
		t.Fatal("legacy capture must resolve to the current primary monitor")
	}
	if _, ok := ResolveDisplay(displays, 2); ok {
		t.Fatal("a missing display must not fall back to another monitor")
	}
}

func TestManagerCapturesSelectedDisplaysAndHandlesTopologyChanges(t *testing.T) {
	store := NewStore()
	enabled, locked, ready := true, false, true
	selected := []int{1, 3}
	displays := []Display{{ID: 1, Name: "Display 1", Device: "a", Width: 1920, Height: 1080, Primary: true}, {ID: 2, Device: "b"}, {ID: 3, Name: "Display 3", Device: "c", X: -1080, Width: 1080, Height: 1920}}
	opened, closed, published := map[int]int{}, map[int]int{}, map[int]int{}
	var enumerationError error
	m := &Manager{
		Enabled: func() bool { return enabled }, Selected: func() []int { return selected }, Store: store,
		Displays: func() ([]Display, error) { return displays, enumerationError },
		NewRunner: func(d Display) (*Runner, func()) {
			opened[d.ID]++
			return &Runner{
				Display: d.ID, Store: store,
				Enabled: func() bool { return enabled && DisplaySelected(selected, d.ID) },
				Locked:  func() bool { return locked }, Ready: func() bool { return ready },
				Capture: func(side int) (image.Image, error) {
					w, h, err := ThumbSize(d.Width, d.Height, side)
					return image.NewRGBA(image.Rect(0, 0, w, h)), err
				},
				Publish: func([]byte) error { published[d.ID]++; return nil },
			}, func() { closed[d.ID]++ }
		},
	}
	t.Cleanup(m.Close)
	tick := func() {
		t.Helper()
		if err := m.Tick(); err != nil {
			t.Fatal(err)
		}
	}
	tick()
	if published[1] != 1 || published[3] != 1 || published[2] != 0 {
		t.Fatalf("published displays = %v", published)
	}
	locked = true
	tick()
	locked, ready = false, false
	tick()
	if published[1] != 1 || published[3] != 1 {
		t.Fatal("captured while locked or without a viewer")
	}
	ready = true
	displays = displays[:2]
	tick()
	if _, _, ok := store.LatestDisplay(3); ok || closed[3] != 1 {
		t.Fatal("disconnected display retained its frame or capture resources")
	}
	displays = append(displays, Display{ID: 3, Name: "Display 3", Device: "c", Width: 1920, Height: 1080})
	tick()
	if opened[3] != 2 || published[3] != 2 {
		t.Fatal("reconnected display did not resume")
	}
	displays[2].X = 1920
	tick()
	if opened[3] != 3 || closed[3] != 2 {
		t.Fatal("display movement did not recreate the capture source")
	}
	selected = []int{3}
	tick()
	if _, _, ok := store.LatestDisplay(1); ok || closed[1] != 1 {
		t.Fatal("deselected display retained its frame or capture resources")
	}
	enumerationError = errors.New("desktop unavailable")
	if m.Tick() == nil {
		t.Fatal("display enumeration error was lost")
	}
	if _, _, ok := store.LatestDisplay(3); ok {
		t.Fatal("enumeration failure left a stale frame")
	}
	enumerationError = nil
	tick()
	enabled = false
	tick()
	if _, _, ok := store.LatestDisplay(3); ok {
		t.Fatal("disabled thumbnails retained a frame")
	}
}

func TestHiddenAppsAreMaskedRelativeToEachDisplay(t *testing.T) {
	for _, display := range []Display{
		{X: -1000, Y: -500, Width: 1000, Height: 1000},
		{X: 1920, Y: 0, Width: 1000, Height: 1000},
	} {
		img := white(100, 100)
		hide := NewDisplayHider(display, func() []string { return []string{"private.exe"} }, func([]string) ([]Rect, int, int, error) {
			return []Rect{{X: display.X - 100, Y: display.Y + 100, W: 400, H: 200}}, 1920, 1080, nil
		})
		if err := hide(img); err != nil {
			t.Fatal(err)
		}
		if !isBlack(img, 0, 10) || !isBlack(img, 29, 29) || isBlack(img, 30, 29) || isBlack(img, 0, 30) {
			t.Fatalf("hidden window was masked on the wrong part of display %+v", display)
		}
	}
}

func TestRunnerDropsFrameIfDisabledDuringCapture(t *testing.T) {
	enabled := true
	store := NewStore()
	r := &Runner{
		Enabled: func() bool { return enabled }, Store: store,
		Capture: func(int) (image.Image, error) { enabled = false; return white(100, 100), nil },
		Publish: func([]byte) error { t.Fatal("published after disabling thumbnails"); return nil },
	}
	if result, err := r.Tick(); result != Skipped || err != nil {
		t.Fatalf("Tick = %v, %v", result, err)
	}
	if _, _, ok := store.Latest(); ok {
		t.Fatal("stored a disabled frame")
	}
}
