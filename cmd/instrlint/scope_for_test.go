package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestCLIScopeFor_optionOrdering(t *testing.T) {
	root := filepath.Join(scopeFixtures, "root-with-siblings")
	tree := "AGENTS.md\n└── backend/AGENTS.md\n"
	json := `{"schema_version":1,"entries":[{"path":"AGENTS.md","parent":null},{"path":"backend/AGENTS.md","parent":"AGENTS.md"}]}` + "\n"
	for _, tt := range []struct {
		args []string
		want string
	}{
		{[]string{root, "--for", "backend/new.go"}, tree},
		{[]string{root, "--for", "backend/new.go", "--format", "tree"}, tree},
		{[]string{"--for", "backend/new.go", root, "--format", "json"}, json},
		{[]string{root, "--for", "backend/new.go", "--format", "json"}, json},
		{[]string{root, "--format", "json", "--for", "backend/new.go"}, json},
		{[]string{root, "--exclude", "frontend", "--for", "backend/new.go"}, tree},
		{[]string{root, "--for", "backend/new.go", "--exclude", "frontend"}, tree},
		{[]string{"--format", "json", "--exclude", "frontend", "--for", "backend/new.go", root}, json},
	} {
		for repeat := 0; repeat < 2; repeat++ {
			var stdout, stderr bytes.Buffer
			code := run(append([]string{"scope"}, tt.args...), &stdout, &stderr)
			if code != 0 || stderr.Len() != 0 || stdout.String() != tt.want {
				t.Fatalf("args=%q code=%d stdout=%q stderr=%q want=%q", tt.args, code, stdout.String(), stderr.String(), tt.want)
			}
		}
	}
}

func TestCLIScopeFor_usageErrors(t *testing.T) {
	root := filepath.Join(scopeFixtures, "root-only")
	for _, args := range [][]string{
		{"scope", root, "--for"},
		{"scope", root, "--for", ""},
		{"scope", root, "--for", "--format", "json"},
		{"scope", root, "--for", "--exclude", "backend"},
		{"scope", root, "--for", "-name"},
		{"scope", root, "--for", "a.go", "--for", "b.go"},
		{"scope", root, "--for", "a.go", "--for", "a.go"},
		{"scope", root, "--for=a.go"},
		{"scope", root, "-f", "a.go"},
		{"scope", root, "--for", "a.go", "--unknown"},
		{root, "--for", "a.go"},
		{filepath.Join(root, "AGENTS.md"), "--for", "a.go"},
	} {
		var stdout, stderr bytes.Buffer
		code := run(args, &stdout, &stderr)
		if code != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "usage:") {
			t.Fatalf("args=%q code=%d stdout=%q stderr=%q", args, code, stdout.String(), stderr.String())
		}
	}
}

func TestCLIScopeFor_dashName(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "-name"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "-name", "AGENTS.md"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := run([]string{"scope", root, "--for", "./-name"}, &stdout, &stderr)
	if code != 0 || stderr.Len() != 0 || stdout.String() != "-name/AGENTS.md\n" {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestCLIScopeFor_excludedTargets(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "vendor"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "vendor", "file.go"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{"vendor", "vendor/file.go"} {
		for _, format := range []string{"tree", "json"} {
			t.Run(target+"/"+format, func(t *testing.T) {
				var stdout, stderr bytes.Buffer

				code := run([]string{"scope", root, "--for", target, "--format", format}, &stdout, &stderr)

				if code != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "excluded directory") {
					t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
				}
			})
		}
	}
}

func TestCLIScopeFor_validationErrorsAndSafeStderr(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "vendor"), 0755); err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{root, "../outside", "vendor/new.go", "missing/child.go", "missing/", "missing/.",
		"missing\n/child", "missing\r/child", "missing\x1b/child", "missing\a/child", "missing\x7f/child",
		"missing\u202e/child", "missing\u2066\u2069/child", "missing\u2028/child", "missing\U000e0041/child", "missing\xff/child"} {
		var stdout, stderr bytes.Buffer
		code := run([]string{"scope", root, "--for", target}, &stdout, &stderr)
		message := stderr.String()
		if code != 2 || stdout.Len() != 0 || message == "" || !utf8.ValidString(message) ||
			strings.Count(message, "\n") != 1 || strings.ContainsAny(message, "\r\x1b\a\x7f\u202e\u2066\u2069\u2028\U000e0041") {
			t.Fatalf("target=%q code=%d stdout=%q stderr=%q", target, code, stdout.String(), message)
		}
	}
}

func TestCLIScopeFor_emptyAndContentIndependence(t *testing.T) {
	root := t.TempDir()
	for _, tt := range []struct {
		options []string
		want    string
	}{
		{nil, "no supported instruction files found\n"},
		{[]string{"--for", "."}, "no matching instruction files found\n"},
		{[]string{"--for", "new.go", "--format", "json"}, "{\"schema_version\":1,\"entries\":[]}\n"},
	} {
		var stdout, stderr bytes.Buffer
		code := run(append([]string{"scope", root}, tt.options...), &stdout, &stderr)
		if code != 0 || stdout.String() != tt.want || stderr.Len() != 0 {
			t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
		}
	}
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte{0xff}, 0644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"scope", root, "--for", "AGENTS.md"}, &stdout, &stderr); code != 0 || stdout.String() != "AGENTS.md\n" || stderr.Len() != 0 {
		t.Fatalf("scope code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := run([]string{root}, &stdout, &stderr); code != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "invalid UTF-8") {
		t.Fatalf("lint code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	for _, args := range [][]string{{"--help"}, {"scope", "--help"}} {
		stdout.Reset()
		stderr.Reset()
		if code := run(args, &stdout, &stderr); code != 0 || !strings.Contains(stdout.String(), "[--for <path>]") || !strings.Contains(stdout.String(), "scan-root-relative") {
			t.Fatalf("help code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
		}
	}
}

type scopeForTestWriter struct {
	limit int
	err   error
}

func (w scopeForTestWriter) Write(p []byte) (int, error) {
	return min(w.limit, len(p)), w.err
}

func TestCLIScopeFor_writerFailures(t *testing.T) {
	for _, root := range []string{t.TempDir(), filepath.Join(scopeFixtures, "root-only")} {
		for _, format := range []string{"tree", "json"} {
			for _, writer := range []scopeForTestWriter{{0, errors.New("write failed")}, {5, errors.New("write failed")}, {5, nil}, {0, nil}} {
				var stderr bytes.Buffer
				code := run([]string{"scope", root, "--for", ".", "--format", format}, writer, &stderr)
				if code != 2 || !strings.Contains(stderr.String(), "write output:") {
					t.Fatalf("root=%q format=%s writer=%+v code=%d stderr=%q", root, format, writer, code, stderr.String())
				}
			}
		}
	}
}
