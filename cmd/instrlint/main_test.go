package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLIExitCodesAndStreams(t *testing.T) {
	root := t.TempDir()
	clean := filepath.Join(root, "clean.md")
	duplicate := filepath.Join(root, "AGENTS.md")
	conflict := filepath.Join(root, "conflict.md")
	if err := os.WriteFile(clean, []byte("Run unit tests.\nRun integration tests.\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(duplicate, []byte("Always run tests.\n- always run tests\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(conflict, []byte("Always use npm.\nNever use npm.\n"), 0644); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout string
		wantStderr string
	}{
		{"clean file", []string{clean}, 0, "", ""},
		{"duplicate file", []string{duplicate}, 1, "duplicate-instruction", ""},
		{"conflict file", []string{conflict}, 1, "conflicting-instruction", ""},
		{"directory", []string{root}, 1, "duplicate-instruction", ""},
		{"missing path", []string{filepath.Join(root, "missing")}, 2, "", "access"},
		{"invalid usage", []string{"-bad"}, 2, "", "usage"},
		{"help", []string{"--help"}, 0, "Usage:", ""},
		{"empty directory", []string{t.TempDir()}, 0, "no supported instruction files found", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(tt.args, &stdout, &stderr)
			if code != tt.wantCode || !strings.Contains(stdout.String(), tt.wantStdout) || !strings.Contains(stderr.String(), tt.wantStderr) {
				t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
			if tt.wantStdout == "" && stdout.Len() != 0 || tt.wantStderr == "" && stderr.Len() != 0 {
				t.Fatalf("unexpected output: stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
		})
	}
}

func TestCLIExcludesMultipleDirectories(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"AGENTS.md", ".omx/AGENTS.md", ".codegraph/AGENTS.md"} {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("Always run tests.\n- always run tests\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	for _, tt := range []struct {
		name string
		args []string
		want int
	}{
		{"all", []string{root}, 3},
		{"single", []string{root, "--exclude", ".omx"}, 2},
		{"multiple", []string{root, "--exclude", ".omx", "--exclude", ".codegraph"}, 1},
		{"before target", []string{"--exclude", ".omx", root}, 2},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(tt.args, &stdout, &stderr)
			if code != 1 || stderr.Len() != 0 || strings.Count(stdout.String(), "duplicate-instruction") != tt.want {
				t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
		})
	}
}

func TestCLIRejectsInvalidExclude(t *testing.T) {
	for _, exclude := range []string{filepath.Join(t.TempDir(), "absolute"), filepath.Join("..", "outside"), ""} {
		var stdout, stderr bytes.Buffer
		code := run([]string{t.TempDir(), "--exclude", exclude}, &stdout, &stderr)
		if code != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "exclude") {
			t.Fatalf("exclude=%q code=%d stdout=%q stderr=%q", exclude, code, stdout.String(), stderr.String())
		}
	}
}

func TestCLIHelpDescribesExclude(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"--help"}, &stdout, &stderr)
	if code != 0 || stderr.Len() != 0 || !strings.Contains(stdout.String(), "--exclude <directory>") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}
