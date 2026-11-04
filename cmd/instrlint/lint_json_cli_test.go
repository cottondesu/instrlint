package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// writeLintFixture creates a scan root whose lint findings cover duplicates,
// conflicts, exclusions, and Japanese text. It returns the root directory.
func writeLintFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"AGENTS.md":         "Always run tests.\n- ALWAYS RUN TESTS\nAlways use npm.\nNever use npm.\n",
		"omx/AGENTS.md":     "Always run tests.\n- always run tests\n",
		"sub/AGENTS.md":     "必ずテストを実行すること\n必ずテストを実行すること。\n",
		"clean/AGENTS.md":   "Run unit tests.\nRun integration tests.\n",
		"empty/placeholder": "",
	}
	for name, content := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestCLIDefaultTextMatchesGolden(t *testing.T) {
	root := writeLintFixture(t)
	want := strings.Join([]string{
		filepath.Join(root, "AGENTS.md") + ":2:1: warning duplicate-instruction: Duplicate instruction: ALWAYS RUN TESTS (first at " + filepath.Join(root, "AGENTS.md") + ":1)",
		filepath.Join(root, "AGENTS.md") + ":4:1: warning conflicting-instruction: Conflicting instruction: Never use npm. (conflicts with " + filepath.Join(root, "AGENTS.md") + ":3)",
		filepath.Join(root, "omx", "AGENTS.md") + ":2:1: warning duplicate-instruction: Duplicate instruction: always run tests (first at " + filepath.Join(root, "omx", "AGENTS.md") + ":1)",
		filepath.Join(root, "sub", "AGENTS.md") + ":2:1: warning duplicate-instruction: Duplicate instruction: 必ずテストを実行すること。 (first at " + filepath.Join(root, "sub", "AGENTS.md") + ":1)",
		"",
		"4 warnings",
		"",
	}, "\n")
	var stdout, stderr bytes.Buffer
	code := run([]string{root}, &stdout, &stderr)
	if code != 1 || stderr.Len() != 0 || stdout.String() != want {
		t.Fatalf("code=%d stdout=%q stderr=%q, want %q", code, stdout.String(), stderr.String(), want)
	}
}

func TestCLITextFormatExplicitMatchesDefault(t *testing.T) {
	root := writeLintFixture(t)
	var defaultOut, explicitOut, stderr bytes.Buffer
	defaultCode := run([]string{root}, &defaultOut, &stderr)
	explicitCode := run([]string{root, "--format", "text"}, &explicitOut, &stderr)
	if defaultCode != explicitCode || defaultOut.String() != explicitOut.String() || stderr.Len() != 0 {
		t.Fatalf("default code=%d out=%q, text code=%d out=%q", defaultCode, defaultOut.String(), explicitCode, explicitOut.String())
	}
}

type cliLintDiagnostic struct {
	Path     string `json:"path"`
	Line     int    `json:"line"`
	Column   int    `json:"column"`
	Severity string `json:"severity"`
	Rule     string `json:"rule"`
	Message  string `json:"message"`
	Related  *struct {
		Path   string `json:"path"`
		Line   int    `json:"line"`
		Column int    `json:"column"`
	} `json:"related"`
}

