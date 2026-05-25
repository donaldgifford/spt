// Package datastore declares the Datastore interface — the persistence
// boundary for every domain type — and the Postgres-backed implementation
// over pgx. SQL migrations live under migrations/ and are embedded into
// the binary. See DESIGN-0002 for the interface surface.
package datastore
