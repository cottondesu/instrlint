package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLIScope_rendersFixtureAndExcludes(t *testing.T) {
	fixture := filepath.Join("..", "..", "internal", "instrlint", "testdata", "scope")
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"default", []string{filepath.Join(fixture, "root-with-siblings")}, "AGENTS.md\n├── backend/AGENTS.md\n└── frontend/AGENTS.md\n"},
		{"name exclude", []string{filepath.Join(fixture, "excluded-subtree"), "--exclude", ".omx"}, "AGENTS.md\n└── src/AGENTS.md\n"},
		{"unmatched exclude", []string{filepath.Join(fixture, "root-only"), "--exclude", "foo"}, "AGENTS.md\n"},
		{"relative exclude", []string{filepath.Join(fixture, "relative-excluded-subtree"), "--exclude", "tools/cache"}, "AGENTS.md\n└── tools/source/AGENTS.md\n"},
		{"multiple excludes", []string{filepath.Join(fixture, "excluded-subtree"), "--exclude", ".omx", "--exclude", "src"}, "AGENTS.md\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			args := append([]string{"scope"}, tt.args...)
			code := run(args, &stdout, &stderr)
			if code != 0 || stderr.Len() != 0 || stdout.String() != tt.want {
				t.Fatalf("code=%d stdout=%q stderr=%q, want %q", code, stdout.String(), stderr.String(), tt.want)
			}
		})
	}
}

func TestCLIScope_helpAndUsage(t *testing.T) {
	for _, args := range [][]string{{"scope", "--help"}, {"scope", "-h"}} {
		var stdout, stderr bytes.Buffer
		code := run(args, &stdout, &stderr)
		if code != 0 || stderr.Len() != 0 || !strings.Contains(stdout.String(), "instrlint scope <directory>") ||
			!strings.Contains(stdout.String(), "does not lint, merge, or interpret instructions") {
			t.Fatalf("args=%q code=%d stdout=%q stderr=%q", args, code, stdout.String(), stderr.String())
		}
	}
	for _, args := range [][]string{{"scope"}, {"scope", "a", "b"}, {"scope", "--unknown"}, {"scope", "--exclude"}} {
		var stdout, stderr bytes.Buffer
		code := run(args, &stdout, &stderr)
		if code != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "usage:") {
			t.Fatalf("args=%q code=%d stdout=%q stderr=%q", args, code, stdout.String(), stderr.String())
		}
	}
}

func TestCLIScope_rejectsFileMissingPathAndInvalidExclude(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "AGENTS.md")
	if err := os.WriteFile(file, nil, 0644); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name string
		args []string
		want string
	}{
		{"file", []string{"scope", file}, "scope target must be a directory"},
		{"missing", []string{"scope", filepath.Join(root, "missing")}, "access"},
		{"absolute exclude", []string{"scope", root, "--exclude", root}, "invalid exclude"},
		{"escaping exclude", []string{"scope", root, "--exclude", "../outside"}, "invalid exclude"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(tt.args, &stdout, &stderr)
			if code != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), tt.want) {
				t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
		})
	}
}

func TestCLIScope_escapesControlCharactersInErrors(t *testing.T) {
	for _, tt := range []struct{ label, name string }{
		{"newline", "missing\nforged"},
		{"carriage return", "carriage\rreturn"},
		{"terminal escape", "esc\x1b]0;title\a"},
		{"bidirectional control", "bidi\u202e"},
	} {
		t.Run(tt.label, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), tt.name)
			var stdout, stderr bytes.Buffer
			code := run([]string{"scope", path}, &stdout, &stderr)
			if code != 2 || stdout.Len() != 0 || strings.Count(stderr.String(), "\n") != 1 ||
				strings.ContainsAny(stderr.String(), "\r\x1b\a\u202e") {
				t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
		})
	}
}

func TestCLIScope_ignoresFileContents(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte{0xff}, 0644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"scope", root}, &stdout, &stderr); code != 0 || stdout.String() != "AGENTS.md\n" || stderr.Len() != 0 {
		t.Fatalf("scope code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := run([]string{root}, &stdout, &stderr); code != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "invalid UTF-8") {
		t.Fatalf("lint code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestCLIScope_reservesOnlyBareCommandName(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "scope"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "scope", "AGENTS.md"), []byte("Always run tests.\n- always run tests\n"), 0644); err != nil {
		t.Fatal(err)
	}
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previous); err != nil {
			t.Error(err)
		}
	})
	var stdout, stderr bytes.Buffer
	code := run([]string{"./scope"}, &stdout, &stderr)
	if code != 1 || stderr.Len() != 0 || !strings.Contains(stdout.String(), "duplicate-instruction") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestCLIScope_reportsEmptyDirectory(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"scope", t.TempDir()}, &stdout, &stderr)
	if code != 0 || stderr.Len() != 0 || stdout.String() != "no supported instruction files found\n" {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestCLIHelp_preservesLintUsageAndListsScope(t *testing.T) {
	for _, arg := range []string{"--help", "-h"} {
		var stdout, stderr bytes.Buffer
		code := run([]string{arg}, &stdout, &stderr)
		if code != 0 || stderr.Len() != 0 || !strings.Contains(stdout.String(), "instrlint <file-or-directory>") ||
			!strings.Contains(stdout.String(), "instrlint scope <directory>") {
			t.Fatalf("arg=%s code=%d stdout=%q stderr=%q", arg, code, stdout.String(), stderr.String())
		}
	}
}
