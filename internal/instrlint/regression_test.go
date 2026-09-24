package instrlint

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestRegressionCorpus(t *testing.T) {
	tests := []struct {
		name string
		want [][2]int
	}{
		{"clean-realistic", nil},
		{"duplicate-realistic", [][2]int{{8, 3}, {9, 4}}},
		{"nested-multiline", [][2]int{{8, 4}}},
		{"code-comments", nil},
		{"japanese-mixed", [][2]int{{3, 2}, {5, 4}}},
		{"false-positives", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join("testdata", "regression", tt.name, "AGENTS.md")
			result, err := Lint(path)
			if err != nil {
				t.Fatal(err)
			}
			if result.Files != 1 || len(result.Diagnostics) != len(tt.want) {
				t.Fatalf("Lint(%q) = %#v, want %d diagnostics", path, result, len(tt.want))
			}
			for i, diagnostic := range result.Diagnostics {
				if diagnostic.Rule != "duplicate-instruction" || diagnostic.Severity != Warning ||
					diagnostic.Line != tt.want[i][0] || diagnostic.Related.Line != tt.want[i][1] {
					t.Fatalf("diagnostic %d = %#v, want line %d related %d", i, diagnostic, tt.want[i][0], tt.want[i][1])
				}
			}
		})
	}
}

func TestRegressionExcludeGeneratedSessions(t *testing.T) {
	root := filepath.Join("testdata", "regression", "exclude-generated")
	all, err := Lint(root)
	if err != nil {
		t.Fatal(err)
	}
	if all.Files != 3 || len(all.Diagnostics) != 3 {
		t.Fatalf("Lint(all) = %#v", all)
	}

	filtered, err := LintWithExcludes(root, []string{".omx"})
	if err != nil {
		t.Fatal(err)
	}
	if filtered.Files != 1 || len(filtered.Diagnostics) != 1 || filtered.Diagnostics[0].File != filepath.Join(root, "AGENTS.md") {
		t.Fatalf("LintWithExcludes() = %#v", filtered)
	}
}

func TestParseNestedMultilineAndLineEndings(t *testing.T) {
	path := filepath.Join("testdata", "regression", "nested-multiline", "AGENTS.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := []Instruction{
		{Text: "Always run the complete test suite before committing changes to the repository.", Line: 4, Column: 3},
		{Text: "Run unit tests.", Line: 6, Column: 3},
		{Text: "Run integration tests.", Line: 7, Column: 3},
		{Text: "Always run the complete test suite before committing changes to the repository.", Line: 8, Column: 1},
		{Text: "Run tests on Linux.", Line: 11, Column: 5},
		{Text: "Run tests on Windows.", Line: 12, Column: 5},
	}
	for _, tt := range []struct {
		name string
		data []byte
	}{
		{"LF", data},
		{"CRLF", []byte(strings.ReplaceAll(string(data), "\n", "\r\n"))},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.data)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("Parse() = %#v, want %#v", got, want)
			}
		})
	}
}

func TestNormalizePreservesInlineCodeCase(t *testing.T) {
	upper := Normalize("Run `GO TEST` before committing.")
	lower := Normalize("run `go test` before committing")
	if upper == lower {
		t.Fatalf("case-sensitive inline code collapsed to %q", upper)
	}
	if got := Normalize("RUN `go test` BEFORE COMMITTING."); got != lower {
		t.Fatalf("ordinary ASCII case was not folded: %q != %q", got, lower)
	}
}

func TestParseListMarkerVariants(t *testing.T) {
	data := []byte("* Keep dependencies minimal.\n+ Do not edit generated files.\n1) Run tests.\n- Before committing:\n  2. Run unit tests.\n")
	got, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	want := []Instruction{
		{Text: "Keep dependencies minimal.", Line: 1, Column: 1},
		{Text: "Do not edit generated files.", Line: 2, Column: 1},
		{Text: "Run tests.", Line: 3, Column: 1},
		{Text: "Run unit tests.", Line: 5, Column: 3},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Parse() = %#v, want %#v", got, want)
	}
}

func TestParseDoesNotCloseFenceAtIndentedCode(t *testing.T) {
	data := []byte("```text\n    ```\n- Ignore this.\n```\n- Keep this.\n")
	got, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	want := []Instruction{{Text: "Keep this.", Line: 5, Column: 1}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Parse() = %#v, want %#v", got, want)
	}
}

func TestNormalizeKeepsEmphasisDistinct(t *testing.T) {
	plain := Normalize("Always run tests before committing.")
	emphasized := Normalize("**Always run tests before committing.**")
	if plain == emphasized {
		t.Fatalf("emphasis was removed: %q", plain)
	}
}

func BenchmarkParseRealistic(b *testing.B) {
	path := filepath.Join("testdata", "regression", "clean-realistic", "AGENTS.md")
	data, err := os.ReadFile(path)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Parse(data); err != nil {
			b.Fatal(err)
		}
	}
}
