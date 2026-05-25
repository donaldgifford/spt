package cli

import (
	"bytes"
	"encoding/json"
	"runtime"
	"strings"
	"testing"
)

func TestVersionTextOutput(t *testing.T) {
	cmd := newVersionCmd(BuildInfo{Version: "v1.2.3", Commit: "abc123", Date: "2026-05-25"})

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	out := stdout.String()
	for _, want := range []string{"v1.2.3", "abc123", "2026-05-25", runtime.Version()} {
		if !strings.Contains(out, want) {
			t.Errorf("text output missing %q\ngot: %s", want, out)
		}
	}
}

func TestVersionJSONOutput(t *testing.T) {
	cmd := newVersionCmd(BuildInfo{Version: "v1.2.3", Commit: "abc123", Date: "2026-05-25"})

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	var payload versionPayload
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode json: %v\nraw: %s", err, stdout.String())
	}

	if payload.Version != "v1.2.3" {
		t.Errorf("version: got %q want v1.2.3", payload.Version)
	}
	if payload.Commit != "abc123" {
		t.Errorf("commit: got %q want abc123", payload.Commit)
	}
	if payload.Date != "2026-05-25" {
		t.Errorf("date: got %q want 2026-05-25", payload.Date)
	}
	if payload.Go != runtime.Version() {
		t.Errorf("go: got %q want %q", payload.Go, runtime.Version())
	}
}