// assertCLIJSONSafe strictly decodes lint JSON output and asserts the
// single-line, schema-version-1 contract.
func assertCLIJSONSafe(t *testing.T, raw []byte) []cliLintDiagnostic {
	t.Helper()
	if !bytes.HasSuffix(raw, []byte("\n")) || bytes.Count(raw, []byte("\n")) != 1 {
		t.Fatalf("JSON output must be a single line: %q", raw)
	}
	var document struct {
		SchemaVersion *int                `json:"schema_version"`
		Diagnostics   []cliLintDiagnostic `json:"diagnostics"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&document); err != nil {
		t.Fatalf("decode %q: %v", raw, err)
	}
	if decoder.More() || document.SchemaVersion == nil || *document.SchemaVersion != 1 || document.Diagnostics == nil {
		t.Fatalf("invalid lint JSON %q", raw)
	}
	return document.Diagnostics
}

func TestCLIJSONFormat(t *testing.T) {
	root := writeLintFixture(t)
	file := filepath.Join(root, "AGENTS.md")
	cases := []struct {
		name      string
		args      []string
		wantCount int
		wantOmx   bool
		wantRules []string
	}{
		{"json after path", []string{root, "--format", "json"}, 4, true, nil},
		{"json before path", []string{"--format", "json", root}, 4, true, nil},
		{"exclude then json", []string{root, "--exclude", "omx", "--format", "json"}, 3, false, nil},
		{"exclude after json", []string{root, "--format", "json", "--exclude", "omx"}, 3, false, nil},
		{"single file json", []string{file, "--format", "json"}, 2, false,
			[]string{"duplicate-instruction", "conflicting-instruction"}},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(tt.args, &stdout, &stderr)
			if code != 1 || stderr.Len() != 0 {
				t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
			diagnostics := assertCLIJSONSafe(t, stdout.Bytes())
			if len(diagnostics) != tt.wantCount {
				t.Fatalf("diagnostics = %+v from %q", diagnostics, stdout.String())
			}
			omxPrefix := filepath.Join(root, "omx") + string(filepath.Separator)
			foundOmx := false
			for _, diagnostic := range diagnostics {
				if diagnostic.Rule != "duplicate-instruction" && diagnostic.Rule != "conflicting-instruction" {
					t.Fatalf("unexpected rule %q in %q", diagnostic.Rule, stdout.String())
				}
				if strings.HasPrefix(diagnostic.Path, omxPrefix) {
					foundOmx = true
					if !tt.wantOmx {
						t.Fatalf("excluded directory still reported: %q", stdout.String())
					}
				}
				if diagnostic.Line == 0 || diagnostic.Column == 0 {
					t.Fatalf("missing location in %q", stdout.String())
				}
			}
			if foundOmx != tt.wantOmx {
				t.Fatalf("omx presence = %v, want %v; diagnostics = %+v", foundOmx, tt.wantOmx, diagnostics)
			}
			if tt.wantRules != nil {
				for i, rule := range tt.wantRules {
					if diagnostics[i].Rule != rule || diagnostics[i].Path != file {
						t.Fatalf("diagnostics[%d] = %+v, want rule %q at %q", i, diagnostics[i], rule, file)
					}
				}
			}
		})
	}
}

// jsonQuote encodes a string exactly as the JSON document should embed it.
func jsonQuote(t *testing.T, value string) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

// TestCLIJSONExactBytesRelativePaths pins the exact JSON document for a
// relative scan root. It changes the process working directory, so no test
// in this package may use t.Parallel while this test exists.
func TestCLIJSONExactBytesRelativePaths(t *testing.T) {
	root := writeLintFixture(t)
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(previous); err != nil {
			t.Fatal(err)
		}
	}()
	var stdout, stderr bytes.Buffer
	code := run([]string{".", "--format", "json"}, &stdout, &stderr)
	if code != 1 || stderr.Len() != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	want := `{"schema_version":1,"diagnostics":[` +
		`{"path":` + jsonQuote(t, "AGENTS.md") + `,"line":2,"column":1,"severity":"warning","rule":"duplicate-instruction","message":"Duplicate instruction: ALWAYS RUN TESTS","related":{"path":` + jsonQuote(t, "AGENTS.md") + `,"line":1,"column":1}},` +
		`{"path":` + jsonQuote(t, "AGENTS.md") + `,"line":4,"column":1,"severity":"warning","rule":"conflicting-instruction","message":"Conflicting instruction: Never use npm.","related":{"path":` + jsonQuote(t, "AGENTS.md") + `,"line":3,"column":1}},` +
		`{"path":` + jsonQuote(t, filepath.FromSlash("omx/AGENTS.md")) + `,"line":2,"column":1,"severity":"warning","rule":"duplicate-instruction","message":"Duplicate instruction: always run tests","related":{"path":` + jsonQuote(t, filepath.FromSlash("omx/AGENTS.md")) + `,"line":1,"column":1}},` +
		`{"path":` + jsonQuote(t, filepath.FromSlash("sub/AGENTS.md")) + `,"line":2,"column":1,"severity":"warning","rule":"duplicate-instruction","message":"Duplicate instruction: 必ずテストを実行すること。","related":{"path":` + jsonQuote(t, filepath.FromSlash("sub/AGENTS.md")) + `,"line":1,"column":1}}]}` + "\n"
	if stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
}

func TestCLIJSONDeterministicOrdering(t *testing.T) {
	root := writeLintFixture(t)
	var first string
	for i := 0; i < 10; i++ {
		var stdout, stderr bytes.Buffer
		if code := run([]string{root, "--format", "json"}, &stdout, &stderr); code != 1 || stderr.Len() != 0 {
			t.Fatalf("code=%d stderr=%q", code, stderr.String())
		}
		if i == 0 {
			first = stdout.String()
			diagnostics := assertCLIJSONSafe(t, stdout.Bytes())
			wantPaths := []string{
				filepath.Join(root, "AGENTS.md"), filepath.Join(root, "AGENTS.md"),
				filepath.Join(root, "omx", "AGENTS.md"), filepath.Join(root, "sub", "AGENTS.md"),
			}
			if len(diagnostics) != len(wantPaths) {
				t.Fatalf("diagnostics = %+v from %q", diagnostics, stdout.String())
			}
			for j, wantPath := range wantPaths {
				if diagnostics[j].Path != wantPath {
					t.Fatalf("diagnostics[%d].Path = %q, want %q", j, diagnostics[j].Path, wantPath)
				}
			}
			continue
		}
		if stdout.String() != first {
			t.Fatalf("run %d = %q, want %q", i, stdout.String(), first)
		}
	}
}

func TestCLIJSONJapaneseMessagePreserved(t *testing.T) {
	root := writeLintFixture(t)
	var stdout, stderr bytes.Buffer
	code := run([]string{filepath.Join(root, "sub", "AGENTS.md"), "--format", "json"}, &stdout, &stderr)
	if code != 1 || stderr.Len() != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"message":"Duplicate instruction: 必ずテストを実行すること。"`) {
		t.Fatalf("Japanese message not preserved raw: %q", stdout.String())
	}
}

