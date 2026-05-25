package datastore

import (
	"context"
	"errors"
	"time"

	"github.com/donaldgifford/spt/internal/domain"
)

// Sentinel errors implementations return.
var (
	// ErrNotFound is the canonical "row does not exist" signal.
	// Callers branch on errors.Is(err, datastore.ErrNotFound) rather
	// than inspecting backend-specific SQL errors.
	ErrNotFound = errors.New("datastore: not found")

	// ErrConflict signals a uniqueness/constraint violation on
	// upsert (e.g., a duplicate ebay_item_id).
	ErrConflict = errors.New("datastore: conflict")
)

// Datastore is the canonical store contract. Per DESIGN-0002 §
// "Datastore". The interface is intentionally broad — see the design
// doc for the "we don't pre-split this" rationale.
//
// Postgres is the canonical backend (ADR-0004). Phase 6 only
// declares the contract; the Postgres implementation lands with the
// datastore IMPL.
type Datastore interface {
	// Watches
	GetWatch(ctx context.Context, id domain.WatchID) (domain.Watch, error)
	ListWatches(ctx context.Context, f WatchFilter) ([]domain.Watch, error)
	UpsertWatch(ctx context.Context, w domain.Watch) error
	DueWatches(ctx context.Context, now time.Time, limit int) ([]domain.Watch, error)

	// Listings
	GetListing(ctx context.Context, id domain.ListingID) (domain.Listing, error)
	UpsertListing(ctx context.Context, l domain.Listing) error
	GetListingByEbayItemID(ctx context.Context, ebayID string) (domain.Listing, error)

	// Components
	ReplaceComponents(ctx context.Context, listingID domain.ListingID, components []domain.Component) error

	// Jobs and Tasks
	CreateJob(ctx context.Context, j domain.Job) error
	UpdateJobState(ctx context.Context, id domain.JobID, state domain.JobState) error
	UpsertTask(ctx context.Context, t domain.Task) error

	// Ping verifies the database is reachable. Used by /readyz probes.
	Ping(ctx context.Context) error
}

// WatchFilter parameterizes ListWatches. domain.WatchFilter and this
// type intentionally overlap — the domain version is the data shape,
// this is the query shape they aren't always identical.
type WatchFilter = domain.WatchFilter
