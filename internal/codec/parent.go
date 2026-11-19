// parent.go ties a tile to the tile one zoom above it.

package codec

import (
	"fmt"

	"github.com/wow-look-at-my/ultra-compact-world-heightmap/internal/terrarium"
)

// Parent names the tile one zoom above a tile, and the quarter of it that the tile covers.
type Parent struct {
	Tile *terrarium.Tile
	// Quadrant counts the quarters row by row: top left, top right, bottom left, bottom right.
	Quadrant int
}

// buildParent lays the parent over a tile of w by h samples. shift is the quantum of the
// tile and parentShift the quantum that the parent was rounded to.
func buildParent(p *Parent, w, h int, shift, parentShift uint) (*parentGrid, error) {
	pt := p.Tile
	if pt == nil || pt.Width != w || pt.Height != h || w%2 != 0 || h%2 != 0 || len(pt.Samples) != w*h || p.Quadrant < 0 || p.Quadrant > 3 {
		return nil, fmt.Errorf("the parent does not fit a tile of %dx%d", w, h)
	}
	if parentShift < shift || parentShift > 24 || 4<<(parentShift-shift) > maxBlockStep {
		return nil, fmt.Errorf("a parent rounded to %d bits does not bound a tile rounded to %d bits", parentShift, shift)
	}
	bw, bh := w/2, h/2
	ox, oy := (p.Quadrant%2)*bw, (p.Quadrant/2)*bh
	g := &parentGrid{stride: bw + 2*parentMargin, blocks: bw, lo: make([]int32, bw*bh), hi: make([]int32, bw*bh)}
	g.mean = make([]int32, g.stride*(bh+2*parentMargin))
	scale := parentShift - shift
	step := int32(4) << scale
	for j := -parentMargin; j < bh+parentMargin; j++ {
		py := min(max(oy+j, 0), h-1)
		for i := -parentMargin; i < bw+parentMargin; i++ {
			px := min(max(ox+i, 0), w-1)
			v := pt.Samples[py*w+px]
			if v < 0 || v >= 1<<24 || v&(1<<parentShift-1) != 0 {
				return nil, fmt.Errorf("the parent sample %d is not a multiple of its quantum", v)
			}
			q := v >> parentShift
			g.mean[(j+parentMargin)*g.stride+i+parentMargin] = q << scale
			if i >= 0 && i < bw && j >= 0 && j < bh {
				// A tie rounds to the even value, so an odd parent sample excludes both ends.
				odd := q & 1
				g.lo[j*bw+i] = q*step - step/2 + odd
				g.hi[j*bw+i] = q*step + step/2 - odd
			}
		}
	}
	return g, nil
}

// fitParent finds the quantum at which the parent is the rounded mean of the tile. It
// gives a nil grid when the tile does not follow its parent at any quantum.
func fitParent(t *terrarium.Tile, p *Parent, shift uint) (*parentGrid, uint, uint) {
	if p.Tile == nil {
		return nil, shift, 0
	}
	for parentShift := int(min(quantumShift(p.Tile.Samples), 12)); parentShift >= 0; parentShift-- {
		s := min(shift, uint(parentShift))
		g, err := buildParent(p, t.Width, t.Height, s, uint(parentShift))
		if err != nil {
			continue
		}
		if follows(t, g, s) {
			return g, s, uint(parentShift)
		}
	}
	return nil, shift, 0
}

func follows(t *terrarium.Tile, g *parentGrid, shift uint) bool {
	w := t.Width
	for j := 0; j < t.Height/2; j++ {
		top, bottom := t.Samples[2*j*w:], t.Samples[(2*j+1)*w:]
		for i := 0; i < w/2; i++ {
			sum := top[2*i]>>shift + top[2*i+1]>>shift + bottom[2*i]>>shift + bottom[2*i+1]>>shift
			if sum < g.lo[j*g.blocks+i] || sum > g.hi[j*g.blocks+i] {
				return false
			}
		}
	}
	return true
}
