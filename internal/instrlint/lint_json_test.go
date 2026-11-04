package instrlint

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"
)

type decodedLintRelated struct {
	Path   string
	Line   int
	Column int
}

type decodedLintDiagnostic struct {
	Path     string
	Line     int
	Column   int
	Severity string
	Rule     string
	Message  string
	Related  *decodedLintRelated
}

// decodeLintJSON strictly decodes a lint JSON document, rejecting unknown
// fields, missing required fields, and anything after the single document.
func decodeLintJSON(t *testing.T, raw []byte) (int, []decodedLintDiagnostic) {
	t.Helper()
	var document struct {
		SchemaVersion *int `json:"schema_version"`
		Diagnostics   []struct {
			Path     *string `json:"path"`
			Line     *int    `json:"line"`
			Column   *int    `json:"column"`
			Severity *string `json:"severity"`
			Rule     *string `json:"rule"`
			Message  *string `json:"message"`
			Related  *struct {
				Path   *string `json:"path"`
				Line   *int    `json:"line"`
				Column *int    `json:"column"`
			} `json:"related"`
		} `json:"diagnostics"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&document); err != nil {
		t.Fatalf("decode %q: %v", raw, err)
	}
	if decoder.More() {
		t.Fatalf("trailing data after JSON document: %q", raw)
	}
	if document.SchemaVersion == nil || document.Diagnostics == nil {
		t.Fatalf("missing schema_version or diagnostics: %q", raw)
	}
	diagnostics := make([]decodedLintDiagnostic, 0, len(document.Diagnostics))
	for _, entry := range document.Diagnostics {
		if entry.Path == nil || entry.Line == nil || entry.Column == nil ||
			entry.Severity == nil || entry.Rule == nil || entry.Message == nil {
			t.Fatalf("diagnostic without required field: %q", raw)
		}
		decoded := decodedLintDiagnostic{
			Path: *entry.Path, Line: *entry.Line, Column: *entry.Column,
			Severity: *entry.Severity, Rule: *entry.Rule, Message: *entry.Message,
		}
		if entry.Related != nil {
			if entry.Related.Path == nil || entry.Related.Line == nil || entry.Related.Column == nil {
				t.Fatalf("related location without required field: %q", raw)
			}
			decoded.Related = &decodedLintRelated{
				Path: *entry.Related.Path, Line: *entry.Related.Line, Column: *entry.Related.Column,
			}
		}
		diagnostics = append(diagnostics, decoded)
	}
	return *document.SchemaVersion, diagnostics
}

func assertLintJSONSafe(t *testing.T, raw []byte) {
	t.Helper()
	if !utf8.Valid(raw) || !bytes.HasSuffix(raw, []byte("\n")) || bytes.Count(raw, []byte("\n")) != 1 {
		t.Fatalf("JSON output must be one valid UTF-8 line: %q", raw)
	}
	for _, r := range string(raw[:len(raw)-1]) {
		if !strconv.IsGraphic(r) {
			t.Fatalf("JSON output contains raw non-graphic rune %U: %q", r, raw)
		}
	}
}

func renderLintJSONString(t *testing.T, diagnostics []Diagnostic) string {
	t.Helper()
	var output bytes.Buffer
	if err := RenderLintJSON(&output, diagnostics); err != nil {
		t.Fatal(err)
	}
	assertLintJSONSafe(t, output.Bytes())
	return output.String()
}

// lintJSONFixtureDiagnostics runs the real detectors over fixture content so
// renderer tests exercise genuine Diagnostic values.
func lintJSONFixtureDiagnostics(t *testing.T, path, content string) []Diagnostic {
	t.Helper()
	instructions, err := Parse([]byte(content))
	if err != nil {
		t.Fatal(err)
	}
	var diagnostics []Diagnostic
	diagnostics = append(diagnostics, Duplicates(path, instructions)...)
	diagnostics = append(diagnostics, Conflicts(path, instructions)...)
	return diagnostics
}

func TestRenderLintJSON_duplicate(t *testing.T) {
	diagnostics := lintJSONFixtureDiagnostics(t, "AGENTS.md", "Always run tests.\n- always run tests\n")
	if len(diagnostics) != 1 || diagnostics[0].Rule != "duplicate-instruction" {
		t.Fatalf("fixture diagnostics = %+v", diagnostics)
	}
	want := `{"schema_version":1,"diagnostics":[{"path":"AGENTS.md","line":2,"column":1,"severity":"warning","rule":"duplicate-instruction","message":"Duplicate instruction: always run tests","related":{"path":"AGENTS.md","line":1,"column":1}}]}` + "\n"
	if got := renderLintJSONString(t, diagnostics); got != want {
		t.Fatalf("lint JSON = %q, want %q", got, want)
	}
	version, decoded := decodeLintJSON(t, []byte(want))
	if version != 1 || len(decoded) != 1 {
		t.Fatalf("version=%d decoded=%+v", version, decoded)
	}
	source := diagnostics[0]
	if decoded[0].Path != source.File || decoded[0].Line != source.Line || decoded[0].Column != source.Column ||
		decoded[0].Severity != string(source.Severity) || decoded[0].Rule != source.Rule || decoded[0].Message != source.Message ||
		decoded[0].Related == nil || decoded[0].Related.Path != source.Related.File ||
		decoded[0].Related.Line != source.Related.Line || decoded[0].Related.Column != source.Related.Column {
		t.Fatalf("decoded = %+v, want round-trip of %+v", decoded[0], source)
	}
}

func TestRenderLintJSON_conflict(t *testing.T) {
	diagnostics := lintJSONFixtureDiagnostics(t, "AGENTS.md", "Always use npm.\nNever use npm.\n")
	if len(diagnostics) != 1 || diagnostics[0].Rule != "conflicting-instruction" {
		t.Fatalf("fixture diagnostics = %+v", diagnostics)
	}
	want := `{"schema_version":1,"diagnostics":[{"path":"AGENTS.md","line":2,"column":1,"severity":"warning","rule":"conflicting-instruction","message":"Conflicting instruction: Never use npm.","related":{"path":"AGENTS.md","line":1,"column":1}}]}` + "\n"
	if got := renderLintJSONString(t, diagnostics); got != want {
		t.Fatalf("lint JSON = %q, want %q", got, want)
	}
	version, decoded := decodeLintJSON(t, []byte(want))
	source := diagnostics[0]
	if version != 1 || len(decoded) != 1 || decoded[0].Rule != source.Rule || decoded[0].Message != source.Message ||
		decoded[0].Related == nil || decoded[0].Related.Line != source.Related.Line {
		t.Fatalf("version=%d decoded=%+v", version, decoded)
	}
}

func TestRenderLintJSON_multipleDiagnosticsPreserveOrder(t *testing.T) {
	diagnostics := []Diagnostic{
		{Rule: "conflicting-instruction", Severity: Warning, Message: "m2",
			Location: Location{File: "b/AGENTS.md", Line: 5, Column: 2}, Related: Location{File: "b/AGENTS.md", Line: 4, Column: 1}},
		{Rule: "duplicate-instruction", Severity: Warning, Message: "m1",
			Location: Location{File: "a/AGENTS.md", Line: 1, Column: 1}, Related: Location{File: "a/AGENTS.md", Line: 2, Column: 1}},
		{Rule: "duplicate-instruction", Severity: Warning, Message: "m3",
			Location: Location{File: "b/AGENTS.md", Line: 1, Column: 1}, Related: Location{File: "a/AGENTS.md", Line: 1, Column: 1}},
	}
	want := `{"schema_version":1,"diagnostics":[{"path":"b/AGENTS.md","line":5,"column":2,"severity":"warning","rule":"conflicting-instruction","message":"m2","related":{"path":"b/AGENTS.md","line":4,"column":1}},{"path":"a/AGENTS.md","line":1,"column":1,"severity":"warning","rule":"duplicate-instruction","message":"m1","related":{"path":"a/AGENTS.md","line":2,"column":1}},{"path":"b/AGENTS.md","line":1,"column":1,"severity":"warning","rule":"duplicate-instruction","message":"m3","related":{"path":"a/AGENTS.md","line":1,"column":1}}]}` + "\n"
	if got := renderLintJSONString(t, diagnostics); got != want {
		t.Fatalf("lint JSON = %q, want %q", got, want)
	}
	_, decoded := decodeLintJSON(t, []byte(want))
	if len(decoded) != len(diagnostics) {
		t.Fatalf("decoded %d diagnostics, want %d", len(decoded), len(diagnostics))
	}
	for i := range diagnostics {
		if decoded[i].Path != diagnostics[i].File || decoded[i].Rule != diagnostics[i].Rule ||
			decoded[i].Message != diagnostics[i].Message || decoded[i].Related == nil ||
			decoded[i].Related.Path != diagnostics[i].Related.File {
			t.Fatalf("decoded[%d] = %+v, want round-trip of %+v", i, decoded[i], diagnostics[i])
		}
	}
}

func TestRenderLintJSON_emptyDiagnosticsIsArray(t *testing.T) {
	for _, diagnostics := range [][]Diagnostic{nil, {}} {
		if got, want := renderLintJSONString(t, diagnostics), "{\"schema_version\":1,\"diagnostics\":[]}\n"; got != want {
			t.Fatalf("empty lint JSON = %q, want %q", got, want)
		}
	}
}

func TestRenderLintJSON_omitsZeroRelated(t *testing.T) {
	diagnostics := []Diagnostic{{
		Rule: "future-rule", Severity: Warning, Message: "m",
		Location: Location{File: "AGENTS.md", Line: 3, Column: 1},
	}}
	want := `{"schema_version":1,"diagnostics":[{"path":"AGENTS.md","line":3,"column":1,"severity":"warning","rule":"future-rule","message":"m"}]}` + "\n"
	got := renderLintJSONString(t, diagnostics)
	if got != want || strings.Contains(got, "related") {
		t.Fatalf("lint JSON = %q, want %q", got, want)
	}
	_, decoded := decodeLintJSON(t, []byte(got))
	if len(decoded) != 1 || decoded[0].Related != nil {
		t.Fatalf("decoded = %+v, want one diagnostic without related", decoded)
	}
}

func TestRenderLintJSON_preservesNormalUnicode(t *testing.T) {
	diagnostics := []Diagnostic{
		{Rule: "duplicate-instruction", Severity: Warning, Message: "Duplicate instruction: 必ずテストを実行すること。",
			Location: Location{File: "日本語/AGENTS.md", Line: 2, Column: 1}, Related: Location{File: "日本語/AGENTS.md", Line: 1, Column: 1}},
		{Rule: "duplicate-instruction", Severity: Warning, Message: "Duplicate instruction: café a&b<c> \"quoted\"",
			Location: Location{File: "équipe/AGENTS.md", Line: 1, Column: 1}, Related: Location{File: "équipe/AGENTS.md", Line: 1, Column: 1}},
	}
	got := renderLintJSONString(t, diagnostics)
	for _, raw := range []string{
		`"path":"日本語/AGENTS.md"`,
		`"message":"Duplicate instruction: 必ずテストを実行すること。"`,
		`"message":"Duplicate instruction: café a&b<c> \"quoted\""`,
	} {
		if !strings.Contains(got, raw) {
			t.Fatalf("graphic Unicode escaped in %q: want %q", got, raw)
		}
	}
	version, decoded := decodeLintJSON(t, []byte(got))
	if version != 1 || len(decoded) != 2 || decoded[0].Message != diagnostics[0].Message || decoded[1].Message != diagnostics[1].Message {
		t.Fatalf("decoded = %+v from %q", decoded, got)
	}
}

func TestRenderLintJSON_isDeterministic(t *testing.T) {
	diagnostics := lintJSONFixtureDiagnostics(t, "AGENTS.md",
		"Always run tests.\n- always run tests\nAlways use npm.\nNever use npm.\nUse npm for deps.\nUse pnpm for deps.\n")
	if len(diagnostics) < 3 {
		t.Fatalf("fixture diagnostics = %+v", diagnostics)
	}
	first := renderLintJSONString(t, diagnostics)
	for i := 0; i < 20; i++ {
		if got := renderLintJSONString(t, diagnostics); got != first {
			t.Fatalf("render %d = %q, want %q", i, got, first)
		}
	}
}

func TestRenderLintJSON_escapesUnsafeRunesLosslessly(t *testing.T) {
	for _, tt := range []struct {
		label, path, message, wantEscape string
	}{
		{"newline", "line\nforged/AGENTS.md", "Duplicate instruction: line\nforged", `\n`},
		{"carriage return", "carriage\rreturn/AGENTS.md", "Duplicate instruction: carriage\rreturn", `\r`},
		{"terminal escape", "esc\x1b]0;title\a/AGENTS.md", "Duplicate instruction: esc\x1b]0;title\a", `\u001b`},
		{"bell", "bell\a/AGENTS.md", "Duplicate instruction: bell\a", `\u0007`},
		{"delete", "del\x7f/AGENTS.md", "Duplicate instruction: del\x7f", `\u007f`},
		{"bidirectional control", "bidi\u202eevil/AGENTS.md", "Duplicate instruction: bidi\u202eevil", `\u202e`},
		{"bidirectional isolate", "isolate\u2066x\u2069/AGENTS.md", "Duplicate instruction: isolate\u2066x\u2069", `\u2066`},
		{"line separator", "sep\u2028x/AGENTS.md", "Duplicate instruction: sep\u2028x", `\u2028`},
		{"supplementary tag", "tag\U000E0041/AGENTS.md", "Duplicate instruction: tag\U000E0041", `\udb40\udc41`},
	} {
		t.Run(tt.label, func(t *testing.T) {
			diagnostics := []Diagnostic{{
				Rule: "duplicate-instruction", Severity: Warning, Message: tt.message,
				Location: Location{File: tt.path, Line: 2, Column: 1},
				Related:  Location{File: tt.path, Line: 1, Column: 1},
			}}
			raw := renderLintJSONString(t, diagnostics)
			version, decoded := decodeLintJSON(t, []byte(raw))
			if version != 1 || len(decoded) != 1 || decoded[0].Path != tt.path || decoded[0].Message != tt.message ||
				decoded[0].Related == nil || decoded[0].Related.Path != tt.path {
				t.Fatalf("decoded = %+v from %q, want lossless round-trip", decoded, raw)
			}
			if !strings.Contains(raw, tt.wantEscape) {
				t.Fatalf("output missing escape %s: %q", tt.wantEscape, raw)
			}
		})
	}
}

func TestRenderLintJSON_rejectsInvalidUTF8WithoutOutput(t *testing.T) {
	for _, tt := range []struct {
		name        string
		diagnostic  Diagnostic
		wantMessage string
	}{
		{"path", Diagnostic{Rule: "duplicate-instruction", Severity: Warning, Message: "m",
			Location: Location{File: "bad\xff/AGENTS.md", Line: 1, Column: 1},
			Related:  Location{File: "bad\xff/AGENTS.md", Line: 1, Column: 1}}, "path is not valid UTF-8"},
		{"message", Diagnostic{Rule: "duplicate-instruction", Severity: Warning, Message: "bad\xffmessage",
			Location: Location{File: "AGENTS.md", Line: 1, Column: 1},
			Related:  Location{File: "AGENTS.md", Line: 1, Column: 1}}, "message is not valid UTF-8"},
		{"related path", Diagnostic{Rule: "duplicate-instruction", Severity: Warning, Message: "m",
			Location: Location{File: "AGENTS.md", Line: 1, Column: 1},
			Related:  Location{File: "bad\xff/AGENTS.md", Line: 1, Column: 1}}, "related path is not valid UTF-8"},
		{"severity", Diagnostic{Rule: "r", Severity: Severity("bad\xff"), Message: "m",
			Location: Location{File: "AGENTS.md", Line: 1, Column: 1}}, "severity is not valid UTF-8"},
		{"rule", Diagnostic{Rule: "bad\xff", Severity: Warning, Message: "m",
			Location: Location{File: "AGENTS.md", Line: 1, Column: 1}}, "rule is not valid UTF-8"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var output bytes.Buffer
			err := RenderLintJSON(&output, []Diagnostic{tt.diagnostic})
			if err == nil || output.Len() != 0 || !strings.Contains(err.Error(), tt.wantMessage) ||
				!strings.Contains(err.Error(), "cannot render lint JSON") || !utf8.ValidString(err.Error()) {
				t.Fatalf("err=%v output=%q", err, output.String())
			}
		})
	}
}

type lintTestWriter struct {
	limit int
	err   error
}

func (w lintTestWriter) Write(p []byte) (int, error) {
	return min(w.limit, len(p)), w.err
}

func TestRenderLintJSON_returnsWriteErrors(t *testing.T) {
	diagnostics := lintJSONFixtureDiagnostics(t, "AGENTS.md", "Always run tests.\n- always run tests\n")
	diskFull := errors.New("disk full")
	for _, tt := range []struct {
		name   string
		writer lintTestWriter
		want   error
	}{
		{"immediate error", lintTestWriter{0, diskFull}, diskFull},
		{"partial write with error", lintTestWriter{5, diskFull}, diskFull},
		{"partial write without error", lintTestWriter{5, nil}, io.ErrShortWrite},
		{"empty write without error", lintTestWriter{0, nil}, io.ErrShortWrite},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if err := RenderLintJSON(tt.writer, diagnostics); !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
		})
	}
}
