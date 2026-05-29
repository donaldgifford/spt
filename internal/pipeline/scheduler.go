package pipeline

import (
	"context"
	"errors"

	"github.com/donaldgifford/spt/internal/domain"
)

// Sentinel errors implementations return.
var (
	// ErrJobNotFound is returned when an operation references a Job
	// the scheduler does not know about (or has already purged).
	ErrJobNotFound = errors.New("pipeline: job not found")

	// ErrJobInFlight is returned by TriggerWatch when an existing Job
	// for the same Watch is still Pending or Running and the
	// orchestrator refuses to coalesce a duplicate.
	ErrJobInFlight = errors.New("pipeline: job already in flight")

	// ErrJobTerminal is returned by CancelJob when the Job is already
	// in a terminal state (Succeeded, Failed, Cancelled).
	ErrJobTerminal = errors.New("pipeline: job is terminal")
)

// Scheduler is the orchestrator contract. Per DESIGN-0002 §
// "Scheduler" plus the DESIGN-0005 addition (CancelJob).
//
// The canonical implementation runs as the `spt scheduler` role with
// a Postgres advisory-lock leader (DESIGN-0005 § "Multi-instance
// scaling"); only the lock-holder ticks. Standby schedulers block on
// the lock and report `spt_scheduler_role{role="standby"} = 1`.
type Scheduler interface {
	// Run blocks until ctx is cancelled. It ticks on the configured
	// cadence, queries the Datastore for Watches whose NextRunAt has
	// elapsed, and creates Jobs for each.
	Run(ctx context.Context) error

	// TriggerWatch creates an ad-hoc Job for a Watch outside the
	// normal cadence. Returns ErrJobInFlight when the orchestrator
	// refuses to coalesce a duplicate.
	TriggerWatch(ctx context.Context, id domain.WatchID, trigger domain.JobTrigger) (domain.JobID, error)

	// CancelJob marks a Running Job as Cancelled. In-flight Tasks
	// complete normally but no further edges are traversed.
	// Idempotent on terminal Jobs (returns nil).
	CancelJob(ctx context.Context, id domain.JobID) error
}
