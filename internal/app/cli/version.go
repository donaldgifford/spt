package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"runtime"

	"github.com/spf13/cobra"
)

// versionPayload is the JSON shape emitted by `spt version --json`.
type versionPayload struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Date    string `json:"date"`
	Go      string `json:"go"`
}

func newVersionCmd(info BuildInfo) *cobra.Command {
	var asJSON bool

	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print version, commit, and build info",
		RunE: func(cmd *cobra.Command, _ []string) error {
			payload := versionPayload{
				Version: info.Version,
				Commit:  info.Commit,
				Date:    info.Date,
				Go:      runtime.Version(),
			}

			if asJSON {
				return writeVersionJSON(cmd.OutOrStdout(), payload)
			}
			return writeVersionText(cmd.OutOrStdout(), payload)
		},
	}

	// Skip the persistent slog setup for `spt version` — version output
	// goes to stdout and the user does not need logger configuration to
	// just print a string.
	cmd.PersistentPreRunE = noopPreRun

	cmd.Flags().BoolVar(&asJSON, "json", false, "Emit machine-readable JSON to stdout")
	return cmd
}

func writeVersionText(w io.Writer, p versionPayload) error {
	if _, err := fmt.Fprintf(w, "spt %s\ncommit:  %s\ndate:    %s\ngo:      %s\n",
		p.Version, p.Commit, p.Date, p.Go); err != nil {
		return fmt.Errorf("cli: write version text: %w", err)
	}
	return nil
}

func writeVersionJSON(w io.Writer, p versionPayload) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(p); err != nil {
		return fmt.Errorf("cli: encode version json: %w", err)
	}
	return nil
}

// noopPreRun is used by subcommands (like `version`) that do not need
// the root's slog installation to fire. Cobra resolves PersistentPreRunE
// by walking up the command tree; setting it explicitly on the
// subcommand short-circuits the inherited one.
func noopPreRun(_ *cobra.Command, _ []string) error { return nil }
