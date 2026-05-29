// Package migrations bundles spt's SQL migrations into the binary via
// embed.FS. `spt migrate up | down | status` reads from FS by default;
// `--migrations-dir` swaps in a real filesystem during local dev so
// in-flight migrations don't need a rebuild to test.
package migrations

import "embed"

// FS holds every numbered migration under this directory. New
// migrations follow the goose timestamp convention:
//
//	YYYYMMDDHHMMSS_<snake_name>.sql
//
// Phase 8 ships only 00001_initial.sql (placeholder); the datastore
// IMPL adds the real DDL with proper timestamps.
//
//go:embed *.sql
var FS embed.FS
