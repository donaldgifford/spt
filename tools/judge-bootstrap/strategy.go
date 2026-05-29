package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/donaldgifford/spt/internal/domain"
)

// Reader is the minimal Datastore surface judge-bootstrap needs. The
// real datastore.Datastore will satisfy this interface; tests inject
// a fake without satisfying the whole CRUD contract.
type Reader interface {
	ListingsSince(ctx context.Context, since time.Duration) ([]domain.Listing, error)
	ComponentsForListing(ctx context.Context, id domain.ListingID) ([]domain.Component, error)
	ScoresForListings(ctx context.Context, ids []domain.ListingID) (map[domain.ListingID]domain.Score, error)
}

// JudgmentReader exposes prior judgment history for the disagreement
// strategy. Kept separate from Reader so a Datastore that doesn't yet
// store judgments can still satisfy the other strategies.
type JudgmentReader interface {
	JudgmentsForScores(ctx context.Context, ids []domain.ScoreID) (map[domain.ScoreID][]domain.Judgment, error)
}

// SurfaceStrategy is the per-strategy contract. Name is used by the
// --strategy flag.
type SurfaceStrategy interface {
	Name() string
	Surface(ctx context.Context, reader Reader, n int) ([]Candidate, error)
}

// ErrUnknownStrategy is returned when --strategy doesn't match a
// registered strategy name.
var ErrUnknownStrategy = errors.New("judge-bootstrap: unknown strategy")

// strategies is the registry. Keep alphabetical for predictable help
// output.
var strategies = map[string]SurfaceStrategy{
	"ambiguous":      AmbiguousStrategy{Window: 30 * 24 * time.Hour, BandPct: 0.05},
	"disagreement":   DisagreementStrategy{Window: 30 * 24 * time.Hour},
	"high-stakes":    HighStakesStrategy{Window: 30 * 24 * time.Hour},
	"low-confidence": LowConfidenceStrategy{Window: 30 * 24 * time.Hour, Threshold: 0.5},
}

// StrategyByName looks up a strategy by --strategy flag value.
func StrategyByName(name string) (SurfaceStrategy, error) {
	s, ok := strategies[name]
	if !ok {
		want := make([]string, 0, len(strategies))
		for k := range strategies {
			want = append(want, k)
		}
		sort.Strings(want)
		return nil, fmt.Errorf("%w %q (valid: %v)", ErrUnknownStrategy, name, want)
	}
	return s, nil
}

// ---- AmbiguousStrategy ----

// AmbiguousStrategy surfaces Scores within ±BandPct of a percentile
// boundary. The percentile boundaries are the 25/50/75 marks of the
// scored population in the trailing Window.
type AmbiguousStrategy struct {
	Window  time.Duration
	BandPct float64
}

// Name returns the strategy's registry key.
func (AmbiguousStrategy) Name() string { return "ambiguous" }

// Surface returns up to n candidates near a percentile band.
func (s AmbiguousStrategy) Surface(ctx context.Context, reader Reader, n int) ([]Candidate, error) {
	candidates, err := loadAllCandidates(ctx, reader, s.Window)
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		return nil, nil
	}
	values := percentileBoundaries(candidates)

	type scored struct {
		c        *Candidate
		distance float64
	}
	all := make([]scored, len(candidates))
	for i := range candidates {
		all[i] = scored{c: &candidates[i], distance: distanceToNearest(candidates[i].ScoreValue, values)}
	}
	sort.Slice(all, func(i, j int) bool { return all[i].distance < all[j].distance })

	out := make([]Candidate, 0, n)
	for _, sc := range all {
		if len(out) >= n {
			break
		}
		if sc.distance > s.BandPct*100 {
			break
		}
		sc.c.Why = fmt.Sprintf("within %.1f pp of percentile boundary", sc.distance)
		out = append(out, *sc.c)
	}
	return out, nil
}

// ---- LowConfidenceStrategy ----

// LowConfidenceStrategy surfaces Scores whose Listings contain at
// least one Component with Confidence below Threshold.
type LowConfidenceStrategy struct {
	Window    time.Duration
	Threshold float64
}

// Name returns the strategy's registry key.
func (LowConfidenceStrategy) Name() string { return "low-confidence" }

// Surface returns up to n low-confidence candidates.
func (s LowConfidenceStrategy) Surface(ctx context.Context, reader Reader, n int) ([]Candidate, error) {
	candidates, err := loadAllCandidates(ctx, reader, s.Window)
	if err != nil {
		return nil, err
	}
	out := make([]Candidate, 0, n)
	for _, c := range candidates {
		if len(out) >= n {
			break
		}
		minConf := math.Inf(1)
		for _, comp := range c.Components {
			if comp.Confidence < minConf {
				minConf = comp.Confidence
			}
		}
		if minConf < s.Threshold {
			c.Why = fmt.Sprintf("min component confidence=%.2f < %.2f", minConf, s.Threshold)
			out = append(out, c)
		}
	}
	return out, nil
}

