package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
)

// DatasetItem is one row to upload. Title is purely informational and
// shows in the dry-run output; Content is the canonical bytes the
// SHA256 ID is computed from.
type DatasetItem struct {
	Title   string
	Content []byte
}

// Uploader pushes items to Langfuse with deterministic IDs.
type Uploader struct {
	Client    Client
	DatasetID string
	DryRun    bool
	Logger    *slog.Logger
}

// Upsert pushes every item. In DryRun the planned (id, title) pairs
// are logged and no HTTP calls fire. Returns the first error
// encountered.
func (u *Uploader) Upsert(ctx context.Context, items []DatasetItem) error {
	for _, it := range items {
		id := IDFor(it.Content)
		if u.DryRun {
			u.Logger.InfoContext(ctx, "would upload",
				"action", "upsert", "id", id, "title", it.Title)
			continue
		}
		if err := u.Client.UpsertDatasetItem(ctx, u.DatasetID, id, it.Content); err != nil {
			return fmt.Errorf("dataset-upload: upsert %s: %w", id, err)
		}
		u.Logger.DebugContext(ctx, "uploaded",
			"id", id, "dataset_id", u.DatasetID)
	}
	return nil
}

// ParseItems decodes the regression JSON envelope produced by
// dataset-bootstrap. The shape mirrors that tool's `fileEnvelope` but
// is decoded into a tool-agnostic shape so dataset-upload can stay
// independent of dataset-bootstrap's internal types.
func ParseItems(raw []byte) ([]DatasetItem, error) {
	var envelope struct {
		Version string `json:"version"`
		Sample  struct {
			Listings []struct {
				ID    string `json:"ID"`
				Title string `json:"Title"`
			} `json:"listings"`
		} `json:"sample"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("dataset-upload: parse envelope: %w", err)
	}
	if envelope.Version == "" {
		return nil, fmt.Errorf("dataset-upload: missing version field")
	}

	items := make([]DatasetItem, 0, len(envelope.Sample.Listings))
	for _, l := range envelope.Sample.Listings {
		content, err := json.Marshal(l)
		if err != nil {
			return nil, fmt.Errorf("dataset-upload: marshal listing %q: %w", l.ID, err)
		}
		items = append(items, DatasetItem{Title: l.Title, Content: content})
	}
	return items, nil
}
