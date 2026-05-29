package main

import (
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"
)

func TestScenarioRegistry_Resolve(t *testing.T) {
	// Build a synthetic two-scenario tree: default has two items,
	// sold-listings overrides one of them. Resolve("sold", overridden)
	// must hit the override, Resolve("sold", default-only) must fall
	// back to default, and Resolve("sold", "missing") must miss both.
	fsys := fstest.MapFS{
		"default/items/v1%7Cdefault-only.json": {Data: []byte(`{"itemId":"default-only"}`)},
		"default/items/v1%7Cshared.json":       {Data: []byte(`{"itemId":"shared","availabilityStatus":"IN_STOCK"}`)},
		"sold/items/v1%7Cshared.json":          {Data: []byte(`{"itemId":"shared","availabilityStatus":"OUT_OF_STOCK"}`)},
	}

	reg, err := LoadScenarios(fsys, ".")
	require.NoError(t, err)
	require.Len(t, reg.scenarios, 2)

	// Active scenario hit.
	body, ok := reg.Resolve("sold", "v1|shared")
	require.True(t, ok)
	require.Contains(t, string(body), "OUT_OF_STOCK")

	// Fallback to default.
	body, ok = reg.Resolve("sold", "v1|default-only")
	require.True(t, ok)
	require.Contains(t, string(body), "default-only")

	// Double miss.
	_, ok = reg.Resolve("sold", "v1|never")
	require.False(t, ok)
}

func TestScenarioRegistry_MissingDefault(t *testing.T) {
	fsys := fstest.MapFS{
		"sold/items/v1%7Ca.json": {Data: []byte(`{"itemId":"a"}`)},
	}
	_, err := LoadScenarios(fsys, ".")
	require.ErrorIs(t, err, ErrFixturesNotFound)
}

func TestScenarioRegistry_SearchTemplateFallback(t *testing.T) {
	// Only default has a search.json; "sold" should fall back to it.
	fsys := fstest.MapFS{
		"default/search.json":       {Data: []byte(`{"itemSummaries":[{"itemId":"a","title":"foo"}]}`)},
		"default/items/v1%7Ca.json": {Data: []byte(`{"itemId":"a"}`)},
		"sold/items/v1%7Ca.json":    {Data: []byte(`{"itemId":"a","availabilityStatus":"OUT_OF_STOCK"}`)},
	}
	reg, err := LoadScenarios(fsys, ".")
	require.NoError(t, err)

	tmpl := reg.SearchTemplate("sold")
	require.NotNil(t, tmpl)
	require.Len(t, tmpl.items, 1)
	require.Equal(t, "foo", tmpl.items[0].title)
}
