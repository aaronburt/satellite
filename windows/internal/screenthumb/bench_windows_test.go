//go:build windows

package screenthumb

import "testing"

func BenchmarkCapture(b *testing.B) {
	if _, err := Capture(MaxSide); err != nil {
		b.Skipf("no capturable desktop here: %v", err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Capture(MaxSide); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCaptureAndEncode(b *testing.B) {
	if _, err := Capture(MaxSide); err != nil {
		b.Skipf("no capturable desktop here: %v", err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		img, err := Capture(MaxSide)
		if err != nil {
			b.Fatal(err)
		}
		if _, err := Encode(img); err != nil {
			b.Fatal(err)
		}
	}
}

func TestCaptureReportsScreenSize(t *testing.T) {
	sw, _, _ := procGetSystemMetrics.Call(smCxScreen)
	sh, _, _ := procGetSystemMetrics.Call(smCyScreen)
	t.Logf("primary screen %dx%d", sw, sh)
}
