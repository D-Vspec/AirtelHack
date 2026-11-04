// Package kyc is the deterministic document pipeline: image quality, OCR, field parsing and the
// cross-document and registry checks that produce typed findings and a route.
package kyc

import (
	"image"
	"image/color"
	"image/draw"
	_ "image/jpeg" // phone photos
	"math"
)

// Gray converts any image to 8-bit grayscale.
func Gray(src image.Image) *image.Gray {
	if g, ok := src.(*image.Gray); ok {
		return g
	}
	g := image.NewGray(src.Bounds())
	draw.Draw(g, g.Bounds(), src, src.Bounds().Min, draw.Src)
	return g
}

// Sharpness is the variance of the 4-neighbour Laplacian: high for crisp text edges, low when blurred.
func Sharpness(g *image.Gray) float64 {
	b := g.Bounds()
	w, h := b.Dx(), b.Dy()
	var sum, sq float64
	n := 0
	for y := 1; y < h-1; y++ {
		row := g.Pix[y*g.Stride:]
		up, down := g.Pix[(y-1)*g.Stride:], g.Pix[(y+1)*g.Stride:]
		for x := 1; x < w-1; x++ {
			l := float64(up[x]) + float64(down[x]) + float64(row[x-1]) + float64(row[x+1]) - 4*float64(row[x])
			sum += l
			sq += l * l
			n++
		}
	}
	if n == 0 {
		return 0
	}
	mean := sum / float64(n)
	return sq/float64(n) - mean*mean
}

// SkewDegrees estimates text-line skew with a projection profile: the rotation whose row sums of
// dark pixels are most peaked (text lines aligned). Positive = content rotated counter-clockwise.
// Searches ±maxDeg in 0.25° steps on dark pixels sampled every 2nd pixel.
func SkewDegrees(g *image.Gray, maxDeg float64) float64 {
	b := g.Bounds()
	type pt struct{ x, y float64 }
	var dark []pt
	cx, cy := float64(b.Dx())/2, float64(b.Dy())/2
	for y := 0; y < b.Dy(); y += 2 {
		for x := 0; x < b.Dx(); x += 2 {
			if g.Pix[y*g.Stride+x] < 128 {
				dark = append(dark, pt{float64(x) - cx, float64(y) - cy})
			}
		}
	}
	if len(dark) == 0 {
		return 0
	}
	diag := int(math.Hypot(cx, cy)) + 2
	bins := make([]float64, 2*diag)
	best, bestScore := 0.0, -1.0
	for a := -maxDeg; a <= maxDeg+1e-9; a += 0.25 {
		s, c := math.Sincos(a * math.Pi / 180)
		for i := range bins {
			bins[i] = 0
		}
		for _, p := range dark {
			// Row of the point after rotating the image by -a (undoing a skew of a).
			bins[int(p.y*c-p.x*s)+diag]++
		}
		score := 0.0
		for _, v := range bins {
			score += v * v
		}
		if score > bestScore {
			best, bestScore = a, score
		}
	}
	return best
}

// Rotate turns the image by deg degrees around its centre (same convention as SkewDegrees, so
// Rotate(g, SkewDegrees(g)) straightens it), filling uncovered corners with white. Bilinear.
func Rotate(g *image.Gray, deg float64) *image.Gray {
	b := g.Bounds()
	w, h := b.Dx(), b.Dy()
	out := image.NewGray(image.Rect(0, 0, w, h))
	s, c := math.Sincos(deg * math.Pi / 180)
	cx, cy := float64(w)/2, float64(h)/2
	at := func(x, y int) float64 {
		if x < 0 || y < 0 || x >= w || y >= h {
			return 255
		}
		return float64(g.Pix[y*g.Stride+x])
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			// Inverse map: output pixel -> source pixel.
			dx, dy := float64(x)-cx, float64(y)-cy
			sx, sy := dx*c-dy*s+cx, dx*s+dy*c+cy
			x0, y0 := int(math.Floor(sx)), int(math.Floor(sy))
			fx, fy := sx-float64(x0), sy-float64(y0)
			v := at(x0, y0)*(1-fx)*(1-fy) + at(x0+1, y0)*fx*(1-fy) + at(x0, y0+1)*(1-fx)*fy + at(x0+1, y0+1)*fx*fy
			out.SetGray(x, y, color.Gray{Y: uint8(v + 0.5)})
		}
	}
	return out
}
