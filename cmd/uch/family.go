// family.go holds the family command, which checks the parent rule against real tiles.

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/wow-look-at-my/ultra-compact-world-heightmap/internal/pyramid"
	"github.com/wow-look-at-my/ultra-compact-world-heightmap/internal/terrarium"
)

var familyCmd = &cobra.Command{
	Use:   "family <parent.webp> <directory of children>",
	Short: "Rebuild a tile from its four children and count the samples that differ from the tile as served",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		line, err := familyLine(args[0], args[1])
		if err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), line)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(familyCmd)
}

// tileAddress reads z, x and y from a name like z-x-y.webp.
func tileAddress(path string) (z, x, y int, err error) {
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	if _, err := fmt.Sscanf(name, "%d-%d-%d", &z, &x, &y); err != nil {
		return 0, 0, 0, fmt.Errorf("%s is not named z-x-y", path)
	}
	return z, x, y, nil
}

// loadChildren reads those children of a tile from dir. A missing file leaves a nil child.
func loadChildren(dir string, z, x, y int) ([4]*terrarium.Tile, int, error) {
	var children [4]*terrarium.Tile
	found := 0
	for k := range children {
		path := filepath.Join(dir, fmt.Sprintf("%d-%d-%d.webp", z+1, 2*x+k%2, 2*y+k/2))
		tile, _, err := loadTile(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return children, 0, fmt.Errorf("%s: %w", path, err)
		}
		children[k] = tile
		found++
	}
	return children, found, nil
}

func familyLine(parentPath, dir string) (string, error) {
	z, x, y, err := tileAddress(parentPath)
	if err != nil {
		return "", err
	}
	served, _, err := loadTile(parentPath)
	if err != nil {
		return "", err
	}
	children, found, err := loadChildren(dir, z, x, y)
	if err != nil {
		return "", err
	}
	if found == 0 {
		return "", fmt.Errorf("%s holds no child of %d-%d-%d", dir, z, x, y)
	}
	built, err := pyramid.Parent(children, z)
	if err != nil {
		return "", err
	}
	differ, covered := 0, 0
	for k, c := range children {
		if c == nil {
			continue
		}
		ox, oy := (k%2)*built.Width/2, (k/2)*built.Height/2
		for yy := 0; yy < built.Height/2; yy++ {
			for xx := 0; xx < built.Width/2; xx++ {
				i := (oy+yy)*built.Width + ox + xx
				covered++
				if built.Samples[i] != served.Samples[i] {
					differ++
				}
			}
		}
	}
	return fmt.Sprintf("%d-%d-%d: %d children, %d of %d samples differ from the served tile", z, x, y, found, differ, covered), nil
}
