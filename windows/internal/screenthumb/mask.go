package screenthumb

import (
	"errors"
	"image"
	"image/color"
	"path"
	"strings"
)

// MaxHiddenApps bounds the hide list so a bad config cannot make every tick
// slow.
const MaxHiddenApps = 50

// Rect is a rectangle in screen pixels.
type Rect struct{ X, Y, W, H int }

// NormalizeApps cleans a hide list: it trims names, drops any folder part,
// lower-cases, removes empties and duplicates, and keeps at most
// MaxHiddenApps entries. It returns nil for an empty list.
func NormalizeApps(apps []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, a := range apps {
		a = strings.ToLower(strings.TrimSpace(a))
		a = strings.ReplaceAll(a, `\`, "/")
		a = path.Base(a)
		if a == "" || a == "." || a == "/" || seen[a] {
			continue
		}
		seen[a] = true
		out = append(out, a)
		if len(out) == MaxHiddenApps {
			break
		}
	}
	return out
}

// MatchApp reports whether a process image name (a bare name or a full path)
// is on the hide list. The ".exe" suffix is optional on either side.
func MatchApp(process string, apps []string) bool {
	p := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(process)), `\`, "/")
	p = strings.TrimSuffix(path.Base(p), ".exe")
	if p == "" || p == "." || p == "/" {
		return false
	}
	for _, a := range apps {
		if strings.TrimSuffix(a, ".exe") == p {
			return true
		}
	}
	return false
}

// MaskImage paints the given screen-pixel rectangles black on a thumbnail of a
// screenW x screenH screen. Rectangles are rounded outward so a hidden window
// is never partly shown, and clipped to the image. It returns an error for an
// image it cannot paint on, so callers can refuse to publish it.
func MaskImage(img image.Image, screenW, screenH int, rects []Rect) error {
	if len(rects) == 0 {
		return nil
	}
	if screenW <= 0 || screenH <= 0 {
		return errors.New("invalid screen size for masking")
	}
	dst, ok := img.(*image.RGBA)
	if !ok {
		return errors.New("cannot mask this image type")
	}
	b := dst.Bounds()
	tw, th := b.Dx(), b.Dy()
	black := image.NewUniform(color.RGBA{A: 0xff})
	for _, r := range rects {
		if r.W <= 0 || r.H <= 0 {
			continue
		}
		x0 := floorDiv(r.X*tw, screenW)
		y0 := floorDiv(r.Y*th, screenH)
		x1 := ceilDiv((r.X+r.W)*tw, screenW)
		y1 := ceilDiv((r.Y+r.H)*th, screenH)
		sub := image.Rect(x0, y0, x1, y1).Add(b.Min).Intersect(b)
		if sub.Empty() {
			continue
		}
		fillRect(dst, sub, black)
	}
	return nil
}

func fillRect(dst *image.RGBA, r image.Rectangle, src *image.Uniform) {
	c := src.C.(color.RGBA)
	for y := r.Min.Y; y < r.Max.Y; y++ {
		row := dst.Pix[dst.PixOffset(r.Min.X, y):dst.PixOffset(r.Max.X, y)]
		for i := 0; i < len(row); i += 4 {
			row[i], row[i+1], row[i+2], row[i+3] = c.R, c.G, c.B, c.A
		}
	}
}

func floorDiv(a, b int) int {
	q := a / b
	if a%b != 0 && (a < 0) != (b < 0) {
		q--
	}
	return q
}

func ceilDiv(a, b int) int { return -floorDiv(-a, b) }

// HiddenRects finds the on-screen windows of the listed apps. ScreenW and
// ScreenH are the size of the captured screen those rectangles are in.
type HiddenRects func(apps []string) (rects []Rect, screenW, screenH int, err error)

// NewHider returns a function that blacks out the windows of the apps
// currently on the list. With an empty list it does nothing.
func NewHider(apps func() []string, find HiddenRects) func(image.Image) error {
	return func(img image.Image) error {
		var list []string
		if apps != nil {
			list = NormalizeApps(apps())
		}
		if len(list) == 0 {
			return nil
		}
		rects, sw, sh, err := find(list)
		if err != nil {
			return err
		}
		return MaskImage(img, sw, sh, rects)
	}
}
