// costs.go holds the costs command, which shows where in a tile the bits go.

package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/wow-look-at-my/ultra-compact-world-heightmap/internal/codec"
	"github.com/wow-look-at-my/ultra-compact-world-heightmap/internal/terrarium"
)

var costsParent parentFlags

var costsCmd = &cobra.Command{
	Use:   "costs <tile.webp or tile.png>",
	Short: "Print the bits per sample that a tile takes, for each place in a block of four samples",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		tile, _, err := loadTile(args[0])
		if err != nil {
			return fmt.Errorf("%s: %w", args[0], err)
		}
		parent, err := costsParent.load(args[0])
		if err != nil {
			return err
		}
		costs, err := codec.CostMap(tile, parent)
		if err != nil {
			return err
		}
		var sum [4]float64
		var count [4]int
		for y := 0; y < tile.Height; y++ {
			for x := 0; x < tile.Width; x++ {
				place := y%2*2 + x%2
				sum[place] += float64(costs[y*tile.Width+x])
				count[place]++
			}
		}
		total := sum[0] + sum[1] + sum[2] + sum[3]
		mean := func(place int) float64 { return sum[place] / float64(max(count[place], 1)) }
		fmt.Fprintf(cmd.OutOrStdout(), "%.4f bits per sample, %.0f bytes\ntop left %.4f  top right %.4f\nbottom left %.4f  bottom right %.4f\n",
			total/float64(len(costs)), total/8, mean(0), mean(1), mean(2), mean(3))
		return nil
	},
}

func init() {
	costsParent.register(costsCmd)
	rootCmd.AddCommand(costsCmd)
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
	t, err := terrarium.Decode(f)
	return t, st.Size(), err
}
