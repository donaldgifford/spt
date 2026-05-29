package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

// newDocsCmd returns the hidden `gen-docs <output-dir>` subcommand
// used by the docgen workflow (`just docs-cli`). The implementation is
// one call into spf13/cobra/doc — too small to justify its own tool
// binary under tools/ (see IMPL-0002 Phase 2 and DESIGN-0006 "docgen").
//
// Hidden:true keeps the command out of `spt --help`; it's a maintainer
// tool, not part of the operator surface.
func newDocsCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "gen-docs <output-dir>",
		Short:  "Regenerate the docs/cli/ markdown tree from the live command set",
		Hidden: true,
		Args:   cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := doc.GenMarkdownTree(cmd.Root(), args[0]); err != nil {
				return fmt.Errorf("cli: gen-docs: %w", err)
			}
			return nil
		},
	}
}
