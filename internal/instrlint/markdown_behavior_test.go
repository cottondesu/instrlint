package instrlint

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func parseRegressionFixture(t *testing.T, name string) []Instruction {
	t.Helper()
	path := filepath.Join("testdata", "regression", name, "AGENTS.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	instructions, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	return instructions
}

func TestParseHeadingParagraphBehavior(t *testing.T) {
	got := parseRegressionFixture(t, "heading-paragraphs")
	want := []Instruction{
		{Text: "Always run tests before committing.", Line: 10, Column: 1},
		{Text: "always run tests before committing", Line: 14, Column: 1},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Parse() = %#v, want %#v", got, want)
	}
}

func TestParseBlockquotesOutsideScope(t *testing.T) {
	got := parseRegressionFixture(t, "blockquotes")
	want := []Instruction{{Text: "Keep quoted examples outside lint scope.", Line: 6, Column: 1}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Parse() = %#v, want %#v", got, want)
	}
}

func TestParseEmphasisSyntaxSensitive(t *testing.T) {
	got := parseRegressionFixture(t, "emphasis")
	want := []Instruction{
		{Text: "Always run tests before committing.", Line: 4, Column: 1},
		{Text: "Use **npm** for publishing packages.", Line: 6, Column: 1},
		{Text: "Use npm for publishing packages.", Line: 7, Column: 1},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Parse() = %#v, want %#v", got, want)
	}
}

func TestParseOrderedListBehavior(t *testing.T) {
	got := parseRegressionFixture(t, "ordered-lists")
	want := []Instruction{
		{Text: "Run tests before committing.", Line: 3, Column: 1},
		{Text: "Keep dependencies minimal.", Line: 4, Column: 1},
		{Text: "Do not modify generated files.", Line: 5, Column: 1},
		{Text: "Run tests before committing", Line: 7, Column: 1},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Parse() = %#v, want %#v", got, want)
	}
}

func TestParseMarkdownEdgeCasesOutsideScope(t *testing.T) {
	if got := parseRegressionFixture(t, "markdown-edge-cases"); len(got) != 0 {
		t.Fatalf("Parse() = %#v, want no instructions", got)
	}
}
