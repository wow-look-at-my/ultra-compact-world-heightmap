// Package terrarium reads and compares Terrarium-encoded elevation tiles.
package terrarium

import (
	"fmt"
	"image"
	"io"

	"golang.org/x/image/webp"
)

// Tile is one elevation tile. Each sample is the 24-bit number R<<16 | G<<8 | B.
type Tile struct {
	Width, Height int
	Samples       []int32
}

// DecodeWebP reads a lossless WebP tile.
func DecodeWebP(r io.Reader) (*Tile, error) {
	img, err := webp.Decode(r)
	if err != nil {
		return nil, fmt.Errorf("decode webp: %w", err)
	}
	px, ok := img.(*image.NRGBA)
	if !ok {
		return nil, fmt.Errorf("the tile decodes to %T, and only a lossless RGB tile holds exact elevations", img)
	}
	b := px.Bounds()
	t := &Tile{Width: b.Dx(), Height: b.Dy(), Samples: make([]int32, b.Dx()*b.Dy())}
	for y := 0; y < t.Height; y++ {
		row := px.Pix[y*px.Stride : y*px.Stride+4*t.Width]
		for x := 0; x < t.Width; x++ {
			p := row[4*x : 4*x+4]
			if p[3] != 0xff {
				return nil, fmt.Errorf("the pixel at %d,%d is transparent, which the format has no sample for", x, y)
			}
			t.Samples[y*t.Width+x] = int32(p[0])<<16 | int32(p[1])<<8 | int32(p[2])
		}
	}
	return t, nil
}

// Metres converts a sample to an elevation.
func Metres(sample int32) float64 {
	return float64(sample)/256 - 32768
}
