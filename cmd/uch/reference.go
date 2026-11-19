// reference.go reads what a tile is coded against in reference mode.

package main

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"

	"github.com/wow-look-at-my/ultra-compact-world-heightmap/internal/codec"
)

const referenceHelp = "a tile of the same size as a WebP or a PNG, or heights before rounding as a .f32 file of float32 metres"

// parseHeights reads little-endian float32 metres in row order.
func parseHeights(raw []byte, width, height int) ([]float32, error) {
	if width < 1 || height < 1 || len(raw) != 4*width*height {
		return nil, fmt.Errorf("%d bytes are not %dx%d heights of 4 bytes each", len(raw), width, height)
	}
	out := make([]float32, width*height)
	for i := range out {
		out[i] = math.Float32frombits(binary.LittleEndian.Uint32(raw[4*i:]))
	}
	return out, nil
}

// loadReference reads a reference for a tile of the given size. A .f32 file holds
// heights. Any other file is a tile.
func loadReference(path string, width, height int) (*codec.Reference, error) {
	if filepath.Ext(path) != ".f32" {
		tile, _, err := loadTile(path)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		return codec.ReferenceTile(tile), nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	metres, err := parseHeights(raw, width, height)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return codec.ReferenceHeights(metres, width, height), nil
}
