package screenthumb

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	_ "image/jpeg"
	"reflect"
	"testing"
)

func white(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := range img.Pix {
		img.Pix[i] = 0xff
	}
	return img
}

func isBlack(img *image.RGBA, x, y int) bool {
	return img.RGBAAt(x, y) == color.RGBA{A: 0xff}
}

func TestNormalizeApps(t *testing.T) {
	in := []string{" Chrome.EXE ", `C:\Program Files\Mozilla\Firefox.exe`, "chrome.exe", "", "  ", "/usr/bin/vlc", ".", "Edge"}
	want := []string{"chrome.exe", "firefox.exe", "vlc", "edge"}
	if got := NormalizeApps(in); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	if got := NormalizeApps(nil); got != nil {
		t.Fatalf("nil list gave %v", got)
	}
}

func TestNormalizeAppsCapsLength(t *testing.T) {
	var in []string
	for i := 0; i < MaxHiddenApps+20; i++ {
		in = append(in, string(rune('a'+i%26))+string(rune('a'+i/26))+".exe")
	}
	if got := NormalizeApps(in); len(got) != MaxHiddenApps {
		t.Fatalf("kept %d, want %d", len(got), MaxHiddenApps)
	}
}

func TestMatchApp(t *testing.T) {
	apps := NormalizeApps([]string{"chrome.exe", "vlc"})
	for proc, want := range map[string]bool{
		"chrome.exe": true,
		"CHROME.EXE": true,
		`C:\Program Files\Google\Chrome\Application\chrome.exe`: true,
		"chrome":        true,
		"vlc.exe":       true,
		"firefox.exe":   false,
		"chrome2.exe":   false,
		"notchrome.exe": false,
		"":              false,
		".":             false,
	} {
		if got := MatchApp(proc, apps); got != want {
			t.Errorf("MatchApp(%q) = %v, want %v", proc, got, want)
		}
	}
	if MatchApp("chrome.exe", nil) {
		t.Error("empty list must match nothing")
	}
}

func TestMaskImagePaintsScaledRectBlack(t *testing.T) {
	img := white(200, 100)
	// A 960x540 window at the top left of a 1920x1080 screen is a quarter of
	// each side: 100x50 thumbnail pixels.
	if err := MaskImage(img, 1920, 1080, []Rect{{0, 0, 960, 540}}); err != nil {
		t.Fatal(err)
	}
	if !isBlack(img, 0, 0) || !isBlack(img, 99, 49) {
		t.Error("window area not black")
	}
	if isBlack(img, 100, 49) || isBlack(img, 99, 50) || isBlack(img, 199, 99) {
		t.Error("area outside the window was blacked out")
	}
}

func TestMaskImageRoundsOutward(t *testing.T) {
	img := white(10, 10)
	// 1 screen px of a 100 px screen is 0.1 thumbnail px; it must still cover
	// the whole pixel it touches.
	if err := MaskImage(img, 100, 100, []Rect{{15, 15, 1, 1}}); err != nil {
		t.Fatal(err)
	}
	if !isBlack(img, 1, 1) {
		t.Error("touched pixel not blacked out")
	}
	n := 0
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			if isBlack(img, x, y) {
				n++
			}
		}
	}
	if n != 1 {
		t.Errorf("%d pixels black, want 1", n)
	}
}

func TestMaskImageClipsToImage(t *testing.T) {
	img := white(20, 10)
	rects := []Rect{{-500, -500, 600, 600}, {1900, 1000, 500, 500}, {3000, 3000, 10, 10}, {0, 0, 0, 5}, {5, 5, -3, 4}}
	if err := MaskImage(img, 1920, 1080, rects); err != nil {
		t.Fatal(err)
	}
	if !isBlack(img, 0, 0) || !isBlack(img, 19, 9) {
		t.Error("clipped rectangles did not cover the corners")
	}
	if isBlack(img, 19, 0) || isBlack(img, 0, 9) {
		t.Error("clipping blacked out too much")
	}
}

func TestMaskImageErrors(t *testing.T) {
	if err := MaskImage(white(4, 4), 0, 100, []Rect{{0, 0, 1, 1}}); err == nil {
		t.Error("zero screen size: want error")
	}
	if err := MaskImage(image.NewGray(image.Rect(0, 0, 4, 4)), 100, 100, []Rect{{0, 0, 1, 1}}); err == nil {
		t.Error("unpaintable image type: want error")
	}
	if err := MaskImage(image.NewGray(image.Rect(0, 0, 4, 4)), 0, 0, nil); err != nil {
		t.Errorf("nothing to hide must be a no-op, got %v", err)
	}
}

func TestHiderSkipsLookupWithEmptyList(t *testing.T) {
	called := false
	find := func([]string) ([]Rect, int, int, error) { called = true; return nil, 0, 0, nil }
	for _, apps := range []func() []string{nil, func() []string { return nil }, func() []string { return []string{" ", ""} }} {
		if err := NewHider(apps, find)(white(4, 4)); err != nil {
			t.Fatal(err)
		}
	}
	if called {
		t.Fatal("window lookup ran with nothing on the list")
	}
}

func TestHiderPassesNormalizedListAndMasks(t *testing.T) {
	var got []string
	find := func(apps []string) ([]Rect, int, int, error) {
		got = apps
		return []Rect{{0, 0, 50, 50}}, 100, 100, nil
	}
	img := white(10, 10)
	if err := NewHider(func() []string { return []string{" Chrome.EXE "} }, find)(img); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []string{"chrome.exe"}) {
		t.Fatalf("lookup got %v", got)
	}
	if !isBlack(img, 0, 0) || isBlack(img, 9, 9) {
		t.Error("window not masked")
	}
}

func TestHiderLookupErrorIsReturned(t *testing.T) {
	boom := errors.New("boom")
	find := func([]string) ([]Rect, int, int, error) { return nil, 0, 0, boom }
	if err := NewHider(func() []string { return []string{"a.exe"} }, find)(white(4, 4)); !errors.Is(err, boom) {
		t.Fatalf("got %v, want boom", err)
	}
}

func TestRunnerDropsFrameWhenHidingFails(t *testing.T) {
	published := false
	r := &Runner{
		Enabled: func() bool { return true },
		Capture: func(int) (image.Image, error) { return white(4, 4), nil },
		Hide:    func(image.Image) error { return errors.New("no") },
		Publish: func([]byte) error { published = true; return nil },
		Store:   NewStore(),
	}
	if res, err := r.Tick(); res != Failed || err == nil {
		t.Fatalf("Tick = %v, %v; want Failed with error", res, err)
	}
	if published {
		t.Fatal("an unmasked frame was published")
	}
	if _, _, ok := r.Store.Latest(); ok {
		t.Fatal("an unmasked frame was stored")
	}
}

func TestRunnerPublishesMaskedFrame(t *testing.T) {
	var data []byte
	r := &Runner{
		Enabled: func() bool { return true },
		Capture: func(int) (image.Image, error) { return white(8, 8), nil },
		Hide:    func(img image.Image) error { return MaskImage(img, 8, 8, []Rect{{0, 0, 8, 8}}) },
		Publish: func(b []byte) error { data = b; return nil },
	}
	if res, err := r.Tick(); res != Published || err != nil {
		t.Fatalf("Tick = %v, %v", res, err)
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if r, g, b, _ := img.At(4, 4).RGBA(); r>>8 > 8 || g>>8 > 8 || b>>8 > 8 {
		t.Fatalf("published pixel is not black: %d %d %d", r>>8, g>>8, b>>8)
	}
}
