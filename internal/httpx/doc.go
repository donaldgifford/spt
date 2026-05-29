// Package httpx provides small net/http helpers shared across roles —
// middleware, error-to-status mapping, request-ID propagation. Kept
// intentionally thin; we reach for chi only if stdlib forces ugly code.
package httpx
