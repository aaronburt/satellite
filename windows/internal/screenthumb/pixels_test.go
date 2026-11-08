package screenthumb

import (
	"bytes"
	"testing"
)

func TestConvertBGRAInPlace(t *testing.T) {
	pixels := []byte{1, 2, 3, 0, 4, 5, 6, 7}
	convertBGRA(pixels, pixels)
	if !bytes.Equal(pixels, []byte{3, 2, 1, 255, 6, 5, 4, 255}) {
		t.Fatalf("converted pixels = %v", pixels)
	}
}

func TestFromBGRAPreservesInputOwnership(t *testing.T) {
	pixels := []byte{1, 2, 3, 0}
	img, err := FromBGRA(pixels, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(pixels, []byte{1, 2, 3, 0}) {
		t.Fatal("conversion changed the source")
	}
	pixels[0] = 99
	if !bytes.Equal(img.Pix, []byte{3, 2, 1, 255}) {
		t.Fatal("returned image aliases the source")
	}
}
