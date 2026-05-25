package ebay

import (
	"errors"
	"fmt"
)

// Sentinel errors returned by the eBay client (and its dependencies:
// rate-limiter, token provider). Per DESIGN-0003 § "Search and
// pagination" callers branch on these via errors.Is.
var (
	// ErrItemNotFound — the item is gone (404 from /buy/browse/v1/item).
	ErrItemNotFound = errors.New("ebay: item not found")

	// ErrItemUnavailable — the item exists but availabilityStatus
	// is OUT_OF_STOCK or the listing is ended.
	ErrItemUnavailable = errors.New("ebay: item unavailable")

	// ErrDailyLimitReached — the per-day quota for this resource is
	// exhausted; the rate limiter returns this before issuing the
	// request.
	ErrDailyLimitReached = errors.New("ebay: daily quota exhausted")

	// ErrUnauthorized — 401 from eBay; the cached token has expired
	// or was rejected.
	ErrUnauthorized = errors.New("ebay: unauthorized")

	// ErrRateLimited — per-second token bucket throttled (not the
	// daily quota; that's ErrDailyLimitReached).
	ErrRateLimited = errors.New("ebay: rate limited (per-second)")

	// ErrTransient — retryable 5xx or network error.
	ErrTransient = errors.New("ebay: transient error")
)

// ItemStateError wraps a sentinel error with the raw eBay-side
// state observed at request time. Callers use both:
//
//	errors.Is(err, ebay.ErrItemNotFound)   // branch on the sentinel
//	errors.As(err, &itemErr)               // inspect raw state
//
// Both work simultaneously because Unwrap returns Cause.
type ItemStateError struct {
	ItemID             string
	AvailabilityStatus string
	EndDate            string
	BidCount           int
	HTTPStatus         int
	Cause              error
}

// Error implements the error interface with a single-line summary
// suitable for log streams.
func (e *ItemStateError) Error() string {
	return fmt.Sprintf(
		"ebay: item=%s http=%d availability=%q end=%q bids=%d: %v",
		e.ItemID, e.HTTPStatus, e.AvailabilityStatus, e.EndDate, e.BidCount, e.Cause,
	)
}

// Unwrap exposes the wrapped sentinel so errors.Is works.
func (e *ItemStateError) Unwrap() error { return e.Cause }
