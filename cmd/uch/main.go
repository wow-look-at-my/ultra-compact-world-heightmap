// Command uch compresses Terrarium elevation tiles.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:           "uch",
	Short:         "Losslessly compress Terrarium elevation tiles",
	SilenceUsage:  true,
	SilenceErrors: true,
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "uch:", err)
		os.Exit(1)
	}
}
