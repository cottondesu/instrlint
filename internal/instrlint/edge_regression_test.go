package instrlint

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLint_detectsDuplicateAfterInlineHTMLComment(t *testing.T) {
	path := filepath.Join(t.TempDir(), "AGENTS.md")
	data := []byte("- Use npm. <!-- note -->\n- Use npm.\n")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	result, err := Lint(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Diagnostics) != 1 || result.Diagnostics[0].Rule != "duplicate-instruction" ||
		result.Diagnostics[0].Line != 2 || result.Diagnostics[0].Related.Line != 1 {
		t.Fatalf("Lint() = %#v, want duplicate at line 2 related to line 1", result)
	}
}

func TestLint_detectsConflictInWideOrderedList(t *testing.T) {
	path := filepath.Join(t.TempDir(), "AGENTS.md")
	data := []byte("100. Use npm for frontend dependencies.\n     1. Use pnpm for frontend dependencies.\n")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	result, err := Lint(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Diagnostics) != 1 || result.Diagnostics[0].Rule != "conflicting-instruction" ||
		result.Diagnostics[0].Line != 2 || result.Diagnostics[0].Column != 6 || result.Diagnostics[0].Related.Line != 1 {
		t.Fatalf("Lint() = %#v, want conflict at line 2 column 6 related to line 1", result)
	}
}

func TestLint_detectsBareEditParagraphConflict(t *testing.T) {
	path := filepath.Join(t.TempDir(), "AGENTS.md")
	data := []byte("Edit generated files directly.\nDo not edit generated files directly.\n")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	result, err := Lint(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Diagnostics) != 1 || result.Diagnostics[0].Rule != "conflicting-instruction" ||
		result.Diagnostics[0].Line != 2 || result.Diagnostics[0].Related.Line != 1 {
		t.Fatalf("Lint() = %#v, want paragraph conflict at line 2 related to line 1", result)
	}
}

func TestParse_skipsSetextHeadings(t *testing.T) {
	data := []byte("Use npm\n---\nUse npm\n===\n- Use npm\n")
	got, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	want := []Instruction{{Text: "Use npm", Line: 5, Column: 1}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Parse() = %#v, want %#v", got, want)
	}
}

func TestParse_reportsIndentedListColumn(t *testing.T) {
	data := []byte("  - Use npm.\n  - Use npm.\n")
	got, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	want := []Instruction{
		{Text: "Use npm.", Line: 1, Column: 3},
		{Text: "Use npm.", Line: 2, Column: 3},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Parse() = %#v, want %#v", got, want)
	}
}

func TestDiscoverWithExcludes_matchesSlashSeparatedRelativePath(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"tools/cache/AGENTS.md", "other/cache/AGENTS.md"} {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("Run tests.\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	got, err := DiscoverWithExcludes(root, []string{"tools/cache"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{filepath.Join(root, "other", "cache", "AGENTS.md")}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("DiscoverWithExcludes() = %v, want %v", got, want)
	}
}
