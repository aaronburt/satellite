// Package screenthumb publishes a tiny, low-resolution thumbnail of the
// screen. It is off by default and must be switched on in settings.
//
// Privacy: the screen is scaled down by the operating system while it is
// copied, so a full-resolution image of the screen never exists in this
// process. Only the small thumbnail (at most MaxSide pixels on its longest
// side) is encoded and handed on. Nothing is written to disk, and only the
// most recent thumbnail is kept in memory until the next capture.
package screenthumb

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"time"
)

const (
	// Interval is how often a thumbnail is captured.
	Interval = 30 * time.Second
	// MaxSide is the longest side of a thumbnail, in pixels.
	MaxSide = 100
	// ContentType is the MIME type of an encoded thumbnail.
	ContentType = "image/jpeg"

	jpegQuality = 75
)

// ErrUnsupported is returned by Capture on platforms without screen capture.
var ErrUnsupported = errors.New("screen thumbnails are not supported on this platform")

// ThumbSize scales a w x h screen so its longest side is maxSide, keeping the
// aspect ratio. The short side is rounded and never below 1 pixel. Screens
// already smaller than maxSide are not enlarged.
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
		// Round to nearest: side*maxSide/long, in integers.
		return max(1, (side*maxSide+long/2)/long)
	}
	if w >= h {
		return maxSide, scale(h), nil
	}
	return scale(w), maxSide, nil
}

// FromBGRA turns a top-down 32-bit BGRA pixel buffer (as Windows GDI returns
// it) into an RGBA image. Alpha is set to opaque because GDI leaves it 0.
func FromBGRA(buf []byte, w, h int) (*image.RGBA, error) {
	if w <= 0 || h <= 0 || len(buf) != w*h*4 {
		return nil, fmt.Errorf("pixel buffer is %d bytes, want %dx%dx4", len(buf), w, h)
	}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < len(buf); i += 4 {
		img.Pix[i+0] = buf[i+2]
		img.Pix[i+1] = buf[i+1]
		img.Pix[i+2] = buf[i+0]
		img.Pix[i+3] = 0xff
	}
	return img, nil
}

// Encode checks the image is thumbnail-sized and encodes it as JPEG. It
// refuses anything larger than MaxSide so a full-size frame can never be
// published by mistake.
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

// Runner captures and publishes a thumbnail on every tick while enabled.
type Runner struct {
	// Enabled reports whether the user has switched the feature on. It is
	// read on every tick so settings changes apply without a restart.
	Enabled func() bool
	// Ready reports whether there is somewhere to send the thumbnail (for
	// example MQTT is connected). Nothing is captured when it is false.
	Ready func() bool
	// Locked reports whether the session is locked. Nothing is captured
	// while it is locked. Optional.
	Locked func() bool
	// Capture returns a screen image already scaled to at most maxSide.
	Capture func(maxSide int) (image.Image, error)
	// Publish sends an encoded thumbnail.
	Publish func(jpeg []byte) error
}

// Result says what one tick did, for logging and tests.
type Result int

const (
	Skipped Result = iota
	Published
	Failed
)

// Tick runs one capture. The encoded thumbnail is not kept after Publish
// returns.
func (r *Runner) Tick() (Result, error) {
	if r.Enabled == nil || !r.Enabled() {
		return Skipped, nil
	}
	if r.Ready != nil && !r.Ready() {
		return Skipped, nil
	}
	if r.Locked != nil && r.Locked() {
		return Skipped, nil
	}
	img, err := r.Capture(MaxSide)
	if err != nil {
		return Failed, err
	}
	data, err := Encode(img)
	if err != nil {
		return Failed, err
	}
	if err := r.Publish(data); err != nil {
		return Failed, err
	}
	return Published, nil
}

// Run ticks every interval until ctx is cancelled. report, if not nil, is
// called after every tick with its error (nil when the tick succeeded or was
// skipped).
func (r *Runner) Run(ctx context.Context, interval time.Duration, report func(error)) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			_, err := r.Tick()
			if report != nil {
				report(err)
			}
		}
	}
}

// LogOnChange wraps log so a failure that repeats every tick is logged once.
// A nil error (a tick that did not fail) resets it, so the same failure is
// logged again if it comes back later.
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
