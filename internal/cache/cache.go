package cache

import (
	"context"
	"errors"
	"time"
)

// Sentinel errors implementations return.
var (
	// ErrMiss signals the requested key is absent. Idiomatic Go-style
	// cache miss; callers should branch on errors.Is rather than
	// relying on (value, bool) ok-returns.
	ErrMiss = errors.New("cache: miss")
)

// Cache is a small key/value cache contract. Backed by Valkey in
// production per ADR-0005; the interface lets in-process maps stand
// in for unit tests.
//
// Values are opaque byte slices — callers serialize. Keeping the
// cache type-agnostic avoids reflection at the interface boundary
// and pushes (de)serialization to the call sites that own the type.
type Cache interface {
	// Get returns the value at key, or ErrMiss if absent.
	Get(ctx context.Context, key string) ([]byte, error)

	// Set stores value at key with the given TTL. A zero or negative
	// TTL stores indefinitely.
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error

	// Del removes key; idempotent (no error if absent).
	Del(ctx context.Context, key string) error

	// Expire updates the TTL of an existing key. Returns ErrMiss if
	// the key does not exist.
	Expire(ctx context.Context, key string, ttl time.Duration) error

	// Ping verifies the cache backend is reachable. Used by /readyz.
	Ping(ctx context.Context) error
}
