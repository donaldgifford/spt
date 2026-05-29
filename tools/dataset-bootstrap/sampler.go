package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"sort"
	"time"

	"github.com/donaldgifford/spt/internal/domain"
)

// Reader is the minimal Datastore surface Sampler needs. We accept an
// interface rather than the full datastore.Datastore so unit tests
// don't have to satisfy the entire CRUD contract.
type Reader interface {
	ListingsSince(ctx context.Context, since time.Duration) ([]domain.Listing, error)
	ComponentsForListing(ctx context.Context, id domain.ListingID) ([]domain.Component, error)
	ScoresForListings(ctx context.Context, ids []domain.ListingID) (map[domain.ListingID]domain.Score, error)
}

// Sample is the output of one stratified pull. The shape is the
// regression dataset's v1 schema; future changes bump the Version
// field in the JSON header.
type Sample struct {
	Listings   []domain.Listing                        `json:"listings"`
	Scores     map[domain.ListingID]domain.Score       `json:"scores"`
	Components map[domain.ListingID][]domain.Component `json:"components"`
}

// fileEnvelope is the JSON wrapper written to disk. Bumping Version
// signals breaking changes in Sample's shape.
type fileEnvelope struct {
	Version     string               `json:"version"`
	GeneratedAt time.Time            `json:"generatedAt"`
	Config      StratificationConfig `json:"config"`
	Sample      Sample               `json:"sample"`
}

// Sampler is the public entry point used by `sample` and by tests.
type Sampler struct {
	cfg    StratificationConfig
	reader Reader
	rng    *rand.Rand
}

// NewSampler returns a Sampler seeded from cfg.Seed for byte-stable
// output across runs. The PRNG is for stratified sampling only — not
// security-sensitive — so math/rand/v2 is the right choice.
func NewSampler(cfg StratificationConfig, reader Reader) *Sampler {
	seed := uint64(cfg.Seed) //nolint:gosec // sampler seed; not crypto
	src := rand.NewPCG(seed, seed^0x5eed)
	return &Sampler{cfg: cfg, reader: reader, rng: rand.New(src)} //nolint:gosec // ditto
}

// Run executes the stratified pull and returns the assembled Sample.
func (s *Sampler) Run(ctx context.Context) (Sample, error) {
	listings, err := s.reader.ListingsSince(ctx, s.cfg.SinceDuration)
	if err != nil {
		return Sample{}, fmt.Errorf("dataset-bootstrap: list listings: %w", err)
	}

	// Per-listing components map keyed by listing ID. We fetch
	// upfront because the stratification step needs per-listing
	// component metadata.
	componentsByListing := make(map[domain.ListingID][]domain.Component, len(listings))
	for _, l := range listings {
		comps, err := s.reader.ComponentsForListing(ctx, l.ID)
		if err != nil {
			return Sample{}, fmt.Errorf(
				"dataset-bootstrap: components for %q: %w", l.ID, err)
		}
		componentsByListing[l.ID] = comps
	}

	// Group listings by (ComponentKind, ConfidenceBucket, ExtractorVer)
	// using the listing's primary component as the dimension carrier.
	// Listings with no components fall into the special "<no-kind>"
	// kind so the operator can still spot-check them.
	groups := groupByStrata(listings, componentsByListing)

	picked := s.selectFromStrata(groups)
	picked = s.applyTotalCap(picked)

	pickedIDs := make([]domain.ListingID, len(picked))
	for i, l := range picked {
		pickedIDs[i] = l.ID
	}
	scores, err := s.reader.ScoresForListings(ctx, pickedIDs)
	if err != nil {
		return Sample{}, fmt.Errorf("dataset-bootstrap: scores: %w", err)
	}

	pickedComps := make(map[domain.ListingID][]domain.Component, len(picked))
	for _, l := range picked {
		pickedComps[l.ID] = componentsByListing[l.ID]
	}

	return Sample{
		Listings:   picked,
		Scores:     scores,
		Components: pickedComps,
	}, nil
}

