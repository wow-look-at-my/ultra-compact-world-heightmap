// stats.go holds trailingZeroBits.

package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/wow-look-at-my/ultra-compact-world-heightmap/internal/terrarium"
)

var statsCmd = &cobra.Command{
	Use:   "stats <tile.webp>...",
	Short: "Print what a tile holds: its quantum, its range, and the entropy simple predictors leave",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		for _, path := range args {
			line, err := statsLine(path)
			if err != nil {
				return fmt.Errorf("%s: %w", path, err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), line)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(statsCmd)
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

func entropy(values []int32) float64 {
	counts := map[int32]int{}
	for _, v := range values {
		counts[v]++
	}
	h := 0.0
	n := float64(len(values))
	for _, c := range counts {
		p := float64(c) / n
		h -= p * math.Log2(p)
	}
	return h
}

// trailingZeroBits counts the low bits that are clear in every sample.
func trailingZeroBits(samples []int32) int {
	all := int32(0)
	for _, v := range samples {
		all |= v
	}
	shift := 0
	for shift < 24 && all>>shift&1 == 0 {
		shift++
	}
	return shift
}

func statsLine(path string) (string, error) {
	t, size, err := loadTile(path)
	if err != nil {
		return "", err
	}
	w, h := t.Width, t.Height
	shift := trailingZeroBits(t.Samples)
	q := make([]int32, len(t.Samples))
	lo, hi := int32(math.MaxInt32), int32(math.MinInt32)
	sea := 0
	for i, v := range t.Samples {
		q[i] = v >> shift
		lo, hi = min(lo, v), max(hi, v)
		if v == 32768*256 {
			sea++
		}
	}
	var left, plane []int32
	for y := 1; y < h; y++ {
		for x := 1; x < w; x++ {
			i := y*w + x
			left = append(left, q[i]-q[i-1])
			plane = append(plane, q[i]-(q[i-1]+q[i-w]-q[i-w-1]))
		}
	}
	n := float64(len(q))
	return fmt.Sprintf("%-18s webp %7d %6.3f bpp  q 2^%-2d  %8.1f..%-8.1f m  sea %.2f  H0 %6.3f left %6.3f plane %6.3f = %7.0f B",
		filepath.Base(path), size, float64(size)*8/n, shift, terrarium.Metres(lo), terrarium.Metres(hi), float64(sea)/n,
		entropy(q), entropy(left), entropy(plane), entropy(plane)*n/8), nil
}
