//go:build windows

package screenthumb

import "testing"

func TestCaptureReturnsThumbnailOfPrimaryScreen(t *testing.T) {
	img, err := Capture(DefaultSide)
	if err != nil {
		t.Skipf("no capturable desktop here: %v", err)
	}
	b := img.Bounds()
	if max(b.Dx(), b.Dy()) > DefaultSide {
		t.Fatalf("capture is %dx%d, larger than %dpx", b.Dx(), b.Dy(), DefaultSide)
	}
	displays, err := Displays()
	if err != nil {
		t.Fatal(err)
	}
	display, ok := ResolveDisplay(displays, 0)
	if !ok {
		t.Fatal("primary display is missing")
	}
	sw, sh := display.Width, display.Height
	wantW, wantH, _ := ThumbSize(sw, sh, DefaultSide)
	if b.Dx() != wantW || b.Dy() != wantH {
		t.Fatalf("capture is %dx%d, want %dx%d for a %dx%d screen", b.Dx(), b.Dy(), wantW, wantH, sw, sh)
	}
	if _, err := Encode(img); err != nil {
		t.Fatal(err)
	}
}

func TestCaptureEachConnectedDisplay(t *testing.T) {
	displays, err := Displays()
	if err != nil || len(displays) == 0 {
		t.Skipf("no interactive desktop: %v", err)
	}
	seen := map[int]bool{}
	for _, display := range displays {
		if display.ID <= 0 || seen[display.ID] || display.Device == "" || display.Width <= 0 || display.Height <= 0 {
			t.Fatalf("invalid display inventory: %+v", displays)
		}
		seen[display.ID] = true
		t.Run(display.Name, func(t *testing.T) {
			img, err := CaptureDisplay(display, DefaultSide)
			if err != nil {
				t.Skipf("display cannot be captured in this session: %v", err)
			}
			w, h, _ := ThumbSize(display.Width, display.Height, DefaultSide)
			if img.Bounds().Dx() != w || img.Bounds().Dy() != h {
				t.Fatalf("capture size = %v, want %dx%d", img.Bounds(), w, h)
			}
		})
	}
}

func TestGPUOutputSelectionUsesDeviceName(t *testing.T) {
	c := &comCapturer{target: `\\.\DISPLAY3`}
	copy(c.outDesc.DeviceName[:], []uint16{'\\', '\\', '.', '\\', 'D', 'I', 'S', 'P', 'L', 'A', 'Y', '1'})
	if c.matchesOutput() {
		t.Fatal("selected the primary output for display 3")
	}
	c.outDesc.DeviceName[11] = '3'
	c.outDesc.Left = -1920
	if !c.matchesOutput() {
		t.Fatal("did not select display 3 at a negative desktop coordinate")
	}
}
