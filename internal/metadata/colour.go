package metadata

import (
	"context"
	"fmt"
	"image"
	_ "image/jpeg" // TMDB's pictures
	_ "image/png"
	"io"
	"net/http"
	"strings"
)

// smallBackdrop is TMDB's smallest backdrop size: enough to read a colour.
func smallBackdrop(backdropPath string) string {
	if backdropPath == "" {
		return ""
	}
	return "https://image.tmdb.org/t/p/w300" + backdropPath
}

// MainColour is the colour a picture is "about", as "r, g, b": its pixels
// averaged, each weighted by how saturated and bright it is, so a purple
// night sky or an orange desert wins over the dark parts around it. It is
// lifted to a brightness that can glow on Cue's dark screens. "" when the
// title has no picture. Kept a while, per picture.
func (c *Client) MainColour(ctx context.Context, backdropPath string) (string, error) {
	u := smallBackdrop(backdropPath)
	if u == "" {
		return "", nil
	}
	key := "colour:" + backdropPath
	if v, ok := c.logos.get(key); ok {
		return v.(string), nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch picture: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetch picture: %s", resp.Status)
	}
	img, _, err := image.Decode(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return "", fmt.Errorf("read picture: %w", err)
	}
	col := mainColourOf(img)
	c.logos.put(key, col)
	return col, nil
}

func mainColourOf(img image.Image) string {
	b := img.Bounds()
	step := max(1, b.Dx()/48)
	var R, G, B, W float64
	for y := b.Min.Y; y < b.Max.Y; y += step {
		for x := b.Min.X; x < b.Max.X; x += step {
			r16, g16, b16, _ := img.At(x, y).RGBA()
			r, g, bl := float64(r16>>8), float64(g16>>8), float64(b16>>8)
			mx, mn := max(r, g, bl), min(r, g, bl)
			w := ((mx-mn)/255)*((mx-mn)/255)*(mx/255) + 0.001
			R, G, B, W = R+r*w, G+g*w, B+bl*w, W+w
		}
	}
	if W == 0 {
		return ""
	}
	rgb := []float64{R / W, G / W, B / W}
	if mx := max(rgb[0], rgb[1], rgb[2]); mx > 0 && mx < 170 {
		for i := range rgb {
			rgb[i] = rgb[i] * 170 / mx
		}
	}
	parts := make([]string, 3)
	for i, v := range rgb {
		parts[i] = fmt.Sprint(int(min(255, v+0.5)))
	}
	return strings.Join(parts, ", ")
}
