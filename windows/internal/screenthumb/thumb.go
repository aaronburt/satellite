package screenthumb

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"sync/atomic"

	"satellite/internal/privacy"
)

const (
	// DefaultSide is the longest thumbnail edge unless configured otherwise.
	DefaultSide = 200
	// MinSide and MaxSide bound the configurable longest edge. MaxSide is a
	// hard privacy cap: above it, ordinary on-screen text becomes readable.
	MinSide     = 64
	MaxSide     = 400
	ContentType = "image/jpeg"

	jpegQuality = 75
)

var ErrUnsupported = errors.New("screen thumbnails are not supported on this platform")

// ClampSide returns side limited to [MinSide, MaxSide]; zero or negative
// means unset and gives DefaultSide.
func ClampSide(side int) int {
	if side <= 0 {
		return DefaultSide
	}
	return min(max(side, MinSide), MaxSide)
}

func ThumbSize(w, h, maxSide int) (int, int, error) {
	if w <= 0 || h <= 0 {
		return 0, 0, fmt.Errorf("invalid screen size %dx%d", w, h)
	}
	if maxSide <= 0 {
		return 0, 0, fmt.Errorf("invalid thumbnail size %d", maxSide)
	}
	long := max(w, h)
	if long <= maxSide {
		return w, h, nil
	}
	scale := func(side int) int {
		return max(1, (side*maxSide+long/2)/long)
	}
	if w >= h {
		return maxSide, scale(h), nil
	}
	return scale(w), maxSide, nil
}

func FromBGRA(buf []byte, w, h int) (*image.RGBA, error) {
	if w <= 0 || h <= 0 || len(buf) != w*h*4 {
		return nil, fmt.Errorf("pixel buffer is %d bytes, want %dx%dx4", len(buf), w, h)
	}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	convertBGRA(img.Pix, buf)
	return img, nil
}

func convertBGRA(dst, src []byte) {
	for i := 0; i < len(src); i += 4 {
		dst[i], dst[i+1], dst[i+2], dst[i+3] = src[i+2], src[i+1], src[i], 0xff
	}
}

func Encode(img image.Image) ([]byte, error) {
	b := img.Bounds()
	if b.Dx() <= 0 || b.Dy() <= 0 {
		return nil, errors.New("empty image")
	}
	if b.Dx() > MaxSide || b.Dy() > MaxSide {
		return nil, fmt.Errorf("image %dx%d is larger than the %dpx thumbnail limit", b.Dx(), b.Dy(), MaxSide)
	}
	var out bytes.Buffer
	if err := jpeg.Encode(&out, img, &jpeg.Options{Quality: jpegQuality}); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

type Runner struct {
	Display int
	Enabled func() bool
	// Ready reports whether anything wants a frame right now: a connected
	// MQTT broker or an open web UI page.
	Ready   func() bool
	Locked  func() bool
	Capture func(maxSide int) (image.Image, error)
	// Side returns the configured longest edge; it is clamped to
	// [MinSide, MaxSide]. Nil means DefaultSide.
	Side    func() int
	Publish func(jpeg []byte) error
	// Store, when set, keeps the latest frame for the web UI. It is cleared
	// while the sensor is disabled.
	Store *Store
	// Hide, when set, blacks out hidden apps' windows on the captured image.
	// If it fails the frame is dropped rather than published unmasked.
	Hide func(image.Image) error

	running atomic.Bool
}

type Result int

const (
	Skipped Result = iota
	Published
	Failed
)

func (r *Runner) Tick() (Result, error) {
	status := privacy.Default.Status()
	if status.Paused || r.Enabled == nil || !r.Enabled() {
		if r.Store != nil {
			r.Store.ClearDisplay(r.Display)
		}
		return Skipped, nil
	}
	if r.Ready != nil && !r.Ready() {
		return Skipped, nil
	}
	if r.Locked != nil && r.Locked() {
		return Skipped, nil
	}
	side := DefaultSide
	if r.Side != nil {
		side = r.Side()
	}
	img, err := r.Capture(ClampSide(side))
	if err != nil {
		return Failed, err
	}
	if r.Hide != nil {
		if err := r.Hide(img); err != nil {
			return Failed, fmt.Errorf("hiding apps: %w", err)
		}
	}
	data, err := Encode(img)
	if err != nil {
		return Failed, err
	}
	if !privacy.Default.Allows(status.Generation) || !r.Enabled() || (r.Locked != nil && r.Locked()) {
		if r.Store != nil && (!privacy.Default.Allows(status.Generation) || !r.Enabled()) {
			r.Store.ClearDisplay(r.Display)
		}
		return Skipped, nil
	}
	if r.Store != nil {
		r.Store.setDisplay(r.Display, data, status.Generation)
	}
	if !privacy.Default.Allows(status.Generation) {
		return Skipped, nil
	}
	if err := r.Publish(data); err != nil {
		return Failed, err
	}
	return Published, nil
}

func (r *Runner) Trigger(report func(error)) {
	if !r.running.CompareAndSwap(false, true) {
		return
	}
	go func() {
		defer r.running.Store(false)
		_, err := r.Tick()
		if report != nil {
			report(err)
		}
	}()
}

func LogOnChange(log func(error)) func(error) {
	last := ""
	return func(err error) {
		if err == nil {
			last = ""
			return
		}
		if err.Error() == last {
			return
		}
		last = err.Error()
		log(err)
	}
}
