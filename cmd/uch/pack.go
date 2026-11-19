// pack.go holds the pack command.

package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/wow-look-at-my/ultra-compact-world-heightmap/internal/archive"
	"github.com/wow-look-at-my/ultra-compact-world-heightmap/internal/terrarium"
)

var packMode string

var packCmd = &cobra.Command{
	Use:   "pack <directory of z-x-y tiles> <out.ucha>",
	Short: "Pack the tiles of one pyramid into a single archive",
	Long: `Pack the tiles of one pyramid into a single archive.

In chain mode the archive stores every tile, each against the tile above it. One tile
then decodes from the few tiles above it.

In leaves mode the archive stores only the tiles that their four children do not
rebuild exactly, and unpack rebuilds the rest. That is the smallest archive, and
reading a tile near the top means decoding every tile under it.`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		mode, ok := map[string]archive.Mode{"chain": archive.Chain, "leaves": archive.Leaves}[packMode]
		if !ok {
			return fmt.Errorf("mode %q is not chain or leaves", packMode)
		}
		paths, err := tilePaths(args[:1])
		if err != nil {
			return err
		}
		tiles := map[archive.Address]*terrarium.Tile{}
		served := int64(0)
		for _, path := range paths {
			z, x, y, err := tileAddress(path)
			if err != nil {
				return err
			}
			tile, size, err := loadTile(path)
			if err != nil {
				return fmt.Errorf("%s: %w", path, err)
			}
			tiles[archive.Address{Z: z, X: x, Y: y}] = tile
			served += size
		}
		packed, err := archive.Pack(tiles, mode)
		if err != nil {
			return err
		}
		if err := os.WriteFile(args[1], packed, 0o644); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s: %d tiles, %d bytes to %d bytes, %.2f%%\n", args[1], len(tiles), served, len(packed), 100*float64(len(packed))/float64(served))
		return nil
	},
}

func init() {
	packCmd.Flags().StringVar(&packMode, "mode", "leaves", "what to store: leaves or chain")
	rootCmd.AddCommand(packCmd)
}
