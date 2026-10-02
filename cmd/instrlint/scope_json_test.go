package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

var scopeFixtures = filepath.Join("..", "..", "internal", "instrlint", "testdata", "scope")

func scopeJSONPaths(t *testing.T, raw string) []string {
	t.Helper()
	var document struct {
		SchemaVersion int `json:"schema_version"`
		Entries       []struct {
			Path   string  `json:"path"`
			Parent *string `json:"parent"`
		} `json:"entries"`
	}
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&document); err != nil || decoder.More() || document.SchemaVersion != 1 || document.Entries == nil {
		t.Fatalf("invalid scope JSON %q: %v", raw, err)
	}
	paths := make([]string, 0, len(document.Entries))
	for _, entry := range document.Entries {
		paths = append(paths, entry.Path)
	}
	return paths
}

func treePaths(tree string) []string {
	var paths []string
	for _, line := range strings.Split(strings.TrimSuffix(tree, "\n"), "\n") {
		paths = append(paths, strings.TrimLeft(line, "├└│─ "))
	}
	return paths
}

func TestCLIScopeFormat_parsing(t *testing.T) {
	root := filepath.Join(scopeFixtures, "root-with-siblings")
	tree := "AGENTS.md\n├── backend/AGENTS.md\n└── frontend/AGENTS.md\n"
	wantJSON := `{"schema_version":1,"entries":[{"path":"AGENTS.md","parent":null},{"path":"backend/AGENTS.md","parent":"AGENTS.md"},{"path":"frontend/AGENTS.md","parent":"AGENTS.md"}]}` + "\n"
	for _, tt := range []struct {
		name string
		args []string
		want string
	}{
		{"default tree", []string{root}, tree},
		{"explicit tree", []string{root, "--format", "tree"}, tree},
		{"json", []string{root, "--format", "json"}, wantJSON},
		{"format before target", []string{"--format", "json", root}, wantJSON},
		{"exclude then format", []string{root, "--exclude", "foo", "--format", "json"}, wantJSON},
		{"format then exclude", []string{root, "--format", "json", "--exclude", "foo"}, wantJSON},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(append([]string{"scope"}, tt.args...), &stdout, &stderr)
			if code != 0 || stderr.Len() != 0 || stdout.String() != tt.want {
				t.Fatalf("code=%d stdout=%q stderr=%q, want %q", code, stdout.String(), stderr.String(), tt.want)
			}
		})
	}
	for _, tt := range []struct {
		name string
		args []string
	}{
		{"missing value", []string{"scope", root, "--format"}},
		{"unknown value", []string{"scope", root, "--format", "yaml"}},
		{"case-sensitive value", []string{"scope", root, "--format", "JSON"}},
		{"empty value", []string{"scope", root, "--format", ""}},
		{"conflicting duplicate", []string{"scope", root, "--format", "json", "--format", "tree"}},
		{"identical duplicate", []string{"scope", root, "--format", "json", "--format", "json"}},
		{"equals syntax", []string{"scope", root, "--format=json"}},
		{"exclude equals syntax", []string{"scope", root, "--exclude=foo", "--format", "json"}},
		{"json alias", []string{"scope", root, "--json"}},
		{"short json alias", []string{"scope", root, "-j"}},
		{"lint directory", []string{root, "--format", "json"}},
		{"lint file", []string{filepath.Join(root, "AGENTS.md"), "--format", "json"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(tt.args, &stdout, &stderr)
			if code != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "usage:") {
				t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
		})
	}
}

func TestCLIScopeFormat_treeMatchesDefaultAndJSONPathSet(t *testing.T) {
	for _, tt := range []struct {
		fixture  string
		excludes []string
	}{
		{"root-only", nil},
		{"root-with-siblings", nil},
		{"deep-nesting", nil},
		{"four-level", nil},
		{"rootless-forest", nil},
		{"multiple-roots", nil},
		{"similar-path-prefix", nil},
		{"excluded-subtree", nil},
		{"excluded-subtree", []string{"--exclude", ".omx"}},
		{"relative-excluded-subtree", nil},
		{"relative-excluded-subtree", []string{"--exclude", "tools/cache"}},
	} {
		t.Run(tt.fixture+strings.Join(tt.excludes, " "), func(t *testing.T) {
			outputs := map[string]string{}
			for name, format := range map[string][]string{"default": nil, "tree": {"--format", "tree"}, "json": {"--format", "json"}} {
				args := append(append([]string{"scope", filepath.Join(scopeFixtures, tt.fixture)}, tt.excludes...), format...)
				var stdout, stderr bytes.Buffer
				if code := run(args, &stdout, &stderr); code != 0 || stderr.Len() != 0 {
					t.Fatalf("args=%q code=%d stderr=%q", args, code, stderr.String())
				}
				outputs[name] = stdout.String()
			}
			if outputs["default"] != outputs["tree"] {
				t.Fatalf("default %q differs from --format tree %q", outputs["default"], outputs["tree"])
			}
			if got, want := strings.Join(scopeJSONPaths(t, outputs["json"]), "\n"), strings.Join(treePaths(outputs["tree"]), "\n"); got != want {
				t.Fatalf("JSON paths %q, tree paths %q", got, want)
			}
		})
	}
}

