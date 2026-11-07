//go:build windows

package screenthumb

import "testing"

// CI runners may have no interactive desktop, so a failed capture skips.
// When capture works, the result must already be thumbnail-sized and match
// the screen's aspect ratio.
func TestCaptureReturnsThumbnailOfPrimaryScreen(t *testing.T) {
	img, err := Capture(MaxSide)
	if err != nil {
		t.Skipf("no capturable desktop here: %v", err)
	}
	b := img.Bounds()
	if max(b.Dx(), b.Dy()) > MaxSide {
		t.Fatalf("capture is %dx%d, larger than %dpx", b.Dx(), b.Dy(), MaxSide)
	}
	sw, _, _ := procGetSystemMetrics.Call(smCxScreen)
	sh, _, _ := procGetSystemMetrics.Call(smCyScreen)
	wantW, wantH, _ := ThumbSize(int(sw), int(sh), MaxSide)
	if b.Dx() != wantW || b.Dy() != wantH {
		t.Fatalf("capture is %dx%d, want %dx%d for a %dx%d screen", b.Dx(), b.Dy(), wantW, wantH, sw, sh)
	}
	if _, err := Encode(img); err != nil {
		t.Fatal(err)
	}
}
