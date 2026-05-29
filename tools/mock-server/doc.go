// Package main hosts the spt-mock-server: an in-memory eBay-shaped HTTP
// server used by unit and integration tests in place of the real eBay
// Browse / Identity / Analytics APIs.
//
// See DESIGN-0006 "mock-server" and IMPL-0002 Phase 1 for the surface
// and the rationale. Scenarios live under fixtures/<name>/ and ship
// embedded in the binary via //go:embed.
package main
