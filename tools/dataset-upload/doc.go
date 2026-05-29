// Package main hosts the spt-dataset-upload tool.
//
// Uploads regression JSON (produced by spt-dataset-bootstrap) to
// Langfuse as DatasetItems. SHA256-truncated IDs make re-uploads
// idempotent — the second upload of the same content is a no-op,
// not a duplicate.
//
// See DESIGN-0006 "dataset-upload" and IMPL-0002 Phase 4.
package main
