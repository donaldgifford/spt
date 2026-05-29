// Package pipeline implements the DAG orchestrator that walks Jobs from
// trigger through terminal stage. The scheduler holds the per-Job state
// machine; workers dequeue Tasks from Valkey and execute stage handlers.
// See DESIGN-0005 for the executor model.
package pipeline
