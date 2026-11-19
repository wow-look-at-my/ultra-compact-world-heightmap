// warp.go holds the warp command.

package main

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"

	"github.com/spf13/cobra"

	"github.com/wow-look-at-my/ultra-compact-world-heightmap/internal/warp"
)

var warpFlags struct {
	zoom, x, y, columns int
	west, north         float64
	stepLon, stepLat    float64
}

var warpCmd = &cobra.Command{
	Use:   "warp <grid.f32> <out.f32>",
	Short: "Reproject a grid of heights in degrees to the heights of one tile",
	Long: `Reproject a grid of heights in degrees to the heights of one tile.

This is the reprojection step of the Mapterhorn pipeline, a cubic B-spline over the
source grid, without GDAL. It gives the same bits on every machine, so a tile coded
with --reference against its output decodes wherever the same grid is at hand.

The grid is little-endian float32 metres in row order, north first. --west and --north
name the centre of its first sample.`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		raw, err := os.ReadFile(args[0])
		if err != nil {
			return err
		}
		f := warpFlags
		if f.columns < 1 || len(raw)%(4*f.columns) != 0 {
			return fmt.Errorf("%s: %d bytes are not rows of %d heights of 4 bytes each", args[0], len(raw), f.columns)
		}
		rows := len(raw) / (4 * f.columns)
		heights, err := parseHeights(raw, f.columns, rows)
		if err != nil {
			return fmt.Errorf("%s: %w", args[0], err)
		}
		grid := &warp.Grid{Width: f.columns, Height: rows, West: f.west, North: f.north, StepLon: f.stepLon, StepLat: f.stepLat, Heights: heights}
		tile, err := warp.Tile(grid, f.zoom, f.x, f.y, 512)
		if err != nil {
			return err
		}
		out := make([]byte, 0, 4*len(tile))
		for _, v := range tile {
			out = binary.LittleEndian.AppendUint32(out, math.Float32bits(v))
		}
		return os.WriteFile(args[1], out, 0o644)
	},
}

func init() {
	f := warpCmd.Flags()
	f.IntVar(&warpFlags.zoom, "zoom", 12, "the zoom of the tile")
	f.IntVar(&warpFlags.x, "x", 0, "the column of the tile")
	f.IntVar(&warpFlags.y, "y", 0, "the row of the tile")
	f.IntVar(&warpFlags.columns, "columns", 3600, "heights in one row of the grid")
	f.Float64Var(&warpFlags.west, "west", 0, "the longitude of the centre of the first height, in degrees")
	f.Float64Var(&warpFlags.north, "north", 0, "the latitude of the centre of the first height, in degrees")
	f.Float64Var(&warpFlags.stepLon, "step-lon", 1.0/3600, "the degrees of longitude from one height to the next")
	f.Float64Var(&warpFlags.stepLat, "step-lat", 1.0/3600, "the degrees of latitude from one height to the next")
	rootCmd.AddCommand(warpCmd)
}
