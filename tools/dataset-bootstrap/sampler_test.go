package main

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/donaldgifford/spt/internal/domain"
)

// fakeReader is a minimal Reader stand-in. Tests construct a population
// and assert sampler behavior against the deterministic seed.
type fakeReader struct {
	listings   []domain.Listing
	components map[domain.ListingID][]domain.Component
	scores     map[domain.ListingID]domain.Score
}

func (f *fakeReader) ListingsSince(_ context.Context, _ time.Duration) ([]domain.Listing, error) {
	return f.listings, nil
}

func (f *fakeReader) ComponentsForListing(_ context.Context, id domain.ListingID) ([]domain.Component, error) {
	return f.components[id], nil
}

func (f *fakeReader) ScoresForListings(_ context.Context, ids []domain.ListingID) (map[domain.ListingID]domain.Score, error) {
	out := make(map[domain.ListingID]domain.Score, len(ids))
	for _, id := range ids {
		if s, ok := f.scores[id]; ok {
			out[id] = s
		}
	}
	return out, nil
}

// buildPopulation returns a fakeReader with 90 listings across 3 kinds
// × 3 buckets × 10 each. Enough to exercise stratification proportions.
func buildPopulation() *fakeReader {
	kinds := []string{"CPU", "RAM", "Drive"}
	buckets := []float64{0.3, 0.6, 0.9}
	f := &fakeReader{
		components: make(map[domain.ListingID][]domain.Component),
		scores:     make(map[domain.ListingID]domain.Score),
	}
	id := 0
	for _, k := range kinds {
		for _, b := range buckets {
			for range 10 {
				id++
				lid := domain.ListingID(jsonItemID(id))
				f.listings = append(f.listings, domain.Listing{
					ID: lid, EbayItemID: jsonItemID(id), Title: k,
				})
				f.components[lid] = []domain.Component{{
					ID: domain.ComponentID(jsonItemID(id)), ListingID: lid,
					Kind: k, Confidence: b, ExtractorVer: "v1",
				}}
				f.scores[lid] = domain.Score{
					ID: domain.ScoreID(jsonItemID(id)), ListingID: lid, Value: float64(id),
				}
			}
		}
	}
	return f
}

func jsonItemID(i int) string {
	return "L" + leftPad(i)
}

func leftPad(i int) string {
	const w = 4
	s := []byte{'0', '0', '0', '0'}
	for j := w - 1; j >= 0 && i > 0; j-- {
		s[j] = byte('0' + i%10)
		i /= 10
	}
	return string(s)
}

func TestSampler_StratificationProportions(t *testing.T) {
	cfg := StratificationConfig{
		SinceDuration: time.Hour,
		PerKind:       5,
		PerConfidenceBucket: map[string]int{
			"<0.5":    2,
			"0.5-0.8": 4,
			"0.8-1.0": 4,
		},
		TotalCap: 200,
		Seed:     42,
	}
	s := NewSampler(cfg, buildPopulation())
	sample, err := s.Run(t.Context())
	require.NoError(t, err)

	// 3 kinds × (2 + 4 + 4) = 30
	require.Len(t, sample.Listings, 30, "stratified picks should total 30")

	byKindBucket := map[string]int{}
	for _, l := range sample.Listings {
		for _, c := range sample.Components[l.ID] {
			key := c.Kind + "/" + ConfidenceBucketFor(c.Confidence)
			byKindBucket[key]++
		}
	}
	// Each (kind, bucket) cell should hit the per-bucket target exactly
	// since population per cell (10) > target.
	for _, kind := range []string{"CPU", "RAM", "Drive"} {
		require.Equal(t, 2, byKindBucket[kind+"/<0.5"])
		require.Equal(t, 4, byKindBucket[kind+"/0.5-0.8"])
		require.Equal(t, 4, byKindBucket[kind+"/0.8-1.0"])
	}
}

func TestSampler_DeterministicSeed(t *testing.T) {
	cfg := StratificationConfig{
		SinceDuration:       time.Hour,
		PerKind:             3,
		PerConfidenceBucket: map[string]int{"<0.5": 1, "0.5-0.8": 1, "0.8-1.0": 1},
		TotalCap:            100,
		Seed:                12345,
	}
	pop1 := buildPopulation()
	pop2 := buildPopulation()

	a, err := NewSampler(cfg, pop1).Run(t.Context())
	require.NoError(t, err)
	b, err := NewSampler(cfg, pop2).Run(t.Context())
	require.NoError(t, err)

	idsA := make([]domain.ListingID, len(a.Listings))
	for i, l := range a.Listings {
		idsA[i] = l.ID
	}
	idsB := make([]domain.ListingID, len(b.Listings))
	for i, l := range b.Listings {
		idsB[i] = l.ID
	}
	require.Equal(t, idsA, idsB, "same seed must produce same listing order")
}

func TestSampler_WriteRoundtrips(t *testing.T) {
	cfg := StratificationConfig{
		SinceDuration:       time.Hour,
		PerKind:             1,
		PerConfidenceBucket: map[string]int{"<0.5": 1, "0.5-0.8": 1, "0.8-1.0": 1},
		TotalCap:            5,
		Seed:                7,
		OutputPath:          "x.json",
	}
	s := NewSampler(cfg, buildPopulation())
	sample, err := s.Run(t.Context())
	require.NoError(t, err)

	var buf bytes.Buffer
	require.NoError(t, s.Write(&buf, sample))

	var envelope map[string]any
	require.NoError(t, json.NewDecoder(&buf).Decode(&envelope))
	require.Equal(t, "v1", envelope["version"])
	require.Contains(t, envelope, "sample")
	require.Contains(t, envelope, "config")
}

func TestSampler_TotalCap(t *testing.T) {
	cfg := StratificationConfig{
		SinceDuration:       time.Hour,
		PerKind:             100,
		PerConfidenceBucket: map[string]int{"<0.5": 100, "0.5-0.8": 100, "0.8-1.0": 100},
		TotalCap:            5,
		Seed:                1,
	}
	s := NewSampler(cfg, buildPopulation())
	sample, err := s.Run(t.Context())
	require.NoError(t, err)
	require.Len(t, sample.Listings, 5, "TotalCap must trim to 5")
}

func TestConfidenceBucketFor(t *testing.T) {
	require.Equal(t, "<0.5", ConfidenceBucketFor(0.1))
	require.Equal(t, "<0.5", ConfidenceBucketFor(0.49))
	require.Equal(t, "0.5-0.8", ConfidenceBucketFor(0.5))
	require.Equal(t, "0.5-0.8", ConfidenceBucketFor(0.79))
	require.Equal(t, "0.8-1.0", ConfidenceBucketFor(0.8))
	require.Equal(t, "0.8-1.0", ConfidenceBucketFor(1.0))
}

func TestParseBuckets(t *testing.T) {
	m, err := parseBuckets("<0.5:5,0.5-0.8:10,0.8-1.0:10")
	require.NoError(t, err)
	require.Equal(t, 5, m["<0.5"])
	require.Equal(t, 10, m["0.5-0.8"])
	require.Equal(t, 10, m["0.8-1.0"])

	_, err = parseBuckets("")
	require.Error(t, err)
	_, err = parseBuckets("bogus")
	require.Error(t, err)
	_, err = parseBuckets("name:abc")
	require.Error(t, err)
	_, err = parseBuckets("name:-1")
	require.Error(t, err)
}
