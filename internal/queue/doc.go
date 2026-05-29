// Package queue declares the Queue interface used by the orchestrator and
// workers, plus the Valkey-backed implementation. Workers dequeue TaskIDs
// via an atomic BLMOVE into a per-worker claimed list with a lease TTL;
// Task payloads live in Postgres. See DESIGN-0005 for the handoff model.
package queue
