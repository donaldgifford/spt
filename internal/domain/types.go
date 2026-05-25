package domain

import (
	"encoding/json"
	"time"
)

// Phase 6 (IMPL-0001) ships placeholder shapes for the domain types
// the service interfaces reference. Field sets are deliberately
// minimal — every IMPL that owns a table flesh-out its primary type
// when it lands. The IDs and enums are stable; the structs are not.

// Watch is the user-defined query that drives pollers + alerts.
// Owned by the datastore IMPL.
type Watch struct {
	ID       WatchID
	Name     string
	Query    string
	Cadence  time.Duration
	Disabled bool
}

// WatchFilter is the query shape Datastore.ListWatches accepts.
type WatchFilter struct {
	IncludeDisabled bool
	Limit           int
}

// Listing is one eBay listing snapshot. Owned by the datastore IMPL.
type Listing struct {
	ID         ListingID
	WatchID    WatchID
	EbayItemID string
	Title      string
}

// Component is one parsed hardware unit attached to a Listing.
// Owned by the extract IMPL.
type Component struct {
	ID        ComponentID
	ListingID ListingID
	Kind      string
}

// Job is one scheduler-spawned unit of work that fans out into Tasks
// across a stage DAG. Owned by the scheduler IMPL.
type Job struct {
	ID        JobID
	WatchID   WatchID
	State     JobState
	Trigger   JobTrigger
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Task is one stage handler invocation. Owned by the scheduler IMPL.
type Task struct {
	ID        TaskID
	JobID     JobID
	Stage     Stage
	State     TaskState
	Input     json.RawMessage
	Output    json.RawMessage
	Attempts  int
	CreatedAt time.Time
	UpdatedAt time.Time
}
