// encode.go holds the encode command.

package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/wow-look-at-my/ultra-compact-world-heightmap/internal/codec"
)

var encodeParent parentFlags

var encodeCmd = &cobra.Command{
	Use:   "encode <tile.webp or tile.png> <out.uch>",
	Short: "Compress a Terrarium tile",
	Long: `Compress a Terrarium tile.

With --parent, the tile is coded against the tile one zoom above it, which makes it
smaller. Decoding then needs the same parent. A tile that is not the pipeline's
downsampling source for that parent is coded alone, and decodes without one.`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		tile, size, err := loadTile(args[0])
		if err != nil {
			return fmt.Errorf("%s: %w", args[0], err)
		}
		parent, err := encodeParent.load(args[0])
		if err != nil {
			return err
		}
		packed, err := codec.Encode(tile, parent)
		if err != nil {
			return err
		}
		if err := os.WriteFile(args[1], packed, 0o644); err != nil {
			return err
		}
		how := "decodes alone"
		if codec.NeedsParent(packed) {
			how = "decodes with its parent"
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s: %d bytes to %d bytes, %.1f%%, %s\n", args[1], size, len(packed), 100*float64(len(packed))/float64(size), how)
		return nil
	},
}

func init() {
	encodeParent.register(encodeCmd)
	rootCmd.AddCommand(encodeCmd)
}
