package instrlint

import (
	"path/filepath"
	"testing"
)

type expectedRegressionDiagnostic struct {
	line        int
	relatedLine int
}

func TestRealWorldRegressionCorpus(t *testing.T) {
	tests := []struct {
		name string
		want []expectedRegressionDiagnostic
	}{
		{name: "realistic-clean"},
		{name: "realistic-single-duplicate", want: []expectedRegressionDiagnostic{{line: 11, relatedLine: 5}}},
		{name: "false-positives"},
		{name: "markdown-edge-cases"},
		{name: "blockquotes"},
		{name: "emphasis"},
		{name: "heading-paragraphs", want: []expectedRegressionDiagnostic{{line: 14, relatedLine: 10}}},
		{name: "inline-code", want: []expectedRegressionDiagnostic{{line: 6, relatedLine: 5}}},
		{name: "ordered-lists", want: []expectedRegressionDiagnostic{{line: 7, relatedLine: 3}}},
		{name: "nested-multiline", want: []expectedRegressionDiagnostic{{line: 8, relatedLine: 4}}},
		{name: "japanese", want: []expectedRegressionDiagnostic{{line: 3, relatedLine: 2}}},
		{name: "mixed-language"},
		{name: "code-comments"},
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
			for i, want := range tt.want {
				diagnostic := result.Diagnostics[i]
				if diagnostic.Rule != "duplicate-instruction" || diagnostic.Severity != Warning ||
					diagnostic.File != path || diagnostic.Line != want.line || diagnostic.Column != 1 ||
					diagnostic.Related.File != path || diagnostic.Related.Line != want.relatedLine || diagnostic.Related.Column != 1 {
					t.Fatalf("diagnostic %d = %#v, want line %d related %d", i, diagnostic, want.line, want.relatedLine)
				}
			}
		})
	}
}

func TestFixtureHeavyRepositoryReportsOnlyIntentionalDuplicates(t *testing.T) {
	root := filepath.Join("testdata", "regression", "fixture-heavy-repository")
	result, err := Lint(root)
	if err != nil {
		t.Fatal(err)
	}
	wantFile := filepath.Join(root, "intentional-duplicates", "AGENTS.md")
	if result.Files != 4 || len(result.Diagnostics) != 1 {
		t.Fatalf("Lint(%q) = %#v, want one diagnostic across four files", root, result)
	}
	diagnostic := result.Diagnostics[0]
	if diagnostic.Rule != "duplicate-instruction" || diagnostic.File != wantFile ||
		diagnostic.Line != 4 || diagnostic.Related.File != wantFile || diagnostic.Related.Line != 3 {
		t.Fatalf("diagnostic = %#v", diagnostic)
	}

	for _, directory := range []string{"clean-realistic", "code-comments", "false-positives"} {
		path := filepath.Join(root, directory)
		clean, err := Lint(path)
		if err != nil {
			t.Fatal(err)
		}
		if clean.Files != 1 || len(clean.Diagnostics) != 0 {
			t.Fatalf("Lint(%q) = %#v, want clean", path, clean)
		}
	}
}

func TestExcludeGeneratedStateChangesOnlyGeneratedDiagnostics(t *testing.T) {
	root := filepath.Join("testdata", "regression", "exclude-generated-state")
	generated := filepath.Join(root, ".omx", "generated", "AGENTS.md")
	repository := filepath.Join(root, "AGENTS.md")
	fixture := filepath.Join(root, "testdata", "intentional-duplicate", "AGENTS.md")

	all, err := Lint(root)
	if err != nil {
		t.Fatal(err)
	}
	if all.Files != 3 || len(all.Diagnostics) != 3 {
		t.Fatalf("Lint(all) = %#v", all)
	}
	for i, wantFile := range []string{generated, repository, fixture} {
		diagnostic := all.Diagnostics[i]
		if diagnostic.File != wantFile || diagnostic.Line != 2 ||
			diagnostic.Related.File != wantFile || diagnostic.Related.Line != 1 {
			t.Fatalf("all diagnostic %d = %#v, want file %q", i, diagnostic, wantFile)
		}
	}

	filtered, err := LintWithExcludes(root, []string{".omx"})
	if err != nil {
		t.Fatal(err)
	}
	if filtered.Files != 2 || len(filtered.Diagnostics) != 2 {
		t.Fatalf("LintWithExcludes() = %#v", filtered)
	}
	for i, wantFile := range []string{repository, fixture} {
		if filtered.Diagnostics[i].File != wantFile {
			t.Fatalf("filtered diagnostic %d = %#v, want file %q", i, filtered.Diagnostics[i], wantFile)
		}
	}
}