func TestCLIJSONCleanAndNoFiles(t *testing.T) {
	root := writeLintFixture(t)
	cleanFile := filepath.Join(root, "clean", "AGENTS.md")
	cleanDir := filepath.Join(root, "clean")
	noFilesDir := t.TempDir()
	want := "{\"schema_version\":1,\"diagnostics\":[]}\n"
	for _, tt := range []struct {
		name string
		args []string
	}{
		{"clean file", []string{cleanFile, "--format", "json"}},
		{"clean directory", []string{cleanDir, "--format", "json"}},
		{"no instruction files", []string{noFilesDir, "--format", "json"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(tt.args, &stdout, &stderr)
			if code != 0 || stderr.Len() != 0 || stdout.String() != want {
				t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
		})
	}
}

func TestCLITextNoFilesUnchanged(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{t.TempDir()}, &stdout, &stderr)
	if code != 0 || stderr.Len() != 0 || stdout.String() != "no supported instruction files found\n" {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestCLIScopeFormatsUnchanged(t *testing.T) {
	root := filepath.Join(scopeFixtures, "root-with-siblings")
	tree := "AGENTS.md\n├── backend/AGENTS.md\n└── frontend/AGENTS.md\n"
	wantJSON := `{"schema_version":1,"entries":[{"path":"AGENTS.md","parent":null},{"path":"backend/AGENTS.md","parent":"AGENTS.md"},{"path":"frontend/AGENTS.md","parent":"AGENTS.md"}]}` + "\n"
	for _, tt := range []struct {
		name string
		args []string
		want string
	}{
		{"scope tree", []string{"scope", root}, tree},
		{"scope explicit tree", []string{"scope", root, "--format", "tree"}, tree},
		{"scope json", []string{"scope", root, "--format", "json"}, wantJSON},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(tt.args, &stdout, &stderr)
			if code != 0 || stderr.Len() != 0 || stdout.String() != tt.want {
				t.Fatalf("code=%d stdout=%q stderr=%q, want %q", code, stdout.String(), stderr.String(), tt.want)
			}
		})
	}
}

func TestCLIRejectsInvalidFormat(t *testing.T) {
	root := writeLintFixture(t)
	for _, tt := range []struct {
		name string
		args []string
	}{
		{"scope value on lint", []string{root, "--format", "tree"}},
		{"uppercase value", []string{root, "--format", "TEXT"}},
		{"trailing space in value", []string{root, "--format", "json "}},
		{"unknown value", []string{root, "--format", "yaml"}},
		{"equals syntax", []string{root, "--format=json"}},
		{"empty value", []string{root, "--format", ""}},
		{"identical duplicate", []string{root, "--format", "json", "--format", "json"}},
		{"conflicting duplicate", []string{root, "--format", "text", "--format", "json"}},
		{"missing value at end", []string{root, "--format"}},
		{"missing value before option", []string{root, "--format", "--exclude", "omx"}},
		{"missing value before path", []string{root, "--format", root}},
		{"no path", []string{"--format", "json"}},
		{"json alias", []string{root, "--json"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(tt.args, &stdout, &stderr)
			if code != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "usage:") {
				t.Fatalf("args=%q code=%d stdout=%q stderr=%q", tt.args, code, stdout.String(), stderr.String())
			}
		})
	}
}

