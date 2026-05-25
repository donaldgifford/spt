package ebay

import (
	"context"
	"time"
)

// Client is the eBay Browse API surface used by the rest of spt. Per
// DESIGN-0003 § "Search and pagination" we deliberately limit the
// surface to Search + GetItem; richer flows (Finding, Trading,
// Inventory, Feed) live in their own packages with their own quota
// tracking when we need them.
//
// Phase 6 declares the contract only; the HTTP client lives in the
// eBay IMPL.
type Client interface {
	// Search runs a Browse-API search query. Pagination semantics are
	// handled by Paginator; raw Search returns the page eBay returned.
	Search(ctx context.Context, req SearchRequest) (SearchResponse, error)

	// GetItem fetches the full /buy/browse/v1/item/{item_id} payload.
	// Returns *ItemStateError wrapping ErrItemNotFound or
	// ErrItemUnavailable when the item is gone or unavailable.
	GetItem(ctx context.Context, ebayItemID string) (Item, error)
}

// SearchRequest is the input shape for Client.Search.
type SearchRequest struct {
	Query       string
	CategoryID  string
	Limit       int
	Offset      int
	Sort        string
	Filters     map[string]string
	Marketplace string
}

// SearchResponse is the result shape for Client.Search.
type SearchResponse struct {
	Items   []ItemSummary
	Total   int
	Offset  int
	Limit   int
	HasMore bool
}

// ItemSummary is the abbreviated shape returned in Search results.
// Field set is a deliberate subset of eBay's response; the eBay IMPL
// will add fields as the orchestrator's stages need them.
type ItemSummary struct {
	EbayItemID string
	Title      string
	Price      ItemPrice
}

// Item is the full /buy/browse/v1/item/{item_id} payload. Richer than
// ItemSummary — includes availability + auction state used by the
// reconciliation flows (DESIGN-0003 § "Search and pagination").
type Item struct {
	ItemID             string
	Title              string
	Price              ItemPrice
	AvailabilityStatus string
	ItemEndDate        string
	BidCount           int
}

// ItemPrice is the price subtree on Item / ItemSummary.
type ItemPrice struct {
	Value    string // raw eBay string; conversion happens in extract
	Currency string
}

// TokenProvider returns a valid eBay OAuth app token. Implementations
// cache + refresh transparently; callers do not need to handle
// expiry. Per DESIGN-0003 § "Auth: app token via OAuth
// client-credentials".
type TokenProvider interface {
	Token(ctx context.Context) (string, error)
}

// RateLimiter is the two-layer (per-second + per-day) limiter Client
// implementations call before issuing a request. Per DESIGN-0003 §
// "Rate-limit tracking". The daily counter lives in Valkey so it
// survives process restarts.
type RateLimiter interface {
	// Wait blocks until a per-second token is available, returning
	// ErrDailyLimitReached if the daily quota is already exhausted.
	Wait(ctx context.Context) error

	// Sync writes authoritative quota state from the Developer
	// Analytics endpoint, overwriting the cached counters.
	Sync(ctx context.Context, count, limit int64, resetAt time.Time) error

	// Snapshot returns the current counter state for observability
	// and admin endpoints.
	Snapshot(ctx context.Context) (Snapshot, error)
}

// Snapshot is the readable view of the RateLimiter's daily counter.
type Snapshot struct {
	Count       int64
	Limit       int64
	Remaining   int64
	WindowStart time.Time
	ResetAt     time.Time
}

// ListingChecker is the "have we seen this listing before?" predicate
// the Paginator calls between pages. Wired by the orchestrator to a
// Datastore.GetListingByEbayItemID lookup. Defined here (not in
// datastore) so the eBay package stays storage-agnostic — per
// DESIGN-0003 § "Search and pagination" rationale.
type ListingChecker func(ctx context.Context, ebayItemID string) (bool, error)