// ---- HighStakesStrategy ----

// HighStakesStrategy surfaces Scores in the top decile of Percentile.
type HighStakesStrategy struct {
	Window time.Duration
}

// Name returns the strategy's registry key.
func (HighStakesStrategy) Name() string { return "high-stakes" }

// Surface returns up to n top-decile candidates.
func (s HighStakesStrategy) Surface(ctx context.Context, reader Reader, n int) ([]Candidate, error) {
	candidates, err := loadAllCandidates(ctx, reader, s.Window)
	if err != nil {
		return nil, err
	}
	// Sort descending by ScoreValue, take top decile.
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].ScoreValue > candidates[j].ScoreValue
	})
	cutoff := len(candidates) / 10
	if cutoff < n {
		cutoff = n
	}
	if cutoff > len(candidates) {
		cutoff = len(candidates)
	}
	out := candidates[:cutoff]
	if len(out) > n {
		out = out[:n]
	}
	for i := range out {
		out[i].Why = "top-decile score"
	}
	return out, nil
}

// ---- DisagreementStrategy ----

// DisagreementStrategy surfaces Scores with prior Judgments whose
// Verdict is Disagrees or Uncertain.
type DisagreementStrategy struct {
	Window time.Duration

	// JudgmentReader is optional; when nil the strategy returns an
	// empty list (no prior judgments to filter on).
	JudgmentReader JudgmentReader
}

// Name returns the strategy's registry key.
func (DisagreementStrategy) Name() string { return "disagreement" }

// Surface returns up to n disagreement candidates.
func (s DisagreementStrategy) Surface(ctx context.Context, reader Reader, n int) ([]Candidate, error) {
	if s.JudgmentReader == nil {
		return nil, nil
	}
	candidates, err := loadAllCandidates(ctx, reader, s.Window)
	if err != nil {
		return nil, err
	}
	scoreIDs := make([]domain.ScoreID, len(candidates))
	for i, c := range candidates {
		scoreIDs[i] = c.ScoreID
	}
	judgments, err := s.JudgmentReader.JudgmentsForScores(ctx, scoreIDs)
	if err != nil {
		return nil, fmt.Errorf("judge-bootstrap: judgments lookup: %w", err)
	}
	out := make([]Candidate, 0, n)
	for _, c := range candidates {
		if len(out) >= n {
			break
		}
		for _, j := range judgments[c.ScoreID] {
			if j.Verdict == domain.VerdictDisagrees || j.Verdict == domain.VerdictUncertain {
				c.Why = fmt.Sprintf("prior judgment %s on %s", j.Verdict, j.CreatedAt.Format(time.RFC3339))
				out = append(out, c)
				break
			}
		}
	}
	return out, nil
}

// ---- shared helpers ----

func loadAllCandidates(ctx context.Context, reader Reader, window time.Duration) ([]Candidate, error) {
	listings, err := reader.ListingsSince(ctx, window)
	if err != nil {
		return nil, fmt.Errorf("judge-bootstrap: listings: %w", err)
	}
	ids := make([]domain.ListingID, len(listings))
	for i, l := range listings {
		ids[i] = l.ID
	}
	scores, err := reader.ScoresForListings(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("judge-bootstrap: scores: %w", err)
	}
	out := make([]Candidate, 0, len(listings))
	for _, l := range listings {
		comps, err := reader.ComponentsForListing(ctx, l.ID)
		if err != nil {
			return nil, fmt.Errorf("judge-bootstrap: components for %q: %w", l.ID, err)
		}
		s, ok := scores[l.ID]
		if !ok {
			continue
		}
		out = append(out, Candidate{
			ListingID:  l.ID,
			ScoreID:    s.ID,
			ScoreValue: s.Value,
			Components: comps,
		})
	}
	return out, nil
}

func percentileBoundaries(cs []Candidate) []float64 {
	if len(cs) == 0 {
		return nil
	}
	vals := make([]float64, len(cs))
	for i, c := range cs {
		vals[i] = c.ScoreValue
	}
	sort.Float64s(vals)
	return []float64{
		vals[len(vals)/4],
		vals[len(vals)/2],
		vals[3*len(vals)/4],
	}
}

func distanceToNearest(v float64, marks []float64) float64 {
	best := math.Inf(1)
	for _, m := range marks {
		d := math.Abs(v - m)
		if d < best {
			best = d
		}
	}
	return best
}
