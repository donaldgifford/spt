package queue

import (
	"context"
	"errors"
	"time"

	"github.com/donaldgifford/spt/internal/domain"
)

// Sentinel errors implementations return. Callers use errors.Is to
// branch on these without importing the concrete backend.
var (
	// ErrQueueClosed indicates the queue has been closed and no
	// further Enqueue/Dequeue calls will succeed.
	ErrQueueClosed = errors.New("queue: closed")

	// ErrTaskNotFound indicates Ack/Nack was called with a TaskID the
	// queue does not have a claim for (already acked, or never
	// claimed by this worker).
	ErrTaskNotFound = errors.New("queue: task not found")
)

// Queue is the per-stage Task queue contract. Per DESIGN-0002 §
// "Queue" plus the DESIGN-0005 additions (QueueDepth, Subscribe).
//
// The canonical implementation is Valkey-backed: each Stage is a
// Valkey LIST, Dequeue performs BLMOVE into a per-worker claim list
// with a lease TTL, and Ack/Nack manage the claim list. Task payloads
// live in Postgres — Queue messages are just IDs (DESIGN-0005 §
// "Stage handoff").
type Queue interface {
	// Enqueue appends task to its stage's queue. The Task's Stage and
	// ID must be set; everything else is ignored by Enqueue.
	Enqueue(ctx context.Context, task domain.Task) error

	// Dequeue blocks until a Task is available on any of the given
	// stages or ctx is cancelled. Returns the claimed Task; the caller
	// is responsible for Ack/Nack within the lease TTL.
	Dequeue(ctx context.Context, stages []domain.Stage) (domain.Task, error)

	// Ack releases the claim on id, marking it complete.
	Ack(ctx context.Context, id domain.TaskID) error

	// Nack returns id to its stage queue after retryAfter has elapsed.
	// Used when a handler fails but the Task should be retried.
	Nack(ctx context.Context, id domain.TaskID, retryAfter time.Duration) error

	// QueueDepth returns the current pending-Task count for stage.
	// Drives the spt_worker_pool_queue_depth gauge (DESIGN-0005).
	QueueDepth(ctx context.Context, stage domain.Stage) (int64, error)

	// Subscribe returns a channel of QueueEvent values for channel
	// (e.g., "spt:pipeline:task_complete"). Closing ctx terminates
	// the subscription and closes the returned channel.
	Subscribe(ctx context.Context, channel string) (<-chan QueueEvent, error)

	// Ping verifies the underlying queue is reachable. Used by
	// /readyz probes.
	Ping(ctx context.Context) error
}

// QueueEvent is the message broadcast on Subscribe channels by the
// task-completion publisher. The scheduler's DAG walker consumes
// these to evaluate edges and enqueue downstream Tasks.
type QueueEvent struct {
	JobID  domain.JobID
	TaskID domain.TaskID
	Stage  domain.Stage
	State  domain.TaskState
}
