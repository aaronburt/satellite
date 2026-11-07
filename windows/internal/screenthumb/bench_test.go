package screenthumb

import (
	"image"
	"testing"
)

func BenchmarkThumbSize(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _, _ = ThumbSize(1920, 1080, MaxSide)
	}
}

func BenchmarkFromBGRA(b *testing.B) {
	w, h := 200, 113
	buf := make([]byte, w*h*4)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := FromBGRA(buf, w, h); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEncode(b *testing.B) {
	img := image.NewRGBA(image.Rect(0, 0, 200, 113))
	for i := range img.Pix {
		img.Pix[i] = byte(i * 7)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Encode(img); err != nil {
			b.Fatal(err)
		}
	}
}
