// decode.go holds the decode command.

package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/wow-look-at-my/ultra-compact-world-heightmap/internal/codec"
	"github.com/wow-look-at-my/ultra-compact-world-heightmap/internal/terrarium"
)

var decodeParent parentFlags

var decodeCmd = &cobra.Command{
	Use:   "decode <tile.uch> <out.png>",
	Short: "Restore a compressed tile to a Terrarium PNG with the samples that went in",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		packed, err := os.ReadFile(args[0])
		if err != nil {
			return err
		}
		var parent *codec.Parent
		if codec.NeedsParent(packed) {
			if decodeParent.path == "" {
				return fmt.Errorf("%s was coded against its parent: pass --parent", args[0])
			}
			if parent, err = decodeParent.load(args[0]); err != nil {
				return err
			}
		}
		tile, err := codec.Decode(packed, parent)
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

func init() {
	decodeParent.register(decodeCmd)
	rootCmd.AddCommand(decodeCmd)
}
