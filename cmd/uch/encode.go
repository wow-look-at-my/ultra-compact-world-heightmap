// encode.go holds the encode command.

package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/wow-look-at-my/ultra-compact-world-heightmap/internal/codec"
	"github.com/wow-look-at-my/ultra-compact-world-heightmap/internal/terrarium"
)

var (
	encodeParent    parentFlags
	encodeReference string
)

var encodeCmd = &cobra.Command{
	Use:   "encode <tile.webp or tile.png> <out.uch>",
	Short: "Compress a Terrarium tile",
	Long: `Compress a Terrarium tile.

With --parent, the tile is coded against the tile one zoom above it, which makes it
smaller. Decoding then needs the same parent. A tile that is not the pipeline's
downsampling source for that parent is coded alone, and decodes without one.

With --reference, the tile is coded as its difference from a tile of the same size.
The result is small only when both tiles are almost equal: an older version of the
tile, or the output of the pipeline run again on the source data. Decoding needs the
same reference.`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		tile, size, err := loadTile(args[0])
		if err != nil {
			return fmt.Errorf("%s: %w", args[0], err)
		}
		packed, err := encodeTile(tile, args[0])
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
		if codec.NeedsReference(packed) {
			how = "decodes with its reference"
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s: %d bytes to %d bytes, %.1f%%, %s\n", args[1], size, len(packed), 100*float64(len(packed))/float64(size), how)
		return nil
	},
}

// encodeTile codes the tile at path the way the flags ask for.
func encodeTile(tile *terrarium.Tile, path string) ([]byte, error) {
	if encodeReference == "" {
		parent, err := encodeParent.load(path)
		if err != nil {
			return nil, err
		}
		return codec.Encode(tile, parent)
	}
	if encodeParent.path != "" {
		return nil, errors.New("a tile is coded against a parent or against a reference, not both")
	}
	ref, err := loadReference(encodeReference, tile.Width, tile.Height)
	if err != nil {
		return nil, err
	}
	return codec.EncodeReference(tile, ref)
}

func init() {
	encodeParent.register(encodeCmd)
	encodeCmd.Flags().StringVar(&encodeReference, "reference", "", "what the tile is almost equal to: "+referenceHelp)
	rootCmd.AddCommand(encodeCmd)
}
