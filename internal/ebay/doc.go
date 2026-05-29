// Package ebay is the only place spt talks to eBay. It handles OAuth
// client-credentials, the Browse search and getItem endpoints, the
// Developer Analytics quota-truth endpoint, Valkey-backed daily-quota
// tracking, and stop-on-known pagination. See DESIGN-0003.
package ebay