// Write writes the Sample envelope to w as deterministic JSON. Used
// by `sample` to write to disk and by tests to write to a buffer.
func (s *Sampler) Write(w io.Writer, sample Sample) error {
	envelope := fileEnvelope{
		Version:     "v1",
		GeneratedAt: time.Now().UTC(),
		Config:      s.cfg,
		Sample:      sample,
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(envelope); err != nil {
		return fmt.Errorf("dataset-bootstrap: encode envelope: %w", err)
	}
	return nil
}

// stratumKey is the per-listing group identifier.
type stratumKey struct {
	Kind         string
	Bucket       string
	ExtractorVer string
}

// groupByStrata maps each listing to its primary component's
// stratification key.
func groupByStrata(
	listings []domain.Listing,
	components map[domain.ListingID][]domain.Component,
) map[stratumKey][]domain.Listing {
	out := make(map[stratumKey][]domain.Listing)
	for _, l := range listings {
		key := stratumKey{Kind: "<no-kind>"}
		if comps := components[l.ID]; len(comps) > 0 {
			c := comps[0]
			key = stratumKey{
				Kind:         c.Kind,
				Bucket:       ConfidenceBucketFor(c.Confidence),
				ExtractorVer: c.ExtractorVer,
			}
		}
		out[key] = append(out[key], l)
	}
	return out
}

// selectFromStrata applies the per-kind and per-bucket targets.
func (s *Sampler) selectFromStrata(
	groups map[stratumKey][]domain.Listing,
) []domain.Listing {
	// Group keys by kind so the per-kind quota slices across buckets.
	byKind := make(map[string][]stratumKey)
	for k := range groups {
		byKind[k.Kind] = append(byKind[k.Kind], k)
	}

	// Stable iteration over byKind by sorting kind names.
	kindNames := make([]string, 0, len(byKind))
	for k := range byKind {
		kindNames = append(kindNames, k)
	}
	sort.Strings(kindNames)

	picked := make([]domain.Listing, 0, len(kindNames)*s.cfg.PerKind)
	for _, kind := range kindNames {
		keys := byKind[kind]
		sort.Slice(keys, func(i, j int) bool {
			if keys[i].Bucket != keys[j].Bucket {
				return keys[i].Bucket < keys[j].Bucket
			}
			return keys[i].ExtractorVer < keys[j].ExtractorVer
		})
		picked = append(picked, s.selectForKind(kind, keys, groups)...)
	}
	// Final stable order so two runs with identical inputs hash the same.
	sort.Slice(picked, func(i, j int) bool {
		return picked[i].ID < picked[j].ID
	})
	return picked
}

func (s *Sampler) selectForKind(
	_ string,
	keys []stratumKey,
	groups map[stratumKey][]domain.Listing,
) []domain.Listing {
	picked := make([]domain.Listing, 0, len(keys)*s.cfg.PerKind)
	for _, k := range keys {
		want := s.cfg.PerConfidenceBucket[k.Bucket]
		if want == 0 {
			want = s.cfg.PerKind
		}
		picked = append(picked, sampleN(groups[k], want, s.rng)...)
	}
	return picked
}

// sampleN returns N elements (or all of them if len(in) < N) via
// Fisher-Yates partial shuffle for unbiased uniform sampling.
func sampleN(in []domain.Listing, n int, rng *rand.Rand) []domain.Listing {
	if n <= 0 || len(in) == 0 {
		return nil
	}
	if n >= len(in) {
		out := make([]domain.Listing, len(in))
		copy(out, in)
		return out
	}
	// Copy first; in-place shuffle would mutate caller state.
	pool := make([]domain.Listing, len(in))
	copy(pool, in)
	for i := range n {
		j := i + rng.IntN(len(pool)-i)
		pool[i], pool[j] = pool[j], pool[i]
	}
	return pool[:n]
}

// applyTotalCap enforces cfg.TotalCap after stratification. We trim
// from the tail rather than re-shuffling so two runs against the same
// input still produce identical output.
func (s *Sampler) applyTotalCap(picked []domain.Listing) []domain.Listing {
	if s.cfg.TotalCap <= 0 || len(picked) <= s.cfg.TotalCap {
		return picked
	}
	return picked[:s.cfg.TotalCap]
}
