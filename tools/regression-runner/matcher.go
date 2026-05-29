package main

import "github.com/donaldgifford/spt/internal/domain"

// MatchComponents classifies one extraction against the baseline
// truth. ExactMatch requires (Kind, Model, Manufacturer, Quantity,
// Spec) agreement; PartialMatch loosens to (Kind, Model,
// Manufacturer). Since the current placeholder domain.Component has
// only Kind, the matcher degrades gracefully — Model/Manufacturer/etc.
// will be added when the extract IMPL fleshes out the type.
func MatchComponents(got, expected []domain.Component) MatchOutcome {
	if len(got) != len(expected) {
		return NoMatch
	}
	exact := true
	partial := true
	for i := range got {
		g, e := got[i], expected[i]
		if g.Kind != e.Kind {
			partial = false
			exact = false
			continue
		}
		// Once Model/Manufacturer/Quantity/Spec land, downgrade exact
		// to partial when those fields disagree. Today Kind agreement
		// is enough to keep both flags true.
		_ = g
		_ = e
	}
	switch {
	case exact:
		return ExactMatch
	case partial:
		return PartialMatch
	default:
		return NoMatch
	}
}
