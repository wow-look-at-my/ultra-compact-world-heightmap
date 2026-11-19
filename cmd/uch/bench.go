// bench.go holds the bench command, which measures the codec against the tiles as served.

package main

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/wow-look-at-my/ultra-compact-world-heightmap/internal/codec"
)

var benchPerTile bool

var benchCmd = &cobra.Command{
	Use:   "bench <tile.webp or directory>...",
	Short: "Compress each tile, check that it decodes to the same samples, and compare sizes with the WebP",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := tilePaths(args)
		if err != nil {
			return err
		}
		results, err := benchAll(paths)
		if err != nil {
			return err
		}
		fmt.Fprint(cmd.OutOrStdout(), benchReport(results, benchPerTile))
		return nil
	},
}

func init() {
	benchCmd.Flags().BoolVar(&benchPerTile, "per-tile", false, "print a line for each tile")
	benchCmd.Flags().Var(tuneFlag{}, "set", "override a model parameter, as name=value")
	rootCmd.AddCommand(benchCmd)
}

type benchResult struct {
	path         string
	zoom         int
	webp, packed int
	elapsed      time.Duration
}

// tilePaths expands each directory into the WebP files under it.
func tilePaths(args []string) ([]string, error) {
	var paths []string
	for _, arg := range args {
		err := filepath.WalkDir(arg, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() && (path == arg || strings.HasSuffix(path, ".webp")) {
				paths = append(paths, path)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("no tiles found")
	}
	return paths, nil
}

func tileZoom(path string) int {
	z, err := strconv.Atoi(strings.SplitN(filepath.Base(path), "-", 2)[0])
	if err != nil {
		return -1
	}
	return z
}

func benchOne(path string) (benchResult, error) {
	tile, size, err := loadTile(path)
	if err != nil {
		return benchResult{}, err
	}
	start := time.Now()
	packed, err := codec.Encode(tile)
	if err != nil {
		return benchResult{}, err
	}
	back, err := codec.Decode(packed)
	if err != nil {
		return benchResult{}, err
	}
	if back.Width != tile.Width || back.Height != tile.Height || !slices.Equal(back.Samples, tile.Samples) {
		return benchResult{}, fmt.Errorf("the tile does not decode to the samples that went in")
	}
	return benchResult{path: path, zoom: tileZoom(path), webp: int(size), packed: len(packed), elapsed: time.Since(start)}, nil
}

func benchAll(paths []string) ([]benchResult, error) {
	results := make([]benchResult, len(paths))
	errs := make([]error, len(paths))
	var wg sync.WaitGroup
	slots := make(chan struct{}, runtime.NumCPU())
	for i, path := range paths {
		wg.Add(1)
		go func() {
			defer wg.Done()
			slots <- struct{}{}
			defer func() { <-slots }()
			results[i], errs[i] = benchOne(path)
		}()
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			return nil, fmt.Errorf("%s: %w", paths[i], err)
		}
	}
	return results, nil
}

func benchReport(results []benchResult, perTile bool) string {
	var b strings.Builder
	type sum struct {
		tiles, webp, packed int
		elapsed             time.Duration
	}
	byZoom := map[int]*sum{}
	total := &sum{}
	slices.SortFunc(results, func(a, b benchResult) int {
		if a.zoom != b.zoom {
			return a.zoom - b.zoom
		}
		return strings.Compare(a.path, b.path)
	})
	for _, r := range results {
		if perTile {
			fmt.Fprintf(&b, "%-22s webp %7d  uch %7d  %6.1f%%  %s\n", filepath.Base(r.path), r.webp, r.packed, 100*float64(r.packed)/float64(r.webp), r.elapsed.Round(time.Millisecond))
		}
		s := byZoom[r.zoom]
		if s == nil {
			s = &sum{}
			byZoom[r.zoom] = s
		}
		for _, t := range []*sum{s, total} {
			t.tiles++
			t.webp += r.webp
			t.packed += r.packed
			t.elapsed += r.elapsed
		}
	}
	zooms := make([]int, 0, len(byZoom))
	for z := range byZoom {
		zooms = append(zooms, z)
	}
	slices.Sort(zooms)
	line := func(name string, s *sum) {
		fmt.Fprintf(&b, "%-6s %4d tiles  webp %9d  uch %9d  %6.2f%%  %5.2f s/tile\n", name, s.tiles, s.webp, s.packed, 100*float64(s.packed)/float64(s.webp), s.elapsed.Seconds()/float64(s.tiles))
	}
	for _, z := range zooms {
		line("z"+strconv.Itoa(z), byZoom[z])
	}
	line("total", total)
	return b.String()
}
