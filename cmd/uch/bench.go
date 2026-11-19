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
	"github.com/wow-look-at-my/ultra-compact-world-heightmap/internal/terrarium"
)

var (
	benchPerTile bool
	benchMode    string
)

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
	benchCmd.Flags().StringVar(&benchMode, "mode", "both", "what to measure: alone, child (against the parent, for the tiles whose parent is among the tiles), or both")
	benchCmd.Flags().Var(tuneFlag{}, "set", "override a model parameter, as name=value")
	rootCmd.AddCommand(benchCmd)
}

type benchResult struct {
	path         string
	zoom         int
	webp, packed int
	// child is the size against the parent, and zero when the parent is not at hand.
	child int
	// derived says that those children are among the tiles and rebuild this tile exactly.
	derived bool
	elapsed time.Duration
}

// verify encodes a tile, decodes it, and gives the size once the samples match.
func verify(tile *terrarium.Tile, parent *codec.Parent) (int, error) {
	packed, err := codec.Encode(tile, parent)
	if err != nil {
		return 0, err
	}
	back, err := codec.Decode(packed, parent)
	if err != nil {
		return 0, err
	}
	if back.Width != tile.Width || back.Height != tile.Height || !slices.Equal(back.Samples, tile.Samples) {
		return 0, fmt.Errorf("the tile does not decode to the samples that went in")
	}
	return len(packed), nil
}

// parentOf finds the tile one zoom above among the tiles of the run.
func parentOf(path string, byAddress map[[3]int]string) (*codec.Parent, error) {
	z, x, y, err := tileAddress(path)
	if err != nil || z == 0 {
		return nil, nil
	}
	parentPath, ok := byAddress[[3]int{z - 1, x / 2, y / 2}]
	if !ok {
		return nil, nil
	}
	tile, _, err := loadTile(parentPath)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", parentPath, err)
	}
	return &codec.Parent{Tile: tile, Quadrant: y%2*2 + x%2}, nil
}

// tilePaths expands each directory into the WebP files under it.
func tilePaths(args []string) ([]string, error) {
	var paths []string
	for _, arg := range args {
		err := filepath.WalkDir(arg, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() && (path == arg || strings.HasSuffix(path, ".webp") || strings.HasSuffix(path, ".png")) {
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

func benchOne(path string, byAddress map[[3]int]string) (benchResult, error) {
	tile, size, err := loadTile(path)
	if err != nil {
		return benchResult{}, err
	}
	r := benchResult{path: path, zoom: tileZoom(path), webp: int(size)}
	start := time.Now()
	if benchMode != "child" {
		if r.packed, err = verify(tile, nil); err != nil {
			return r, err
		}
	}
	r.elapsed = time.Since(start)
	if z, x, y, err := tileAddress(path); err == nil {
		if r.derived, err = derives(tile, z, x, y, byAddress); err != nil {
			return r, err
		}
	}
	parent, err := parentOf(path, byAddress)
	if err != nil {
		return r, err
	}
	if parent != nil && benchMode != "alone" {
		if r.child, err = verify(tile, parent); err != nil {
			return r, fmt.Errorf("against its parent: %w", err)
		}
	}
	return r, nil
}

func benchAll(paths []string) ([]benchResult, error) {
	byAddress := map[[3]int]string{}
	for _, path := range paths {
		if z, x, y, err := tileAddress(path); err == nil {
			byAddress[[3]int{z, x, y}] = path
		}
	}
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
			results[i], errs[i] = benchOne(path, byAddress)
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
		tiles, webp, packed            int
		pTiles, pWebp, pPacked, pChild int
		elapsed                        time.Duration
		// chained codes each tile against its parent where it has one. leaves drops each tile that its children rebuild.
		chained, leaves, derived int
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
			fmt.Fprintf(&b, "%-22s webp %7d  uch %7d  %6.1f%%  child %7d  %s\n", filepath.Base(r.path), r.webp, r.packed, 100*float64(r.packed)/float64(r.webp), r.child, r.elapsed.Round(time.Millisecond))
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
			if r.derived {
				t.derived++
			} else {
				t.leaves += r.packed
			}
			t.chained += r.packed
			if r.child > 0 {
				t.chained += r.child - r.packed
			}
			if r.child > 0 {
				t.pTiles++
				t.pWebp += r.webp
				t.pPacked += r.packed
				t.pChild += r.child
			}
		}
	}
	zooms := make([]int, 0, len(byZoom))
	for z := range byZoom {
		zooms = append(zooms, z)
	}
	slices.Sort(zooms)
	line := func(name string, s *sum) {
		fmt.Fprintf(&b, "%-6s %4d tiles  webp %9d  uch %9d  %6.2f%%  %5.2f s/tile", name, s.tiles, s.webp, s.packed, 100*float64(s.packed)/float64(s.webp), s.elapsed.Seconds()/float64(s.tiles))
		if s.pTiles > 0 {
			fmt.Fprintf(&b, "  | %4d with parent  webp %9d  alone %9d  child %9d  %6.2f%% of webp  %6.2f%% of alone", s.pTiles, s.pWebp, s.pPacked, s.pChild, 100*float64(s.pChild)/float64(s.pWebp), 100*float64(s.pChild)/float64(s.pPacked))
		}
		b.WriteByte('\n')
	}
	for _, z := range zooms {
		line("z"+strconv.Itoa(z), byZoom[z])
	}
	line("total", total)
	if total.packed > 0 && total.pChild > 0 {
		share := func(n int) float64 { return 100 * float64(n) / float64(total.webp) }
		fmt.Fprintf(&b, "\nall %d tiles as one set:\n  webp as served             %10d\n  each tile alone            %10d  %6.2f%%\n  each tile against a parent %10d  %6.2f%%\n  without the %d tiles that their children rebuild exactly\n                             %10d  %6.2f%%\n",
			total.tiles, total.webp, total.packed, share(total.packed), total.chained, share(total.chained), total.derived, total.leaves, share(total.leaves))
	}
	return b.String()
}
