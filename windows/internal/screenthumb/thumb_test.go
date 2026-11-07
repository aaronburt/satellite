package screenthumb

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"sync"
	"testing"
	"time"
)

func TestThumbSizeKeepsAspectRatio(t *testing.T) {
	cases := []struct {
		name         string
		w, h         int
		wantW, wantH int
	}{
		{"1080p landscape", 1920, 1080, 200, 113},
		{"1440p", 2560, 1440, 200, 113},
		{"4k", 3840, 2160, 200, 113},
		{"16:10", 1920, 1200, 200, 125},
		{"ultrawide", 3440, 1440, 200, 84},
		{"4:3", 1024, 768, 200, 150},
		{"portrait", 1080, 1920, 113, 200},
		{"square", 2000, 2000, 200, 200},
		{"extreme strip keeps 1px", 10000, 10, 200, 1},
		{"already small is not enlarged", 160, 90, 160, 90},
		{"exactly max", 200, 80, 200, 80},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w, h, err := ThumbSize(c.w, c.h, MaxSide)
			if err != nil {
				t.Fatal(err)
			}
			if w != c.wantW || h != c.wantH {
				t.Fatalf("ThumbSize(%d,%d) = %dx%d, want %dx%d", c.w, c.h, w, h, c.wantW, c.wantH)
			}
			if max(w, h) > MaxSide {
				t.Fatalf("longest side %d exceeds %d", max(w, h), MaxSide)
			}
		})
	}
}

func TestMaxSideIs200(t *testing.T) {
	if MaxSide != 200 {
		t.Fatalf("MaxSide = %d, want 200", MaxSide)
	}
}

func TestThumbSizeRejectsBadInput(t *testing.T) {
	for _, in := range [][3]int{{0, 1080, 200}, {1920, 0, 200}, {-1, 5, 200}, {1920, 1080, 0}} {
		if _, _, err := ThumbSize(in[0], in[1], in[2]); err == nil {
			t.Errorf("ThumbSize%v: want error", in)
		}
	}
}

func TestFromBGRASwapsChannelsAndSetsOpaque(t *testing.T) {
	buf := []byte{255, 0, 0, 0, 0, 0, 255, 0}
	img, err := FromBGRA(buf, 2, 1)
	if err != nil {
		t.Fatal(err)
	}
	if got := img.RGBAAt(0, 0); got != (color.RGBA{0, 0, 255, 255}) {
		t.Errorf("pixel 0 = %v, want blue", got)
	}
	if got := img.RGBAAt(1, 0); got != (color.RGBA{255, 0, 0, 255}) {
		t.Errorf("pixel 1 = %v, want red", got)
	}
}

func TestFromBGRARejectsWrongLength(t *testing.T) {
	if _, err := FromBGRA(make([]byte, 7), 2, 1); err == nil {
		t.Fatal("want error for short buffer")
	}
}

func TestEncodeProducesSmallJPEG(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 200, 113))
	data, err := Encode(img)
	if err != nil {
		t.Fatal(err)
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if format != "jpeg" || cfg.Width != 200 || cfg.Height != 113 {
		t.Fatalf("decoded %s %dx%d", format, cfg.Width, cfg.Height)
	}
	if _, err := jpeg.Decode(bytes.NewReader(data)); err != nil {
		t.Fatal(err)
	}
}

func TestEncodeRefusesFullSizeImages(t *testing.T) {
	for _, r := range []image.Rectangle{
		image.Rect(0, 0, 1920, 1080),
		image.Rect(0, 0, 201, 50),
		image.Rect(0, 0, 50, 201),
		image.Rect(0, 0, 0, 0),
	} {
		if _, err := Encode(image.NewRGBA(r)); err == nil {
			t.Errorf("Encode(%v): want error", r)
		}
	}
}

type fakes struct {
	enabled, ready, locked bool
	captured               int
	maxSide                int
	img                    image.Image
	captureErr             error
	published              [][]byte
	publishErr             error
}

func (f *fakes) runner() *Runner {
	return &Runner{
		Enabled: func() bool { return f.enabled },
		Ready:   func() bool { return f.ready },
		Locked:  func() bool { return f.locked },
		Capture: func(m int) (image.Image, error) {
			f.captured++
			f.maxSide = m
			return f.img, f.captureErr
		},
		Publish: func(b []byte) error {
			f.published = append(f.published, b)
			return f.publishErr
		},
	}
}

func newFakes() *fakes {
	return &fakes{enabled: true, ready: true, img: image.NewRGBA(image.Rect(0, 0, 200, 113))}
}

