package native

import (
	"bytes"
	"image/png"
	"testing"
)

// TestTrayIconProperties verifies that the template PNG generated for the macOS
// menu bar meets Apple's HIG specifications: valid 32x32 dimensions (for @2x Retina),
// proper vertical centering, and solid black template pixels with transparent alpha.
func TestTrayIconProperties(t *testing.T) {
	data := trayIcon()
	if len(data) == 0 {
		t.Fatal("trayIcon() returned empty byte slice")
	}

	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("trayIcon() failed to encode valid PNG: %v", err)
	}

	b := img.Bounds()
	if b.Dx() != 32 || b.Dy() != 32 {
		t.Fatalf("expected 32x32 icon, got %dx%d", b.Dx(), b.Dy())
	}

	// Count black pixels and measure bounding box.
	blackCount := 0
	minY, maxY := 32, -1
	minX, maxX := 32, -1

	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			if a > 0 {
				blackCount++
				if y < minY {
					minY = y
				}
				if y > maxY {
					maxY = y
				}
				if x < minX {
					minX = x
				}
				if x > maxX {
					maxX = x
				}
				// Template images in macOS should be pure black (RGB=0) with alpha.
				if r != 0 || g != 0 || b != 0 {
					t.Errorf("pixel at (%d, %d) has non-zero RGB in template image: (%d,%d,%d)", x, y, r, g, b)
				}
			}
		}
	}

	if blackCount < 100 {
		t.Errorf("tray icon is too thin/sparse: only %d opaque pixels", blackCount)
	}

	// The icon must be vertically centered in the 32px canvas:
	// Optical center around y=16 (between 7 and 25).
	if minY < 4 || maxY > 27 {
		t.Errorf("tray icon exceeds safe optical margin [4..27]: minY=%d, maxY=%d", minY, maxY)
	}
	if minX < 2 || maxX > 29 {
		t.Errorf("tray icon exceeds safe optical margin [2..29]: minX=%d, maxX=%d", minX, maxX)
	}

	center := (minY + maxY) / 2
	if center < 15 || center > 17 {
		t.Errorf("tray icon vertical center %d is not centered around 16 (minY=%d, maxY=%d)", center, minY, maxY)
	}
}
