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
			if err := printStats(cmd, path); err != nil {
				return fmt.Errorf("%s: %w", path, err)
			}
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

func printStats(cmd *cobra.Command, path string) error {
	t, size, err := loadTile(path)
	if err != nil {
		return err
	}
	w, h := t.Width, t.Height
	shift := 0
	for shift < 24 {
		clear := true
		for _, v := range t.Samples {
			if v>>shift&1 != 0 {
				clear = false
				break
			}
		}
		if !clear {
			break
		}
		shift++
	}
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
	var left, plane, second []int32
	zeroPlane := 0
	for y := 2; y < h; y++ {
		for x := 2; x < w; x++ {
			i := y*w + x
			left = append(left, q[i]-q[i-1])
			p := q[i] - (q[i-1] + q[i-w] - q[i-w-1])
			plane = append(plane, p)
			if p == 0 {
				zeroPlane++
			}
			// Second order in both directions: 2W - WW + 2N - NN - (2NW - NWW ... ) kept simple.
			second = append(second, q[i]-(2*q[i-1]-q[i-2]+2*q[i-w]-q[i-2*w])/2)
		}
	}
	out := cmd.OutOrStdout()
	n := float64(len(q))
	fmt.Fprintf(out, "%s %dx%d webp=%d (%.3f bpp)\n", filepath.Base(path), w, h, size, float64(size)*8/n)
	fmt.Fprintf(out, "  quantum 2^%d/256 m  range %.2f..%.2f m  sea %.3f\n", shift, terrarium.Metres(lo), terrarium.Metres(hi), float64(sea)/n)
	fmt.Fprintf(out, "  H0 %.3f  left %.3f  plane %.3f (zero %.3f)  second %.3f  -> plane bytes %.0f\n",
		entropy(q), entropy(left), entropy(plane), float64(zeroPlane)/float64(len(plane)), entropy(second), entropy(plane)*n/8)
	return nil
}