func TestTickPublishesThumbnail(t *testing.T) {
	f := newFakes()
	res, err := f.runner().Tick()
	if err != nil || res != Published {
		t.Fatalf("Tick = %v, %v", res, err)
	}
	if f.maxSide != MaxSide {
		t.Errorf("capture asked for %d px, want %d", f.maxSide, MaxSide)
	}
	if len(f.published) != 1 || !bytes.HasPrefix(f.published[0], []byte{0xff, 0xd8}) {
		t.Fatalf("published %d items, want one JPEG", len(f.published))
	}
}

func TestTickDoesNotCaptureUnlessAllowed(t *testing.T) {
	cases := map[string]func(*fakes){
		"disabled":     func(f *fakes) { f.enabled = false },
		"no output":    func(f *fakes) { f.ready = false },
		"session lock": func(f *fakes) { f.locked = true },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			f := newFakes()
			mutate(f)
			res, err := f.runner().Tick()
			if err != nil || res != Skipped {
				t.Fatalf("Tick = %v, %v; want Skipped", res, err)
			}
			if f.captured != 0 {
				t.Fatal("the screen must not be captured")
			}
		})
	}
}

func TestTickWithNilEnabledIsOff(t *testing.T) {
	f := newFakes()
	r := f.runner()
	r.Enabled = nil
	if res, _ := r.Tick(); res != Skipped || f.captured != 0 {
		t.Fatal("a runner without an Enabled check must stay off")
	}
}

func TestTickNeverPublishesOversizedCapture(t *testing.T) {
	f := newFakes()
	f.img = image.NewRGBA(image.Rect(0, 0, 1920, 1080))
	res, err := f.runner().Tick()
	if res != Failed || err == nil {
		t.Fatalf("Tick = %v, %v; want failure", res, err)
	}
	if len(f.published) != 0 {
		t.Fatal("a full-size frame was published")
	}
}

func TestTickReportsCaptureAndPublishErrors(t *testing.T) {
	f := newFakes()
	f.captureErr = errors.New("secure desktop")
	if res, err := f.runner().Tick(); res != Failed || err == nil || len(f.published) != 0 {
		t.Fatalf("capture error: Tick = %v, %v", res, err)
	}

	f = newFakes()
	f.publishErr = errors.New("broker gone")
	if res, err := f.runner().Tick(); res != Failed || err == nil {
		t.Fatalf("publish error: Tick = %v, %v", res, err)
	}
}

func TestTriggerRunsATickInTheBackground(t *testing.T) {
	f := newFakes()
	r := f.runner()
	done := make(chan error, 1)
	r.Trigger(func(err error) { done <- err })
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Trigger never ran")
	}
	if len(f.published) != 1 {
		t.Fatalf("published %d thumbnails, want 1", len(f.published))
	}
}

func TestTriggerSkipsWhileACaptureIsStillRunning(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	var mu sync.Mutex
	captures := 0
	r := &Runner{
		Enabled: func() bool { return true },
		Capture: func(int) (image.Image, error) {
			mu.Lock()
			captures++
			mu.Unlock()
			close(started)
			<-release
			return image.NewRGBA(image.Rect(0, 0, 10, 10)), nil
		},
		Publish: func([]byte) error { return nil },
	}
	done := make(chan struct{})
	r.Trigger(func(error) { close(done) })
	<-started
	for i := 0; i < 5; i++ {
		r.Trigger(nil)
	}
	close(release)
	<-done

	mu.Lock()
	defer mu.Unlock()
	if captures != 1 {
		t.Fatalf("captures = %d, want 1: a slow capture must not stack up", captures)
	}
}

func TestTriggerCanRunAgainAfterFinishing(t *testing.T) {
	f := newFakes()
	r := f.runner()
	for i := 0; i < 2; i++ {
		done := make(chan struct{})
		r.Trigger(func(error) { close(done) })
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatalf("trigger %d never ran", i)
		}
		for r.running.Load() {
			time.Sleep(time.Millisecond)
		}
	}
	if len(f.published) != 2 {
		t.Fatalf("published %d, want 2", len(f.published))
	}
}

func TestLogOnChangeLogsRepeatedFailureOnce(t *testing.T) {
	var logged []string
	report := LogOnChange(func(err error) { logged = append(logged, err.Error()) })
	a, b := errors.New("a"), errors.New("b")
	for _, err := range []error{a, a, a, b, b, nil, nil, b} {
		report(err)
	}
	want := []string{"a", "b", "b"}
	if len(logged) != len(want) {
		t.Fatalf("logged %v, want %v", logged, want)
	}
	for i := range want {
		if logged[i] != want[i] {
			t.Fatalf("logged %v, want %v", logged, want)
		}
	}
}