func TestCLIScopeFormat_emptyDirectory(t *testing.T) {
	for _, tt := range []struct {
		args []string
		want string
	}{
		{nil, "no supported instruction files found\n"},
		{[]string{"--format", "tree"}, "no supported instruction files found\n"},
		{[]string{"--format", "json"}, "{\"schema_version\":1,\"entries\":[]}\n"},
	} {
		var stdout, stderr bytes.Buffer
		code := run(append([]string{"scope", t.TempDir()}, tt.args...), &stdout, &stderr)
		if code != 0 || stderr.Len() != 0 || stdout.String() != tt.want {
			t.Fatalf("args=%q code=%d stdout=%q stderr=%q", tt.args, code, stdout.String(), stderr.String())
		}
	}
}

func TestCLIScopeFormat_jsonIgnoresInvalidUTF8Contents(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte{0xff}, 0644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := run([]string{"scope", root, "--format", "json"}, &stdout, &stderr)
	if code != 0 || stderr.Len() != 0 || stdout.String() != `{"schema_version":1,"entries":[{"path":"AGENTS.md","parent":null}]}`+"\n" {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestCLIScopeFormat_jsonErrorsLeaveStdoutEmpty(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "AGENTS.md")
	if err := os.WriteFile(file, nil, 0644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"scope", file, "--format", "json"},
		{"scope", filepath.Join(root, "missing\nforged"), "--format", "json"},
		{"scope", root, "--exclude", "../outside", "--format", "json"},
	} {
		var stdout, stderr bytes.Buffer
		code := run(args, &stdout, &stderr)
		if code != 2 || stdout.Len() != 0 || strings.Count(stderr.String(), "\n") != 1 || !strings.HasPrefix(stderr.String(), "instrlint: ") {
			t.Fatalf("args=%q code=%d stdout=%q stderr=%q", args, code, stdout.String(), stderr.String())
		}
	}
}

func TestCLIScopeFormat_rejectsInvalidUTF8FilenameInJSON(t *testing.T) {
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		t.Skipf("%s cannot create invalid UTF-8 filenames", runtime.GOOS)
	}
	root := t.TempDir()
	directory := filepath.Join(root, "bad\xff\x1b")
	if err := os.Mkdir(directory, 0755); err != nil {
		t.Skipf("invalid UTF-8 filenames unavailable: %v", err)
	}
	for _, name := range []string{filepath.Join(root, "AGENTS.md"), filepath.Join(directory, "AGENTS.md")} {
		if err := os.WriteFile(name, nil, 0644); err != nil {
			t.Fatal(err)
		}
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"scope", root, "--format", "json"}, &stdout, &stderr)
	if code != 2 || stdout.Len() != 0 || stderr.String() != "instrlint: cannot render scope JSON: path is not valid UTF-8\n" {
		t.Fatalf("json code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = run([]string{"scope", root}, &stdout, &stderr)
	if want := "AGENTS.md\n└── \"bad\\xff\\x1b/AGENTS.md\"\n"; code != 0 || stderr.Len() != 0 || stdout.String() != want {
		t.Fatalf("tree code=%d stdout=%q stderr=%q, want %q", code, stdout.String(), stderr.String(), want)
	}
}

func TestCLIScopeFormat_helpDocumentsScopeOnlyFormat(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"-h"}, {"scope", "--help"}, {"scope", "-h"}} {
		var stdout, stderr bytes.Buffer
		code := run(args, &stdout, &stderr)
		if code != 0 || stderr.Len() != 0 || !strings.Contains(stdout.String(), "--format <tree|json>") ||
			!strings.Contains(stdout.String(), "instrlint scope <directory> [--exclude <directory>]... [--format <tree|json>]") {
			t.Fatalf("args=%q code=%d stdout=%q stderr=%q", args, code, stdout.String(), stderr.String())
		}
	}
	var stdout, stderr bytes.Buffer
	run([]string{"--help"}, &stdout, &stderr)
	if !strings.Contains(stdout.String(), "  instrlint <file-or-directory> [--exclude <directory>]...\n") {
		t.Fatalf("lint usage changed or gained --format: %q", stdout.String())
	}
}
