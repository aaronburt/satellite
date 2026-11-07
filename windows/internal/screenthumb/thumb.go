package screenthumb

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"sync/atomic"
)

const (
	MaxSide     = 200
	ContentType = "image/jpeg"

	jpegQuality = 75
)

var ErrUnsupported = errors.New("screen thumbnails are not supported on this platform")

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
	for i := 0; i < len(buf); i += 4 {
		img.Pix[i+0] = buf[i+2]
		img.Pix[i+1] = buf[i+1]
		img.Pix[i+2] = buf[i+0]
		img.Pix[i+3] = 0xff
	}
	return img, nil
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
	Enabled func() bool
	Ready   func() bool
	Locked  func() bool
	Capture func(maxSide int) (image.Image, error)
	Publish func(jpeg []byte) error

	running atomic.Bool
}

type Result int

const (
	Skipped Result = iota
	Published
	Failed
)

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
