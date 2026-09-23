package instrlint

import (
	"path/filepath"
	"testing"
)

func TestConflictRegressionCorpus(t *testing.T) {
	tests := []struct {
		name string
		want [][2]int
	}{
		{name: "always-never", want: [][2]int{{4, 3}}},
		{name: "positive-negative", want: [][2]int{{4, 3}, {7, 6}}},
		{name: "scoped-tool-conflict", want: [][2]int{{4, 3}}},
		{name: "different-scope-clean"},
		{name: "unrelated-negation-clean"},
		{name: "code-comment-clean"},
		{name: "japanese-clean-or-supported"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			path := filepath.Join("testdata", "regression", "conflicts", tt.name, "AGENTS.md")

			// When
			result, err := Lint(path)

			// Then
			if err != nil {
				t.Fatal(err)
			}
			if result.Files != 1 || len(result.Diagnostics) != len(tt.want) {
				t.Fatalf("Lint(%q) = %#v, want %d diagnostics", path, result, len(tt.want))
			}
			for i, diagnostic := range result.Diagnostics {
				if diagnostic.Rule != "conflicting-instruction" || diagnostic.Severity != Warning ||
					diagnostic.Line != tt.want[i][0] || diagnostic.Related.Line != tt.want[i][1] {
					t.Fatalf("diagnostic %d = %#v, want line %d related %d", i, diagnostic, tt.want[i][0], tt.want[i][1])
				}
			}
		})
	}
}

func TestConflictRegressionCorpus_reportsDuplicateAndConflictTogether(t *testing.T) {
	// Given
	path := filepath.Join("testdata", "regression", "conflicts", "duplicate-and-conflict", "AGENTS.md")

	// When
	result, err := Lint(path)

	// Then
	if err != nil {
		t.Fatal(err)
	}
	want := []struct {
		rule    string
		line    int
		related int
	}{
		{rule: "duplicate-instruction", line: 4, related: 3},
		{rule: "conflicting-instruction", line: 5, related: 3},
	}
	if len(result.Diagnostics) != len(want) {
		t.Fatalf("Lint(%q) = %#v, want %d diagnostics", path, result, len(want))
	}
	for i, diagnostic := range result.Diagnostics {
		if diagnostic.Rule != want[i].rule || diagnostic.Line != want[i].line || diagnostic.Related.Line != want[i].related {
			t.Fatalf("diagnostic %d = %#v, want %#v", i, diagnostic, want[i])
		}
	}
}
