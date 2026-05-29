package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestDocsCmd_GeneratesMarkdownTree(t *testing.T) {
	// Build a minimal tree: root with one child. doc.GenMarkdownTree
	// emits a file per command, so we expect at least
	// "rootcli_child.md" plus the root index.
	root := &cobra.Command{Use: "rootcli", Short: "test root"}
	root.AddCommand(&cobra.Command{Use: "child", Short: "test child", Run: func(*cobra.Command, []string) {}})

	docs := newDocsCmd()
	root.AddCommand(docs)

	out := t.TempDir()
	docs.SetArgs([]string{out})
	require.NoError(t, docs.RunE(docs, []string{out}))

	entries, err := os.ReadDir(out)
	require.NoError(t, err)
	require.NotEmpty(t, entries, "doc tree should not be empty")

	childPath := filepath.Join(out, "rootcli_child.md")
	_, err = os.Stat(childPath)
	require.NoError(t, err, "child command markdown should be generated")
}

func TestDocsCmd_HiddenInTree(t *testing.T) {
	docs := newDocsCmd()
	require.True(t, docs.Hidden, "gen-docs must be Hidden to stay out of `spt --help`")
}
