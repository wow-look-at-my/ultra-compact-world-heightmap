// round.go holds the round command.

package main

import (
	"fmt"
	"math"
	"os"

	"github.com/spf13/cobra"

	"github.com/wow-look-at-my/ultra-compact-world-heightmap/internal/pyramid"
	"github.com/wow-look-at-my/ultra-compact-world-heightmap/internal/terrarium"
)

var (
	roundZoom   int
	roundWidth  int
	roundHeight int
)

const seaSample = 32768 * 256

var roundCmd = &cobra.Command{
	Use:   "round <heights.f32> <out.png>",
	Short: "Round a raster of heights to a Terrarium tile, the way the pipeline rounds at a zoom",
	Long: `Round a raster of heights to a Terrarium tile.

The input is little-endian float32 metres in row order, such as gdal_translate writes
with -of ENVI -ot Float32. Each height is rounded half to even to the step of the zoom,
which is what the Mapterhorn pipeline does to a reprojected tile.`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		raw, err := os.ReadFile(args[0])
		if err != nil {
			return err
		}
		tile, err := roundHeights(raw, roundWidth, roundHeight, roundZoom)
		if err != nil {
			return fmt.Errorf("%s: %w", args[0], err)
		}
		out, err := os.Create(args[1])
		if err != nil {
			return err
		}
		if err := terrarium.EncodePNG(out, tile); err != nil {
			out.Close()
			return err
		}
		return out.Close()
	},
}

func roundHeights(raw []byte, width, height, zoom int) (*terrarium.Tile, error) {
	values, err := parseHeights(raw, width, height)
	if err != nil {
		return nil, err
	}
	step := float64(pyramid.Quantum(zoom))
	tile := &terrarium.Tile{Width: width, Height: height, Samples: make([]int32, width*height)}
	for i := range tile.Samples {
		metres := float64(values[i])
		v := math.RoundToEven(metres*256/step)*step + seaSample
		if !(v >= 0 && v < 1<<24) {
			return nil, fmt.Errorf("height %d is %v metres, outside what a tile holds", i, metres)
		}
		tile.Samples[i] = int32(v)
	}
	return tile, nil
}

func init() {
	roundCmd.Flags().IntVar(&roundZoom, "zoom", 12, "the zoom of the tile, which sets the step")
	roundCmd.Flags().IntVar(&roundWidth, "width", 512, "samples in a row")
	roundCmd.Flags().IntVar(&roundHeight, "height", 512, "rows")
	rootCmd.AddCommand(roundCmd)
}
