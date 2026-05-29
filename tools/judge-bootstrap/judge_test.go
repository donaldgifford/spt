package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/donaldgifford/spt/internal/domain"
)

// fakeReader satisfies Reader + JudgmentReader for strategy tests.
type fakeReader struct {
	listings   []domain.Listing
	components map[domain.ListingID][]domain.Component
	scores     map[domain.ListingID]domain.Score
	judgments  map[domain.ScoreID][]domain.Judgment
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

func (f *fakeReader) JudgmentsForScores(_ context.Context, ids []domain.ScoreID) (map[domain.ScoreID][]domain.Judgment, error) {
	out := make(map[domain.ScoreID][]domain.Judgment, len(ids))
	for _, id := range ids {
		if js, ok := f.judgments[id]; ok {
			out[id] = js
		}
	}
	return out, nil
}

// populate20 returns a fakeReader with 20 listings, scored 1..20, one
// component each. Confidence ranges from 0.05 to 1.0 in 0.05 steps so
// half are below 0.5.
func populate20() *fakeReader {
	f := &fakeReader{
		components: make(map[domain.ListingID][]domain.Component),
		scores:     make(map[domain.ListingID]domain.Score),
		judgments:  make(map[domain.ScoreID][]domain.Judgment),
	}
	for i := 1; i <= 20; i++ {
		lid := domain.ListingID(jsonItemID(i))
		sid := domain.ScoreID(jsonItemID(i))
		f.listings = append(f.listings, domain.Listing{ID: lid, EbayItemID: jsonItemID(i)})
		f.components[lid] = []domain.Component{{
			ID: domain.ComponentID(jsonItemID(i)), ListingID: lid,
			Kind: "CPU", Confidence: float64(i) * 0.05,
		}}
		f.scores[lid] = domain.Score{ID: sid, ListingID: lid, Value: float64(i)}
	}
	return f
}

func jsonItemID(i int) string {
	const w = 3
	s := []byte{'L', '0', '0', '0'}
	for j := w; j >= 1 && i > 0; j-- {
		s[j] = byte('0' + i%10)
		i /= 10
	}
	return string(s)
}

func TestLowConfidenceStrategy_SurfacesBelowThreshold(t *testing.T) {
	s := LowConfidenceStrategy{Window: time.Hour, Threshold: 0.5}
	got, err := s.Surface(t.Context(), populate20(), 100)
	require.NoError(t, err)
	// Confidences 0.05..0.45 are below 0.5 → 9 listings (0.5 itself is excluded).
	require.Len(t, got, 9)
}

func TestHighStakesStrategy_TopDecile(t *testing.T) {
	s := HighStakesStrategy{Window: time.Hour}
	got, err := s.Surface(t.Context(), populate20(), 5)
	require.NoError(t, err)
	require.LessOrEqual(t, len(got), 5)
	require.Equal(t, float64(20), got[0].ScoreValue, "top candidate must be the max ScoreValue")
}

func TestAmbiguousStrategy_NearPercentileBoundary(t *testing.T) {
	s := AmbiguousStrategy{Window: time.Hour, BandPct: 1.0}
	got, err := s.Surface(t.Context(), populate20(), 3)
	require.NoError(t, err)
	require.NotEmpty(t, got)
	require.LessOrEqual(t, len(got), 3)
}

func TestDisagreementStrategy_NeedsJudgmentReader(t *testing.T) {
	pop := populate20()
	pop.judgments[domain.ScoreID("L001")] = []domain.Judgment{
		{ID: "J001", ScoreID: "L001", Verdict: domain.VerdictDisagrees, CreatedAt: time.Now()},
	}
	s := DisagreementStrategy{Window: time.Hour, JudgmentReader: pop}
	got, err := s.Surface(t.Context(), pop, 10)
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, domain.ScoreID("L001"), got[0].ScoreID)
}

func TestDisagreementStrategy_NoJudgmentReaderReturnsEmpty(t *testing.T) {
	s := DisagreementStrategy{Window: time.Hour}
	got, err := s.Surface(t.Context(), populate20(), 10)
	require.NoError(t, err)
	require.Empty(t, got)
}

func TestStrategyByName_UnknownErrors(t *testing.T) {
	_, err := StrategyByName("nope")
	require.ErrorIs(t, err, ErrUnknownStrategy)
}

func TestStrategyByName_AllKnownResolve(t *testing.T) {
	for _, name := range []string{"ambiguous", "low-confidence", "high-stakes", "disagreement"} {
		s, err := StrategyByName(name)
		require.NoError(t, err, name)
		require.Equal(t, name, s.Name())
	}
}

func TestFilterAccepted_RequiresNotes(t *testing.T) {
	_, err := FilterAccepted([]Candidate{
		{ScoreID: "S1", Accepted: true, Notes: "ok"},
		{ScoreID: "S2", Accepted: true}, // missing Notes
		{ScoreID: "S3", Accepted: false},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "S2")
}

func TestFilterAccepted_PicksAcceptedOnly(t *testing.T) {
	out, err := FilterAccepted([]Candidate{
		{ScoreID: "S1", Accepted: true, Notes: "good"},
		{ScoreID: "S2", Accepted: false},
		{ScoreID: "S3", Accepted: true, Notes: "good"},
	})
	require.NoError(t, err)
	require.Len(t, out, 2)
}

func TestApply_RoundtripsExamplesJSON(t *testing.T) {
	candidates := []Candidate{
		{ScoreID: "S1", Accepted: true, Notes: "matches", ScoreValue: 1.0},
		{ScoreID: "S2", Accepted: false},
	}
	in, err := json.Marshal(candidates)
	require.NoError(t, err)

	dir := t.TempDir()
	inPath := filepath.Join(dir, "candidates.json")
	outPath := filepath.Join(dir, "examples.json")
	require.NoError(t, os.WriteFile(inPath, in, 0o600))

	cmd := newApplyCmd()
	cmd.SetArgs([]string{"--input=" + inPath, "--output=" + outPath})
	require.NoError(t, cmd.Execute())

	got, err := os.ReadFile(outPath)
	require.NoError(t, err)
	var parsed []Candidate
	require.NoError(t, json.Unmarshal(got, &parsed))
	require.Len(t, parsed, 1)
	require.Equal(t, domain.ScoreID("S1"), parsed[0].ScoreID)
}
