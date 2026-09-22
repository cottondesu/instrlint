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
	if err := os.WriteFile(clean, []byte("Run unit tests.\nRun integration tests.\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(duplicate, []byte("Always run tests.\n- always run tests\n"), 0644); err != nil {
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
