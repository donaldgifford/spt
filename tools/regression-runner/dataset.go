package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/donaldgifford/spt/internal/domain"
)

// DatasetEntry is one (listing, expected components) pair the runner
// feeds into each backend.
type DatasetEntry struct {
	Listing  domain.Listing     `json:"listing"`
	Expected []domain.Component `json:"expected"`
}

// LoadDataset accepts either a local directory of *.json files or a
// langfuse:// URL. The Langfuse path needs the upload tool's Client
// to fetch dataset items — until that wiring lands, the URL form
// returns ErrLangfuseDatasetNotWired so operators see a clear error.
func LoadDataset(path string) ([]DatasetEntry, error) {
	if strings.HasPrefix(path, "langfuse://") {
		return nil, ErrLangfuseDatasetNotWired
	}
	return loadLocalDataset(os.DirFS(path))
}

// ErrLangfuseDatasetNotWired indicates the langfuse:// URL form is
// declared but not yet implemented in this IMPL — the full regression
// set is in Langfuse, and that wiring lands when the agent IMPL adds
// the consuming code.
var ErrLangfuseDatasetNotWired = errors.New(
	"regression-runner: langfuse:// dataset URLs not yet wired " +
		"(use a local --dataset path until the agent IMPL lands)",
)

func loadLocalDataset(fsys fs.FS) ([]DatasetEntry, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, fmt.Errorf("regression-runner: read dataset dir: %w", err)
	}
	out := make([]DatasetEntry, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, err := fs.ReadFile(fsys, e.Name())
		if err != nil {
			return nil, fmt.Errorf("regression-runner: read %s: %w", e.Name(), err)
		}
		var entry DatasetEntry
		if err := json.Unmarshal(data, &entry); err != nil {
			return nil, fmt.Errorf("regression-runner: parse %s: %w", e.Name(), err)
		}
		out = append(out, entry)
	}
	return out, nil
}
