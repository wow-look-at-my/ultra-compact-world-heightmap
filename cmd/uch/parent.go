// parent.go holds the flags that name the parent of a tile, for the commands that take one.

package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/wow-look-at-my/ultra-compact-world-heightmap/internal/codec"
)

type parentFlags struct {
	path     string
	quadrant int
}

func (p *parentFlags) register(cmd *cobra.Command) {
	cmd.Flags().StringVar(&p.path, "parent", "", "the tile one zoom above, as a WebP or a PNG")
	cmd.Flags().IntVar(&p.quadrant, "quadrant", -1, "the quarter of the parent that the tile covers, counted row by row from 0 to 3 (read from names like z-x-y when both have one)")
}

// load reads the parent, and gives nil when no parent was named. tilePath is the tile
// whose name gives the quadrant when the flag does not.
func (p *parentFlags) load(tilePath string) (*codec.Parent, error) {
	if p.path == "" {
		return nil, nil
	}
	quadrant := p.quadrant
	if quadrant < 0 {
		z, x, y, err := tileAddress(tilePath)
		pz, px, py, perr := tileAddress(p.path)
		if err != nil || perr != nil || pz != z-1 || px != x/2 || py != y/2 {
			return nil, fmt.Errorf("the names do not say which quarter of %s the tile covers: pass --quadrant", p.path)
		}
		quadrant = y%2*2 + x%2
	}
	if quadrant > 3 {
		return nil, fmt.Errorf("quadrant %d is not one of 0 to 3", quadrant)
	}
	tile, _, err := loadTile(p.path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", p.path, err)
	}
	return &codec.Parent{Tile: tile, Quadrant: quadrant}, nil
}
