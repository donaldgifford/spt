package main

import (
	"crypto/sha256"
	"encoding/hex"
)

// IDFor produces a deterministic, Langfuse-friendly DatasetItem ID
// from the item's canonical content. The first 8 bytes of SHA-256 are
// hex-encoded (16 hex chars).
//
// Collision math: birthday bound for 2^64 IDs gives a 50% collision
// probability around 2^32 (~4 billion) items in a single dataset. At
// expected scale (≤10^6 items per dataset) the probability is
// ~10^-8 — negligible for an eval dataset.
func IDFor(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:8])
}
