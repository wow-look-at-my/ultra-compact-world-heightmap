// tiles.go holds what the commands share to find a tile, its parent and its children.

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/wow-look-at-my/ultra-compact-world-heightmap/internal/pyramid"
	"github.com/wow-look-at-my/ultra-compact-world-heightmap/internal/terrarium"
)

func loadTile(path string) (*terrarium.Tile, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, 0, err
	}
	t, err := terrarium.Decode(f)
	return t, st.Size(), err
}

// tileAddress reads z, x and y from a name like z-x-y.webp.
func tileAddress(path string) (z, x, y int, err error) {
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	if _, err := fmt.Sscanf(name, "%d-%d-%d", &z, &x, &y); err != nil {
		return 0, 0, 0, fmt.Errorf("%s is not named z-x-y", path)
	}
	return z, x, y, nil
}

// derives reports whether a tile is exactly what the pipeline builds from its
// children. It is false when a child is not among the tiles, because a tile that is
// absent here may still exist on the server.
func derives(tile *terrarium.Tile, z, x, y int, byAddress map[[3]int]string) (bool, error) {
	var children [4]*terrarium.Tile
	for k := range children {
		path, ok := byAddress[[3]int{z + 1, 2*x + k%2, 2*y + k/2}]
		if !ok {
			return false, nil
		}
		child, _, err := loadTile(path)
		if err != nil {
			return false, fmt.Errorf("%s: %w", path, err)
		}
		children[k] = child
	}
	built, err := pyramid.Parent(children, z)
	if err != nil {
		return false, err
	}
	return built.Width == tile.Width && built.Height == tile.Height && slices.Equal(built.Samples, tile.Samples), nil
}
