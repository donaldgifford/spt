package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestRootCommandRegistersSubcommands(t *testing.T) {
	root := NewRootCmd(BuildInfo{Version: "test", Commit: "test", Date: "test"})

	want := []string{"version", "api", "scheduler", "worker", "migrate"}
	for _, name := range want {
		if _, _, err := root.Find([]string{name}); err != nil {
			t.Errorf("subcommand %q not registered: %v", name, err)
		}
	}
}

func TestRootCommandPersistentFlagsDefined(t *testing.T) {
	root := NewRootCmd(BuildInfo{})

	want := []string{"config", "config-dir", "log-format", "log-level", "admin-addr"}
	for _, name := range want {
		if root.PersistentFlags().Lookup(name) == nil {
			t.Errorf("persistent flag --%s not defined", name)
		}
	}
}

func TestRootCommandBareInvocationPrintsHelp(t *testing.T) {
	root := NewRootCmd(BuildInfo{Version: "test"})

	var stdout bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{})

	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	out := stdout.String()
	for _, want := range []string{"Usage:", "Available Commands:", "version"} {
		if !strings.Contains(out, want) {
			t.Errorf("help output missing %q\ngot: %s", want, out)
		}
	}
}

func TestRootCommandLogFormatDefault(t *testing.T) {
	root := NewRootCmd(BuildInfo{})

	flag := root.PersistentFlags().Lookup("log-format")
	if flag == nil {
		t.Fatal("--log-format not defined")
	}
	if flag.DefValue != logFormatAuto {
		t.Errorf("--log-format default: got %q want %q", flag.DefValue, logFormatAuto)
	}
}

func TestRootCommandAdminAddrDefault(t *testing.T) {
	root := NewRootCmd(BuildInfo{})

	flag := root.PersistentFlags().Lookup("admin-addr")
	if flag == nil {
		t.Fatal("--admin-addr not defined")
	}
	if flag.DefValue != ":9090" {
		t.Errorf("--admin-addr default: got %q want :9090", flag.DefValue)
	}
}
