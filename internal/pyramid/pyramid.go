// Package pyramid holds the rule that ties a tile to the tiles one zoom below it.
package pyramid

import (
	"fmt"

	"github.com/wow-look-at-my/ultra-compact-world-heightmap/internal/terrarium"
)

const (
	seaLevel = 32768 * 256
	// fullZoom is the zoom at which a tile keeps every bit of a Terrarium sample.
	fullZoom = 19
)

// Quantum gives the step, in sample units, that the pipeline rounds a tile of this zoom to.
func Quantum(zoom int) int32 {
	if zoom >= fullZoom {
		return 1
	}
	return int32(min(256, 1<<min(fullZoom-zoom, 8)))
}

// RoundHalfEven divides sum by step and rounds a tie to the even result.
func RoundHalfEven(sum, step int64) int64 {
	q := sum / step
	r := sum % step
	if r < 0 {
		q--
		r += step
	}
	if 2*r > step || (2*r == step && q&1 != 0) {
		q++
	}
	return q
}

// Parent computes a tile from its children, in the order top left, top right,
// bottom left, bottom right. A nil child stands at sea level, which is what the
// pipeline puts where it has no tile. zoom is the zoom of the parent.
func Parent(children [4]*terrarium.Tile, zoom int) (*terrarium.Tile, error) {
	var w, h int
	for _, c := range children {
		if c == nil {
			continue
		}
		if c.Width%2 != 0 || c.Height%2 != 0 || (w != 0 && (c.Width != w || c.Height != h)) {
			return nil, fmt.Errorf("the children are %dx%d and %dx%d, and a parent needs four tiles of one even size", w, h, c.Width, c.Height)
		}
		w, h = c.Width, c.Height
	}
	if w == 0 {
		return nil, fmt.Errorf("a parent needs at least one child")
	}
	step := int64(Quantum(zoom))
	p := &terrarium.Tile{Width: w, Height: h, Samples: make([]int32, w*h)}
	for i := range p.Samples {
		p.Samples[i] = seaLevel
	}
	for k, c := range children {
		if c == nil {
			continue
		}
		ox, oy := (k%2)*w/2, (k/2)*h/2
		for y := 0; y < h/2; y++ {
			top := c.Samples[2*y*w : (2*y+1)*w]
			bottom := c.Samples[(2*y+1)*w : (2*y+2)*w]
			for x := 0; x < w/2; x++ {
				sum := int64(top[2*x]) + int64(top[2*x+1]) + int64(bottom[2*x]) + int64(bottom[2*x+1]) - 4*seaLevel
				p.Samples[(oy+y)*w+ox+x] = int32(RoundHalfEven(sum, 4*step)*step) + seaLevel
			}
		}
	}
	return p, nil
}
