package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenerate_WriteThenValidate(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, Generate(&bytes.Buffer{}, dir, ModeWrite))

	// Every spec should now exist on disk.
	for _, spec := range DashboardSpecs() {
		_, err := os.Stat(filepath.Join(dir, spec.File))
		require.NoErrorf(t, err, "dashboard %s missing after write", spec.File)
	}
	for _, rf := range RuleFiles() {
		_, err := os.Stat(filepath.Join(dir, rf.File))
		require.NoErrorf(t, err, "rule file %s missing after write", rf.File)
	}

	// Validate should be a no-op against the just-written tree.
	require.NoError(t, Generate(&bytes.Buffer{}, dir, ModeValidate))
}

func TestGenerate_DetectsDrift(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, Generate(&bytes.Buffer{}, dir, ModeWrite))

	// Mutate one byte on the first dashboard.
	target := filepath.Join(dir, DashboardSpecs()[0].File)
	data, err := os.ReadFile(target)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(target, append(data, '!'), 0o600))

	err = Generate(&bytes.Buffer{}, dir, ModeValidate)
	require.ErrorIs(t, err, ErrDriftDetected)
}

func TestGenerate_DetectsMissingFile(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, Generate(&bytes.Buffer{}, dir, ModeWrite))

	target := filepath.Join(dir, DashboardSpecs()[0].File)
	require.NoError(t, os.Remove(target))

	err := Generate(&bytes.Buffer{}, dir, ModeValidate)
	require.ErrorIs(t, err, ErrDriftDetected)
}

func TestDashboardsProduceValidJSON(t *testing.T) {
	for _, spec := range DashboardSpecs() {
		raw, err := marshalDashboard(spec.Build())
		require.NoErrorf(t, err, spec.Name)
		var got map[string]any
		require.NoErrorf(t, json.Unmarshal(raw, &got), spec.Name)
		require.NotEmpty(t, got["title"], spec.Name)
		require.NotEmpty(t, got["panels"], spec.Name)
	}
}
