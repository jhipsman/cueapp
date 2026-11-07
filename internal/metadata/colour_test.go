package metadata

import (
	"fmt"
	"image"
	"image/color"
	"testing"
)

func TestMainColourPicksTheVividPart(t *testing.T) {
	// Mostly near-black, with a band of strong orange: the orange wins.
	img := image.NewRGBA(image.Rect(0, 0, 96, 54))
	for y := 0; y < 54; y++ {
		for x := 0; x < 96; x++ {
			c := color.RGBA{12, 12, 14, 255}
			if y > 30 && y < 40 {
				c = color.RGBA{230, 110, 30, 255}
			}
			img.Set(x, y, c)
		}
	}
	var r, g, b int
	if _, err := fmt.Sscanf(mainColourOf(img), "%d, %d, %d", &r, &g, &b); err != nil {
		t.Fatal(err)
	}
	// Within a few levels of the orange (the dark pixels count a little).
	if abs(r-230) > 4 || abs(g-110) > 4 || abs(b-30) > 4 {
		t.Fatalf("colour = %d, %d, %d, want close to 230, 110, 30", r, g, b)
	}
}

func TestMainColourLiftsDarkPictures(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			img.Set(x, y, color.RGBA{20, 40, 80, 255})
		}
	}
	if got := mainColourOf(img); got != "43, 85, 170" {
		t.Fatalf("colour = %q, want it lifted to glow", got)
	}
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