func TestCLILintRejectsFor(t *testing.T) {
	root := writeLintFixture(t)
	for _, args := range [][]string{
		{root, "--for", "new.go"},
		{filepath.Join(root, "AGENTS.md"), "--for", "new.go"},
	} {
		var stdout, stderr bytes.Buffer
		code := run(args, &stdout, &stderr)
		if code != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "usage:") {
			t.Fatalf("args=%q code=%d stdout=%q stderr=%q", args, code, stdout.String(), stderr.String())
		}
	}
}

func TestCLIHelpDocumentsLintFormat(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"-h"}} {
		var stdout, stderr bytes.Buffer
		code := run(args, &stdout, &stderr)
		if code != 0 || stderr.Len() != 0 ||
			!strings.Contains(stdout.String(), "--format <text|json>") ||
			!strings.Contains(stdout.String(), "Output format for lint diagnostics. Default: text.") ||
			!strings.Contains(stdout.String(), "  instrlint <file-or-directory> [--exclude <directory>]...\n") ||
			!strings.Contains(stdout.String(), "--format <tree|json>") {
			t.Fatalf("args=%q code=%d stdout=%q stderr=%q", args, code, stdout.String(), stderr.String())
		}
	}
}

func TestCLILintInvalidUTF8PathJSON(t *testing.T) {
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		t.Skipf("%s cannot create invalid UTF-8 filenames", runtime.GOOS)
	}
	root := t.TempDir()
	directory := filepath.Join(root, "bad\xff\x1b")
	if err := os.Mkdir(directory, 0755); err != nil {
		t.Skipf("invalid UTF-8 filenames unavailable: %v", err)
	}
	for _, file := range []string{filepath.Join(root, "AGENTS.md"), filepath.Join(directory, "AGENTS.md")} {
		if err := os.WriteFile(file, []byte("Always run tests.\n- always run tests\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{root, "--format", "json"}, &stdout, &stderr)
	if code != 2 || stdout.Len() != 0 || stderr.String() != "instrlint: cannot render lint JSON: path is not valid UTF-8\n" {
		t.Fatalf("json code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = run([]string{root}, &stdout, &stderr)
	if code != 1 || stderr.Len() != 0 || strings.Count(stdout.String(), "duplicate-instruction") != 2 {
		t.Fatalf("text code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

type cliFailingWriter struct {
	written int
	limit   int
	err     error
}

func (w *cliFailingWriter) Write(p []byte) (int, error) {
	n := min(w.limit, len(p))
	w.written += n
	return n, w.err
}

func TestCLIJSONWriteFailure(t *testing.T) {
	root := writeLintFixture(t)
	file := filepath.Join(root, "AGENTS.md")
	for _, tt := range []struct {
		name       string
		writer     *cliFailingWriter
		wantStderr string
	}{
		{"immediate error", &cliFailingWriter{0, 0, errors.New("disk full")}, "instrlint: write output: disk full\n"},
		{"partial write", &cliFailingWriter{0, 5, nil}, "instrlint: write output: short write\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var stderr bytes.Buffer
			code := run([]string{file, "--format", "json"}, tt.writer, &stderr)
			if code != 2 || stderr.String() != tt.wantStderr {
				t.Fatalf("code=%d stdout=%d bytes stderr=%q, want %q", code, tt.writer.written, stderr.String(), tt.wantStderr)
			}
		})
	}
}

func TestCLIExcludeShadowsFormat(t *testing.T) {
	root := writeLintFixture(t)
	var plainOut, shadowOut, combinedOut, filteredOut, stderr bytes.Buffer
	plainCode := run([]string{root}, &plainOut, &stderr)
	shadowCode := run([]string{"--exclude", "--format", root}, &shadowOut, &stderr)
	if plainCode != shadowCode || plainOut.String() != shadowOut.String() || stderr.Len() != 0 {
		t.Fatalf("plain code=%d out=%q, shadow code=%d out=%q", plainCode, plainOut.String(), shadowCode, shadowOut.String())
	}
	run([]string{root, "--exclude", "omx"}, &filteredOut, &stderr)
	run([]string{"--exclude", "--format", "--exclude", "omx", root}, &combinedOut, &stderr)
	if filteredOut.String() != combinedOut.String() {
		t.Fatalf("combined out=%q, want %q", combinedOut.String(), filteredOut.String())
	}
}
