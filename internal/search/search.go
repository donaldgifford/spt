package search

import (
	"context"
	"errors"

	"github.com/donaldgifford/spt/internal/domain"
)

// Sentinel errors implementations return.
var (
	// ErrNotFound is returned when a queried document does not exist
	// in the index.
	ErrNotFound = errors.New("search: not found")
)

// Search is the contract for the listings search index. Per ADR-0006
// the canonical backend is Meilisearch; this interface keeps the
// backend swappable for tests and future migrations.
//
// Phase 6 declares only the read/write surface — concrete index
// schema and analyzer configuration live with the search IMPL.
type Search interface {
	// IndexListing creates or replaces the document for l.
	IndexListing(ctx context.Context, l domain.Listing) error

	// DeleteListing removes the document for id; returns nil if the
	// document did not exist (delete is idempotent).
	DeleteListing(ctx context.Context, id domain.ListingID) error

	// Query runs a full-text search and returns matching listing IDs
	// in relevance order.
	Query(ctx context.Context, q Query) ([]domain.ListingID, error)

	// Ping verifies the search backend is reachable. Used by
	// /readyz probes.
	Ping(ctx context.Context) error
}

// Query is the parameter shape Query accepts. Filters mirror
// Meilisearch's filter syntax; the API layer translates user query
// strings into this shape.
type Query struct {
	Text    string
	Filters map[string]string
	Limit   int
	Offset  int
}
