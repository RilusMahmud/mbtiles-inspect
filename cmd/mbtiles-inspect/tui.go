package main

import (
	"context"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/spf13/cobra"

	"github.com/rilus/mbtiles-inspect/internal/mbtiles"
	"github.com/rilus/mbtiles-inspect/internal/tui"
)

func newTUICmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tui <file.mbtiles>",
		Short: "Launch the interactive TUI.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			noColor, _ := cmd.Flags().GetBool("no-color")
			return runTUI(cmd.Context(), args[0], noColor)
		},
	}
}

func runTUI(ctx context.Context, path string, noColor bool) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if noColor || os.Getenv("NO_COLOR") != "" {
		lipgloss.SetColorProfile(termenv.Ascii)
	}
	r, err := mbtiles.Open(path)
	if err != nil {
		return err
	}
	defer r.Close()

	model, err := tui.New(ctx, r)
	if err != nil {
		return fmt.Errorf("tui init: %w", err)
	}

	prog := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := prog.Run(); err != nil {
		return fmt.Errorf("tui run: %w", err)
	}
	return nil
}
