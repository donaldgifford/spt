package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadDefaultsOnly(t *testing.T) {
	cfg, err := Load(nil, "", nil, &FlagOverrides{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Log.Format != "auto" {
		t.Errorf("log.format default: got %q want auto", cfg.Log.Format)
	}
	if cfg.Admin.Addr != ":9090" {
		t.Errorf("admin.addr default: got %q want :9090", cfg.Admin.Addr)
	}
	if cfg.Scheduler.TickInterval != "5s" {
		t.Errorf("scheduler.tick_interval default: got %q want 5s", cfg.Scheduler.TickInterval)
	}
}

func TestLoadSingleFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.hcl")
	writeFile(t, path, `
log {
  format = "json"
  level  = "warn"
}

postgres {
  dsn = "postgres://localhost/spt"
}
`)

	cfg, err := Load([]string{path}, "", nil, &FlagOverrides{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Log.Format != "json" {
		t.Errorf("log.format: got %q want json", cfg.Log.Format)
	}
	if cfg.Log.Level != "warn" {
		t.Errorf("log.level: got %q want warn", cfg.Log.Level)
	}
	if cfg.Postgres.DSN != "postgres://localhost/spt" {
		t.Errorf("postgres.dsn: got %q", cfg.Postgres.DSN)
	}
}

func TestLoadMultiFilePrecedence(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "01-base.hcl")
	second := filepath.Join(dir, "02-override.hcl")

	writeFile(t, first, `
log { format = "json" }
admin { addr = ":9999" }
`)
	writeFile(t, second, `
log { format = "text" }
`)

	cfg, err := Load([]string{first, second}, "", nil, &FlagOverrides{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Log.Format != "text" {
		t.Errorf("log.format: got %q want text (later file wins)", cfg.Log.Format)
	}
	if cfg.Admin.Addr != ":9999" {
		t.Errorf("admin.addr: got %q want :9999 (preserved from earlier file)", cfg.Admin.Addr)
	}
}

func TestLoadConfigDirLexical(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "02-second.hcl"), `log { level = "error" }`)
	writeFile(t, filepath.Join(dir, "01-first.hcl"), `log { level = "debug" }`)
	writeFile(t, filepath.Join(dir, "ignored.txt"), `not hcl`)

	cfg, err := Load(nil, dir, nil, &FlagOverrides{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Log.Level != "error" {
		t.Errorf("log.level: got %q want error (02 sorts after 01)", cfg.Log.Level)
	}
}

func TestLoadConfigDirBeforeExplicit(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "base.hcl"), `log { level = "debug" }`)

	explicit := filepath.Join(t.TempDir(), "explicit.hcl")
	writeFile(t, explicit, `log { level = "error" }`)

	cfg, err := Load([]string{explicit}, dir, nil, &FlagOverrides{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Log.Level != "error" {
		t.Errorf("log.level: got %q want error (explicit --config overrides --config-dir)", cfg.Log.Level)
	}
}

func TestLoadEnvFunctionOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "env.hcl")
	writeFile(t, path, `
ebay {
  app_id  = env("EBAY_APP_ID")
  cert_id = env("EBAY_CERT_ID")
}
`)

	env := map[string]string{
		"EBAY_APP_ID":  "from-env",
		"EBAY_CERT_ID": "cert-from-env",
	}

	cfg, err := Load([]string{path}, "", env, &FlagOverrides{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Ebay.AppID != "from-env" {
		t.Errorf("ebay.app_id: got %q want from-env", cfg.Ebay.AppID)
	}
	if cfg.Ebay.CertID != "cert-from-env" {
		t.Errorf("ebay.cert_id: got %q want cert-from-env", cfg.Ebay.CertID)
	}
}

func TestLoadFlagOverridesEnv(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "env.hcl")
	writeFile(t, path, `
ebay {
  app_id = env("EBAY_APP_ID")
}
`)
	env := map[string]string{"EBAY_APP_ID": "from-env"}

	cfg, err := Load([]string{path}, "", env, &FlagOverrides{EbayAppID: "from-flag"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Ebay.AppID != "from-flag" {
		t.Errorf("ebay.app_id: got %q want from-flag (CLI > env > file)", cfg.Ebay.AppID)
	}
}

func TestLoadEnvMissingReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "env.hcl")
	writeFile(t, path, `
ebay { app_id = env("NEVER_SET_VAR_FOR_TEST") }
`)
	cfg, err := Load([]string{path}, "", map[string]string{}, &FlagOverrides{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Ebay.AppID != "" {
		t.Errorf("ebay.app_id from missing env: got %q want empty", cfg.Ebay.AppID)
	}
}

func TestLoadParseError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.hcl")
	writeFile(t, path, `log { format = "json"`) // unterminated block

	_, err := Load([]string{path}, "", nil, &FlagOverrides{})
	if !errors.Is(err, ErrParse) {
		t.Fatalf("got %v, want ErrParse", err)
	}
	if !strings.Contains(err.Error(), "bad.hcl") {
		t.Errorf("error should mention file path: %v", err)
	}
}

func TestLoadDecodeError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "decode.hcl")
	writeFile(t, path, `
unknown_block "x" {
  field = "value"
}
`)

	_, err := Load([]string{path}, "", nil, &FlagOverrides{})
	if !errors.Is(err, ErrDecode) {
		t.Fatalf("got %v, want ErrDecode", err)
	}
}

func TestLoadValidationError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad-watch.hcl")
	writeFile(t, path, `
watch "no_query" {
  cadence = "15m"
}
`)
	_, err := Load([]string{path}, "", nil, &FlagOverrides{})
	if !errors.Is(err, ErrRequired) {
		t.Fatalf("got %v, want ErrRequired", err)
	}
	if !strings.Contains(err.Error(), `watch["no_query"].query`) {
		t.Errorf("error should mention field path: %v", err)
	}
}

func TestLoadConfigDirMissingFails(t *testing.T) {
	_, err := Load(nil, "/path/that/definitely/does/not/exist", nil, &FlagOverrides{})
	if !errors.Is(err, ErrReadFile) {
		t.Fatalf("got %v, want ErrReadFile", err)
	}
}

func TestLoadExampleConfigParses(t *testing.T) {
	// Walks back up to repo root since tests run in the package dir.
	example, err := filepath.Abs("../../test/config/example.hcl")
	if err != nil {
		t.Fatalf("abs path: %v", err)
	}
	if _, err := os.Stat(example); err != nil {
		t.Skipf("example config not present: %v", err)
	}

	cfg, err := Load([]string{example}, "", nil, &FlagOverrides{})
	if err != nil {
		t.Fatalf("example config failed to load: %v", err)
	}
	if len(cfg.Worker.Pools) != 9 {
		t.Errorf("worker pools: got %d want 9", len(cfg.Worker.Pools))
	}
	if len(cfg.Watches) != 1 {
		t.Errorf("watches: got %d want 1", len(cfg.Watches))
	}
}

func TestEnvSliceToMap(t *testing.T) {
	got := EnvSliceToMap([]string{
		"FOO=bar",
		"EMPTY=",
		"WITH=equals=in=value",
		"malformed",
	})
	want := map[string]string{
		"FOO":   "bar",
		"EMPTY": "",
		"WITH":  "equals=in=value",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("EnvSliceToMap[%q]: got %q want %q", k, got[k], v)
		}
	}
	if _, ok := got["malformed"]; ok {
		t.Error("malformed entry should be skipped")
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
