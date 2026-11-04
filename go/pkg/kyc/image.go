// Package kyc is the deterministic KYC pipeline: image quality, OCR, field parsing and the
// checks that turn documents plus registry records into findings and a route.
package kyc

import (
	"bytes"
	"image"
	"image/color"
	_ "image/jpeg" // register decoder
	"image/png"
	"math"
)

// Gray is an 8-bit grayscale image (0 black, 255 white).
type Gray struct {
	W, H int
	Pix  []uint8
}

func (g *Gray) at(x, y int) uint8 { return g.Pix[y*g.W+x] }

// RedChannel converts an image to gray using only the red channel. The specimen watermark is
// pink (high red) and the text is dark, so the watermark almost disappears in this channel.
func RedChannel(img image.Image) *Gray {
	b := img.Bounds()
	g := &Gray{W: b.Dx(), H: b.Dy(), Pix: make([]uint8, b.Dx()*b.Dy())}
	for y := 0; y < g.H; y++ {
		for x := 0; x < g.W; x++ {
			r, _, _, a := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
			if a == 0 {
				g.Pix[y*g.W+x] = 255
				continue
			}
			g.Pix[y*g.W+x] = uint8(r >> 8)
		}
	}
	return g
}

// Luma converts to standard luminance gray (used for the blur measure).
func Luma(img image.Image) *Gray {
	b := img.Bounds()
	g := &Gray{W: b.Dx(), H: b.Dy(), Pix: make([]uint8, b.Dx()*b.Dy())}
	for y := 0; y < g.H; y++ {
		for x := 0; x < g.W; x++ {
			g.Pix[y*g.W+x] = color.GrayModel.Convert(img.At(b.Min.X+x, b.Min.Y+y)).(color.Gray).Y
		}
	}
	return g
}

// InkThreshold separates text from background and watermark in the red channel.
const InkThreshold = 150

// Sharpness is a blur score: the variance of the 4-neighbour Laplacian, taken over pixels near
// ink so that large blank areas of a page do not dilute it. Sharp text gives large values;
// a blurred page gives small ones.
func Sharpness(g *Gray) float64 {
	var sum, sumSq float64
	n := 0
	for y := 1; y < g.H-1; y++ {
		for x := 1; x < g.W-1; x++ {
			c := int(g.at(x, y))
			l := int(g.at(x-1, y)) + int(g.at(x+1, y)) + int(g.at(x, y-1)) + int(g.at(x, y+1)) - 4*c
			if l == 0 && c > 200 {
				continue // flat background
			}
			v := float64(l)
			sum += v
			sumSq += v * v
			n++
		}
	}
	if n == 0 {
		return 0
	}
	mean := sum / float64(n)
	return sumSq/float64(n) - mean*mean
}

// DetectSkew returns the angle in degrees that straightens the page (pass it to Rotate). It
// maximises the variance of the horizontal projection profile of ink pixels: text lines give
// sharp peaks only when they are level.
func DetectSkew(g *Gray, maxDeg, stepDeg float64) float64 {
	best, bestScore := 0.0, profileScore(g, 0)
	for a := -maxDeg; a <= maxDeg+1e-9; a += stepDeg {
		if s := profileScore(g, a); s > bestScore*1.0001 {
			best, bestScore = a, s
		}
	}
	return best
}

// Rotate rotates the image by -deg degrees about its centre (so DetectSkew's result straightens
// it), filling uncovered pixels with white. Nearest-neighbour sampling is enough for OCR.
func Rotate(g *Gray, deg float64) *Gray {
	if deg == 0 {
		return g
	}
	out := &Gray{W: g.W, H: g.H, Pix: make([]uint8, len(g.Pix))}
	s, c := math.Sincos(deg * math.Pi / 180)
	cx, cy := float64(g.W)/2, float64(g.H)/2
	for y := 0; y < g.H; y++ {
		for x := 0; x < g.W; x++ {
			dx, dy := float64(x)-cx, float64(y)-cy
			// inverse mapping: destination point rotated by +deg gives the source point
			sx := int(math.Round(dx*c - dy*s + cx))
			sy := int(math.Round(dx*s + dy*c + cy))
			v := uint8(255)
			if sx >= 0 && sy >= 0 && sx < g.W && sy < g.H {
				v = g.at(sx, sy)
			}
			out.Pix[y*g.W+x] = v
		}
	}
	return out
}

// Upscale2x doubles the image size (nearest neighbour). Tesseract reads the ~20px monospace
// identifier font noticeably better at double size.
func Upscale2x(g *Gray) *Gray {
	out := &Gray{W: g.W * 2, H: g.H * 2, Pix: make([]uint8, 4*len(g.Pix))}
	for y := 0; y < out.H; y++ {
		for x := 0; x < out.W; x++ {
			out.Pix[y*out.W+x] = g.at(x/2, y/2)
		}
	}
	return out
}

// Crop returns the part of the image inside r (clipped to the image).
func (g *Gray) Crop(r image.Rectangle) *Gray {
	r = r.Intersect(image.Rect(0, 0, g.W, g.H))
	out := &Gray{W: r.Dx(), H: r.Dy(), Pix: make([]uint8, r.Dx()*r.Dy())}
	for y := 0; y < out.H; y++ {
		copy(out.Pix[y*out.W:(y+1)*out.W], g.Pix[(r.Min.Y+y)*g.W+r.Min.X:(r.Min.Y+y)*g.W+r.Max.X])
	}
	return out
}

// Thicken grows ink by one pixel (a 3x3 cross minimum filter). Thin monospace strokes are
// what make tesseract read "1Z" as "17Z"; slightly bolder glyphs read correctly.
func Thicken(g *Gray) *Gray {
	out := &Gray{W: g.W, H: g.H, Pix: make([]uint8, len(g.Pix))}
	for y := 0; y < g.H; y++ {
		for x := 0; x < g.W; x++ {
			v := g.at(x, y)
			for _, d := range [4][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}} {
				nx, ny := x+d[0], y+d[1]
				if nx >= 0 && ny >= 0 && nx < g.W && ny < g.H {
					v = min(v, g.at(nx, ny))
				}
			}
			out.Pix[y*g.W+x] = v
		}
	}
	return out
}

// Pad adds a white border of n pixels.
func Pad(g *Gray, n int) *Gray {
	out := &Gray{W: g.W + 2*n, H: g.H + 2*n}
	out.Pix = make([]uint8, out.W*out.H)
	for i := range out.Pix {
		out.Pix[i] = 255
	}
	for y := 0; y < g.H; y++ {
		copy(out.Pix[(y+n)*out.W+n:], g.Pix[y*g.W:(y+1)*g.W])
	}
	return out
}

// Binarize maps ink to black and everything else to white.
func Binarize(g *Gray) *Gray {
	out := &Gray{W: g.W, H: g.H, Pix: make([]uint8, len(g.Pix))}
	for i, v := range g.Pix {
		if v < InkThreshold {
			out.Pix[i] = 0
		} else {
			out.Pix[i] = 255
		}
	}
	return out
}

// PNG encodes the gray image.
func (g *Gray) PNG() ([]byte, error) {
	img := &image.Gray{Pix: g.Pix, Stride: g.W, Rect: image.Rect(0, 0, g.W, g.H)}
	var buf bytes.Buffer
	err := png.Encode(&buf, img)
	return buf.Bytes(), err
}
