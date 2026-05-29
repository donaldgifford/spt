package domain

// JobState is the lifecycle state of a pipeline Job. Values are
// persisted in Postgres as small ints; do not renumber.
type JobState int

// JobState values are persisted as small ints in Postgres.
const (
	JobStatePending JobState = iota
	JobStateRunning
	JobStateSucceeded
	JobStateFailed
	JobStateCancelled
)

// TaskState is the lifecycle state of a single Task within a Job.
type TaskState int

// TaskState values are persisted as small ints in Postgres.
const (
	TaskStatePending TaskState = iota
	TaskStateRunning
	TaskStateSucceeded
	TaskStateFailed
	TaskStateSkipped
)

// JobTrigger records why a Job was created — useful for triage and
// for the scheduler's decision to deduplicate or coalesce Jobs.
type JobTrigger int

// JobTrigger values are persisted as small ints in Postgres.
const (
	JobTriggerCadence JobTrigger = iota
	JobTriggerManual
	JobTriggerBackfill
)
