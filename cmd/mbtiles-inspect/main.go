// Command mbtiles-inspect inspects .mbtiles files from the terminal.
//
// Run `mbtiles-inspect --help` for usage.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// version is set by the build via -ldflags; default for `go run`.
var version = "dev"

func main() {
	root := newRootCmd()
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(exitCodeFor(err))
	}
}

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "mbtiles-inspect [file.mbtiles]",
		Short:         "Inspect .mbtiles files: metadata, zooms, gaps, tile previews.",
		Version:       version,
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			noColor, _ := cmd.Flags().GetBool("no-color")
			return runTUI(cmd.Context(), args[0], noColor)
		},
	}
	cmd.PersistentFlags().Bool("json", false, "emit JSON output where supported")
	cmd.PersistentFlags().Bool("no-color", false, "disable ANSI color")

	cmd.AddCommand(
		newInfoCmd(),
		newGapsCmd(),
		newVerifyCmd(),
		newDiffCmd(),
		newExtractCmd(),
		newTUICmd(),
	)
	return cmd
}
