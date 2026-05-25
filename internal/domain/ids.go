package domain

// Strongly-typed identifiers per DESIGN-0002. Each is a named string
// type so the compiler refuses to pass a WatchID where a ListingID is
// wanted. Concrete generation policy (UUIDv7, ULID, etc.) lives with
// the datastore IMPL.

// WatchID identifies a user-defined query.
type WatchID string

// ListingID identifies one eBay listing snapshot in our store.
type ListingID string

// ComponentID identifies one parsed hardware component on a Listing.
type ComponentID string

// ScoreID identifies a per-listing Score.
type ScoreID string

// JudgmentID identifies one sampled LLM-as-judge verdict.
type JudgmentID string

// AlertID identifies one alert raised against a Watch.
type AlertID string

// NotificationID identifies one delivered notification record.
type NotificationID string

// JobID identifies a scheduler-spawned unit of work (a DAG run).
type JobID string

// TaskID identifies a single stage invocation within a Job.
type TaskID string
