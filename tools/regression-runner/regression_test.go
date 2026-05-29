package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/donaldgifford/spt/internal/domain"
)

func TestMatchComponents(t *testing.T) {
	cpu := domain.Component{Kind: "CPU"}
	ram := domain.Component{Kind: "RAM"}

	require.Equal(t, ExactMatch, MatchComponents(
		[]domain.Component{cpu}, []domain.Component{cpu},
	))
	require.Equal(t, NoMatch, MatchComponents(
		[]domain.Component{cpu}, []domain.Component{ram},
	))
	require.Equal(t, NoMatch, MatchComponents(
		[]domain.Component{cpu, ram}, []domain.Component{cpu},
	))
}

func TestAggregate_AccuracyAndPercentiles(t *testing.T) {
	results := []Result{
		{Backend: "ollama", Outcome: ExactMatch, Latency: 10 * time.Millisecond, Expected: []domain.Component{{Kind: "CPU"}}},
		{Backend: "ollama", Outcome: ExactMatch, Latency: 20 * time.Millisecond, Expected: []domain.Component{{Kind: "CPU"}}},
		{Backend: "ollama", Outcome: NoMatch, Latency: 50 * time.Millisecond, Expected: []domain.Component{{Kind: "RAM"}}},
		{Backend: "ollama", Outcome: PartialMatch, Latency: 30 * time.Millisecond, Expected: []domain.Component{{Kind: "Drive"}}},
	}
	r := Aggregate("ollama", results)
	require.Equal(t, 0.5, r.Accuracy, "2/4 exact matches")
	require.Equal(t, 2, r.Counts.ExactMatch)
	require.Equal(t, 1, r.Counts.PartialMatch)
	require.Equal(t, 1, r.Counts.NoMatch)
	require.InDelta(t, 1.0, r.PerKindAccuracy["CPU"], 0.001)
	require.InDelta(t, 0.0, r.PerKindAccuracy["RAM"], 0.001)
	require.NotZero(t, r.LatencyP50)
	require.NotZero(t, r.LatencyP95)
}

func TestWriteReport_JSONRoundTrip(t *testing.T) {
	report := Report{
		GeneratedAt: time.Now().UTC(),
		Dataset:     "test",
		Backends: []BackendReport{
			{Name: "ollama", Accuracy: 0.9, Counts: OutcomeCounts{ExactMatch: 9, NoMatch: 1}},
		},
	}
	var buf bytes.Buffer
	require.NoError(t, WriteReport(&buf, report, FormatJSON))

	var parsed Report
	require.NoError(t, json.Unmarshal(buf.Bytes(), &parsed))
	require.Equal(t, "test", parsed.Dataset)
	require.Len(t, parsed.Backends, 1)
}

func TestWriteReport_TextSurface(t *testing.T) {
	report := Report{
		GeneratedAt: time.Now().UTC(),
		Dataset:     "test",
		Backends: []BackendReport{
			{
				Name: "ollama", Accuracy: 0.9, Counts: OutcomeCounts{ExactMatch: 9, NoMatch: 1},
				LatencyP50: 10 * time.Millisecond, LatencyP95: 50 * time.Millisecond,
			},
		},
	}
	var buf bytes.Buffer
	require.NoError(t, WriteReport(&buf, report, FormatText))
	require.Contains(t, buf.String(), "BACKEND")
	require.Contains(t, buf.String(), "ollama")
}

func TestLoadDataset_LocalDir(t *testing.T) {
	entries, err := LoadDataset(filepath.Join("testdata", "baseline"))
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(entries), 2, "baseline ships with 2+ entries")
}

func TestLoadDataset_LangfuseURLNotWired(t *testing.T) {
	_, err := LoadDataset("langfuse://my-dataset")
	require.ErrorIs(t, err, ErrLangfuseDatasetNotWired)
}

func TestResolveBackends(t *testing.T) {
	got, err := resolveBackends("ollama,anthropic")
	require.NoError(t, err)
	require.Len(t, got, 2)
	require.Equal(t, "ollama", got[0].Name())

	_, err = resolveBackends("not-a-backend")
	require.Error(t, err)
}

// fakeBackend is used by TestExecuteAll to assert the runner correctly
// translates backend output into Results.
type fakeBackend struct {
	name string
	out  []domain.Component
	err  error
}

func (f *fakeBackend) Name() string { return f.name }

func (f *fakeBackend) Extract(_ context.Context, l domain.Listing) ([]domain.Component, error) {
	if f.err != nil {
		return nil, f.err
	}
	out := make([]domain.Component, len(f.out))
	copy(out, f.out)
	for i := range out {
		out[i].ListingID = l.ID
	}
	return out, nil
}

func TestExecuteAll_PerBackendReport(t *testing.T) {
	entries := []DatasetEntry{
		{Listing: domain.Listing{ID: "L1"}, Expected: []domain.Component{{Kind: "CPU"}}},
		{Listing: domain.Listing{ID: "L2"}, Expected: []domain.Component{{Kind: "CPU"}}},
	}
	backends := []Backend{
		&fakeBackend{name: "a", out: []domain.Component{{Kind: "CPU"}}},
		&fakeBackend{name: "b", err: errors.New("backend down")},
	}
	report := executeAll(t.Context(), "synthetic", entries, backends)
	require.Len(t, report.Backends, 2)
	require.Equal(t, 1.0, report.Backends[0].Accuracy, "perfect backend")
	require.Equal(t, 0.0, report.Backends[1].Accuracy, "failing backend gives 0 accuracy")
}

func TestAntiCIComment_IsPreservedInDoc(t *testing.T) {
	// The doc.go file must mention the anti-CI rationale (key
	// exposure to fork PRs) so contributors don't try to wire this
	// tool into a workflow.
	data, err := stringFromTestFile(t, "doc.go")
	require.NoError(t, err)
	require.True(t, strings.Contains(strings.ToLower(data), "do not wire") ||
		strings.Contains(strings.ToLower(data), "do not wire this"),
		"doc.go must include the anti-CI directive")
	require.Contains(t, strings.ToLower(data), "anthropic_api_key",
		"doc.go must spell out the key-exposure rationale")
}

// stringFromTestFile is a t-test helper avoiding os.ReadFile in test
// to keep imports minimal.
func stringFromTestFile(t *testing.T, name string) (string, error) {
	t.Helper()
	return readFile(name)
}
