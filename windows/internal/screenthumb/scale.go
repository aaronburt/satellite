package screenthumb

import "image"

func PickMipLevel(w, h, maxSide int) int {
	level := 0
	for max(w>>(level+1), h>>(level+1)) >= maxSide && w>>(level+1) >= 1 && h>>(level+1) >= 1 {
		level++
	}
	return level
}

func MipSize(w, h, level int) (int, int) {
	return max(1, w>>level), max(1, h>>level)
}

func BoxResize(src *image.RGBA, w, h int) *image.RGBA {
	sb := src.Bounds()
	sw, sh := sb.Dx(), sb.Dy()
	if sw == w && sh == h {
		return src
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		y0 := y * sh / h
		y1 := max(y0+1, (y+1)*sh/h)
		for x := 0; x < w; x++ {
			x0 := x * sw / w
			x1 := max(x0+1, (x+1)*sw/w)
			var r, g, b, n uint32
			for sy := y0; sy < y1 && sy < sh; sy++ {
				row := src.Pix[sy*src.Stride:]
				for sx := x0; sx < x1 && sx < sw; sx++ {
					p := row[sx*4:]
					r += uint32(p[0])
					g += uint32(p[1])
					b += uint32(p[2])
					n++
				}
			}
			o := dst.Pix[y*dst.Stride+x*4:]
			if n == 0 {
				o[3] = 0xff
				continue
			}
			o[0], o[1], o[2], o[3] = uint8(r/n), uint8(g/n), uint8(b/n), 0xff
		}
	}
	return dst
}
