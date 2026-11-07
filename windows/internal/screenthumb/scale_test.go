package screenthumb

import (
	"image"
	"image/color"
	"testing"
)

func TestPickMipLevel(t *testing.T) {
	cases := []struct {
		w, h, max int
		want      int
	}{
		{1920, 1080, 200, 3},
		{2560, 1440, 200, 3},
		{3840, 2160, 200, 4},
		{1080, 1920, 200, 3},
		{1366, 768, 200, 2},
		{400, 300, 200, 1},
		{399, 300, 200, 0},
		{200, 100, 200, 0},
		{150, 100, 200, 0},
		{10000, 10, 200, 3},
	}
	for _, c := range cases {
		got := PickMipLevel(c.w, c.h, c.max)
		if got != c.want {
			t.Errorf("PickMipLevel(%d,%d,%d) = %d, want %d", c.w, c.h, c.max, got, c.want)
		}
		mw, mh := MipSize(c.w, c.h, got)
		if c.max <= max(c.w, c.h) && max(mw, mh) < c.max {
			t.Errorf("mip %dx%d for %dx%d is smaller than the %dpx target", mw, mh, c.w, c.h, c.max)
		}
	}
}

func TestPickMipLevelNeverGoesBelowOnePixel(t *testing.T) {
	level := PickMipLevel(10000, 3, 1)
	mw, mh := MipSize(10000, 3, level)
	if mw < 1 || mh < 1 {
		t.Fatalf("mip is %dx%d", mw, mh)
	}
}

func TestMipSize(t *testing.T) {
	cases := []struct{ w, h, level, wantW, wantH int }{
		{1920, 1080, 0, 1920, 1080},
		{1920, 1080, 3, 240, 135},
		{3840, 2160, 4, 240, 135},
		{10000, 3, 5, 312, 1},
	}
	for _, c := range cases {
		w, h := MipSize(c.w, c.h, c.level)
		if w != c.wantW || h != c.wantH {
			t.Errorf("MipSize(%d,%d,%d) = %dx%d, want %dx%d", c.w, c.h, c.level, w, h, c.wantW, c.wantH)
		}
	}
}

func solid(w, h int, c color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = c.R, c.G, c.B, c.A
	}
	return img
}

func TestBoxResizeKeepsSolidColours(t *testing.T) {
	out := BoxResize(solid(240, 135, color.RGBA{10, 120, 250, 255}), 200, 113)
	if b := out.Bounds(); b.Dx() != 200 || b.Dy() != 113 {
		t.Fatalf("size = %v", b)
	}
	for y := 0; y < 113; y++ {
		for x := 0; x < 200; x++ {
			if got := out.RGBAAt(x, y); got != (color.RGBA{10, 120, 250, 255}) {
				t.Fatalf("pixel %d,%d = %v", x, y, got)
			}
		}
	}
}

func TestBoxResizeAveragesNeighbours(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 4, 2))
	for y := 0; y < 2; y++ {
		for x := 0; x < 4; x++ {
			v := uint8(0)
			if x%2 == 1 {
				v = 200
			}
			src.SetRGBA(x, y, color.RGBA{v, v, v, 255})
		}
	}
	out := BoxResize(src, 2, 1)
	for x := 0; x < 2; x++ {
		if got := out.RGBAAt(x, 0); got != (color.RGBA{100, 100, 100, 255}) {
			t.Errorf("pixel %d = %v, want 100 grey", x, got)
		}
	}
}

func TestBoxResizeSameSizeIsUnchanged(t *testing.T) {
	src := solid(8, 4, color.RGBA{1, 2, 3, 255})
	if BoxResize(src, 8, 4) != src {
		t.Fatal("same-size resize should return the input")
	}
}

func TestBoxResizeKeepsLeftAndRightApart(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 240, 135))
	for y := 0; y < 135; y++ {
		for x := 0; x < 240; x++ {
			c := color.RGBA{255, 0, 0, 255}
			if x >= 120 {
				c = color.RGBA{0, 0, 255, 255}
			}
			src.SetRGBA(x, y, c)
		}
	}
	out := BoxResize(src, 200, 113)
	if got := out.RGBAAt(10, 50); got != (color.RGBA{255, 0, 0, 255}) {
		t.Errorf("left = %v", got)
	}
	if got := out.RGBAAt(190, 50); got != (color.RGBA{0, 0, 255, 255}) {
		t.Errorf("right = %v", got)
	}
}
