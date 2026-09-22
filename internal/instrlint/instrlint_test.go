package instrlint

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestDiscover(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{
		"AGENTS.md", "nested/AGENTS.md", ".git/AGENTS.md",
		"node_modules/AGENTS.md", "vendor/AGENTS.md", "dist/AGENTS.md",
		"build/AGENTS.md", "out/AGENTS.md", "coverage/AGENTS.md",
		"tmp/AGENTS.md", ".cache/AGENTS.md",
	} {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("Run tests."), 0644); err != nil {
			t.Fatal(err)
		}
	}

	got, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{filepath.Join(root, "AGENTS.md"), filepath.Join(root, "nested", "AGENTS.md")}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Discover() = %v, want %v", got, want)
	}

	file, err := Discover(filepath.Join(root, "nested", "AGENTS.md"))
	if err != nil || !reflect.DeepEqual(file, want[1:]) {
		t.Fatalf("Discover(file) = %v, %v", file, err)
	}
}

func TestDiscoverEmptyAndSymlink(t *testing.T) {
	root := t.TempDir()
	got, err := Discover(root)
	if err != nil || len(got) != 0 {
		t.Fatalf("Discover(empty) = %v, %v", got, err)
	}
	if err := os.Mkdir(filepath.Join(root, "nested"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(root, filepath.Join(root, "nested", "loop")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	got, err = Discover(root)
	if err != nil || len(got) != 0 {
		t.Fatalf("Discover(symlink loop) = %v, %v", got, err)
	}
	if _, err := Discover(filepath.Join(root, "missing")); err == nil {
		t.Fatal("missing path accepted")
	}
	if _, err := Discover(filepath.Join(root, "nested", "loop", "missing")); err == nil {
		t.Fatal("broken path through symlink accepted")
	}
}

func TestParse(t *testing.T) {
	input := "# Title\r\n- Always run tests.\r\n1. テストを実行してください。\r\n\r\nUse npm for frontend dependencies.\r\n```go\r\n- Ignore this.\r\n```\r\n    code()\r\n---\r\nテスト後に commit してください。\r\n"
	got, err := Parse([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	want := []Instruction{
		{Text: "Always run tests.", Line: 2, Column: 1},
		{Text: "テストを実行してください。", Line: 3, Column: 1},
		{Text: "Use npm for frontend dependencies.", Line: 5, Column: 1},
		{Text: "テスト後に commit してください。", Line: 11, Column: 1},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Parse() = %#v, want %#v", got, want)
	}
	if _, err := Parse([]byte{0xff}); err == nil {
		t.Fatal("invalid UTF-8 accepted")
	}
}

func TestParseKeepsFenceOpenUntilClosingMarker(t *testing.T) {
	input := "```text\n```go\n- ignore this\n```\n- Keep this.\n"
	got, err := Parse([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	want := []Instruction{{Text: "Keep this.", Line: 5, Column: 1}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Parse() = %#v, want %#v", got, want)
	}
}

func TestParseSkipsCodeOnlyListItem(t *testing.T) {
	got, err := Parse([]byte("- `go test ./...`\n- Run tests.\n"))
	if err != nil {
		t.Fatal(err)
	}
	want := []Instruction{{Text: "Run tests.", Line: 2, Column: 1}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Parse() = %#v, want %#v", got, want)
	}
}

func TestNormalize(t *testing.T) {
	tests := []struct{ input, want string }{
		{"  -  ALWAYS   run\ttests.  ", "always run tests"},
		{"1. Use npm!", "use npm"},
		{"テストを実行してください。", "テストを実行してください"},
		{"英語と日本語を MIX してください！", "英語と日本語を mix してください"},
		{"Run unit tests.", "run unit tests"},
		{"Run integration tests.", "run integration tests"},
		{"Run tests?", "run tests?"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := Normalize(tt.input); got != tt.want {
				t.Fatalf("Normalize() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLintDuplicateOrderingAndDistinctInstructions(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "AGENTS.md")
	content := strings.Join([]string{
		"Always run tests.",
		"- always run tests",
		"- ALWAYS RUN TESTS!",
		"Run unit tests before committing.",
		"Run integration tests before committing.",
		"Use npm for frontend dependencies.",
		"Use npm for publishing packages.",
	}, "\n")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	result, err := Lint(path)
	if err != nil {
		t.Fatal(err)
	}
	got := result.Diagnostics
	if len(got) != 2 || got[0].Line != 2 || got[0].Related.Line != 1 || got[1].Line != 3 || got[1].Related.Line != 1 {
		t.Fatalf("Lint() = %#v", got)
	}
}

func TestLintRejectsMalformedUTF8(t *testing.T) {
	path := filepath.Join(t.TempDir(), "AGENTS.md")
	if err := os.WriteFile(path, []byte{0xff}, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Lint(path); err == nil {
		t.Fatal("malformed UTF-8 accepted")
	}
}

func TestLintJapaneseDuplicate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "AGENTS.md")
	if err := os.WriteFile(path, []byte("テストを実行してください。\n- テストを実行してください\n"), 0644); err != nil {
		t.Fatal(err)
	}
	result, err := Lint(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Diagnostics) != 1 || result.Diagnostics[0].Line != 2 {
		t.Fatalf("Lint() = %#v", result)
	}
}

func BenchmarkDuplicateRule(b *testing.B) {
	instructions := make([]Instruction, 1000)
	for i := range instructions {
		instructions[i] = Instruction{Text: "Run tests before commit " + string(rune('a'+i%26)), Line: i + 1, Column: 1}
	}
	for i := 0; i < b.N; i++ {
		_ = Duplicates("AGENTS.md", instructions)
	}
}
