// decode.go holds the decode command.

package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/wow-look-at-my/ultra-compact-world-heightmap/internal/codec"
	"github.com/wow-look-at-my/ultra-compact-world-heightmap/internal/terrarium"
)

var (
	decodeParent    parentFlags
	decodeReference string
)

var decodeCmd = &cobra.Command{
	Use:   "decode <tile.uch> <out.png>",
	Short: "Restore a compressed tile to a Terrarium PNG with the samples that went in",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		packed, err := os.ReadFile(args[0])
		if err != nil {
			return err
		}
		tile, err := decodeTile(packed, args[0])
		if err != nil {
			return fmt.Errorf("%s: %w", args[0], err)
		}
		out, err := os.Create(args[1])
		if err != nil {
			return err
		}
		if err := terrarium.EncodePNG(out, tile); err != nil {
			out.Close()
			return err
		}
		return out.Close()
	},
}

// decodeTile restores the tile at path with the parent or the reference it was coded against.
func decodeTile(packed []byte, path string) (*terrarium.Tile, error) {
	if codec.NeedsReference(packed) {
		if decodeReference == "" {
			return nil, fmt.Errorf("%s was coded against a reference: pass --reference", path)
		}
		ref, _, err := loadTile(decodeReference)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", decodeReference, err)
		}
		return codec.DecodeReference(packed, ref)
	}
	var parent *codec.Parent
	if codec.NeedsParent(packed) {
		if decodeParent.path == "" {
			return nil, fmt.Errorf("%s was coded against its parent: pass --parent", path)
		}
		var err error
		if parent, err = decodeParent.load(path); err != nil {
			return nil, err
		}
	}
	return codec.Decode(packed, parent)
}

func init() {
	decodeParent.register(decodeCmd)
	decodeCmd.Flags().StringVar(&decodeReference, "reference", "", "the tile that the encoder was given as its reference")
	rootCmd.AddCommand(decodeCmd)
}
