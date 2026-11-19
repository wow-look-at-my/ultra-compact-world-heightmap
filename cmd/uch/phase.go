// phase.go holds the phase command, which shows where on the source grid the bits go.

package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/wow-look-at-my/ultra-compact-world-heightmap/internal/codec"
	"github.com/wow-look-at-my/ultra-compact-world-heightmap/internal/geo"
	"github.com/wow-look-at-my/ultra-compact-world-heightmap/internal/terrarium"
)

const phaseBins = 12

var phaseCmd = &cobra.Command{
	Use:   "phase <z-x-y.webp>...",
	Short: "Print the bits per sample against the place of the sample in the cell of the GLO-30 source grid",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		for _, path := range args {
			report, err := phaseReport(path)
			if err != nil {
				return fmt.Errorf("%s: %w", path, err)
			}
			fmt.Fprint(cmd.OutOrStdout(), report)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(phaseCmd)
}

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
	t, err := terrarium.DecodeWebP(f)
	return t, st.Size(), err
}

func phaseReport(path string) (string, error) {
	z, x, y, err := tileAddress(path)
	if err != nil {
		return "", err
	}
	tile, _, err := loadTile(path)
	if err != nil {
		return "", err
	}
	costs, err := codec.CostMap(tile)
	if err != nil {
		return "", err
	}
	var sum [phaseBins][phaseBins]float64
	var count [phaseBins][phaseBins]int
	total := 0.0
	lonStep := geo.GLO30LonStep(geo.LatArcsec(z, y, tile.Height/2, tile.Height))
	for row := 0; row < tile.Height; row++ {
		by := int(geo.Phase(geo.LatArcsec(z, y, row, tile.Height), 1) * phaseBins)
		for col := 0; col < tile.Width; col++ {
			bx := int(geo.Phase(geo.LonArcsec(z, x, col, tile.Width), lonStep) * phaseBins)
			c := float64(costs[row*tile.Width+col])
			sum[by][bx] += c
			count[by][bx]++
			total += c
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d-%d-%d: %.3f bits per sample, %.0f bytes. Rows are the latitude phase, columns the longitude phase.\n", z, x, y, total/float64(len(costs)), total/8)
	for by := range sum {
		for bx := range sum[by] {
			fmt.Fprintf(&b, " %5.2f", sum[by][bx]/float64(max(count[by][bx], 1)))
		}
		b.WriteByte('\n')
	}
	return b.String(), nil
}
