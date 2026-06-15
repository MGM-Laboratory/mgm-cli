package cmd

import (
	"bytes"
	"context"
	"log/slog"
	"os"

	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/term"
)

// Run launches the embedded Crush root command with the given context and
// arguments, returning any error instead of calling os.Exit (unlike Execute,
// which is the standalone-process entrypoint). With empty args the interactive
// TUI starts. This is the seam the mgm CLI's `mgm megumi` command calls so the
// agent runs in-process as part of the single mgm binary.
func Run(ctx context.Context, args []string) error {
	// Mirror Execute's early-log discard: config.Load logs via slog before the
	// file logger is configured.
	slog.SetDefault(slog.New(slog.DiscardHandler))

	if term.IsTerminal(os.Stdout.Fd()) {
		var b bytes.Buffer
		w := colorprofile.NewWriter(os.Stdout, os.Environ())
		w.Forward = &b
		_, _ = w.WriteString(heartbit.String())
		rootCmd.SetVersionTemplate(b.String() + "\n" + defaultVersionTemplate)
	}

	rootCmd.SetArgs(args)
	return rootCmd.ExecuteContext(ctx)
}
