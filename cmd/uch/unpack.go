// unpack.go holds the unpack command.

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/wow-look-at-my/ultra-compact-world-heightmap/internal/archive"
	"github.com/wow-look-at-my/ultra-compact-world-heightmap/internal/terrarium"
)

var unpackCmd = &cobra.Command{
	Use:   "unpack <in.ucha> <directory>",
	Short: "Restore every tile of an archive as a Terrarium PNG named z-x-y.png",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		data, err := os.ReadFile(args[0])
		if err != nil {
			return err
		}
		tiles, err := archive.Unpack(data)
		if err != nil {
			return fmt.Errorf("%s: %w", args[0], err)
		}
		if err := os.MkdirAll(args[1], 0o755); err != nil {
			return err
		}
		for a, tile := range tiles {
			f, err := os.Create(filepath.Join(args[1], a.String()+".png"))
			if err != nil {
				return err
			}
			if err := terrarium.EncodePNG(f, tile); err != nil {
				f.Close()
				return err
			}
			if err := f.Close(); err != nil {
				return err
			}
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s: %d tiles\n", args[1], len(tiles))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(unpackCmd)
}
