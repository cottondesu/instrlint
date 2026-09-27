package instrlint

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScope_rendersDiscoveredAncestry(t *testing.T) {
	tests := []struct {
		name     string
		fixture  string
		excludes []string
		want     string
	}{
		{"root only", "root-only", nil, "AGENTS.md\n"},
		{"root with siblings", "root-with-siblings", nil, "AGENTS.md\n├── backend/AGENTS.md\n└── frontend/AGENTS.md\n"},
		{"deep nesting", "deep-nesting", nil, "AGENTS.md\n└── be/AGENTS.md\n    └── be/src/exprs_ext/AGENTS.md\n"},
		{"four levels and missing intermediate file", "four-level", nil, "AGENTS.md\n├── a/AGENTS.md\n│   └── a/b/AGENTS.md\n│       └── a/b/c/AGENTS.md\n└── services/app/internal/AGENTS.md\n"},
		{"rootless forest", "rootless-forest", nil, "apps/application/AGENTS.md\npackages/reporter/AGENTS.md\n"},
		{"multiple roots", "multiple-roots", nil, "apps/AGENTS.md\n└── apps/web/AGENTS.md\npackages/AGENTS.md\n└── packages/cli/AGENTS.md\n"},
		{"excluded subtree visible", "excluded-subtree", nil, "AGENTS.md\n├── .omx/generated/AGENTS.md\n└── src/AGENTS.md\n"},
		{"excluded subtree hidden", "excluded-subtree", []string{".omx"}, "AGENTS.md\n└── src/AGENTS.md\n"},
		{"similar path prefixes remain siblings", "similar-path-prefix", nil, "foo/AGENTS.md\nfoobar/AGENTS.md\n"},
		{"relative excluded subtree", "relative-excluded-subtree", []string{"tools/cache"}, "AGENTS.md\n└── tools/source/AGENTS.md\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := filepath.Join("testdata", "scope", tt.fixture)

			nodes, err := Scope(root, tt.excludes)
			if err != nil {
				t.Fatal(err)
			}
			var output bytes.Buffer
			if err := RenderScope(&output, nodes); err != nil {
				t.Fatal(err)
			}
			if output.String() != tt.want {
				t.Fatalf("scope output = %q, want %q", output.String(), tt.want)
			}
		})
	}
}

func TestRenderScope_escapesControlCharactersInPaths(t *testing.T) {
	root := "repo"
	files := []string{
		filepath.Join(root, "line\nforged", "AGENTS.md"),
		filepath.Join(root, "carriage\rreturn", "AGENTS.md"),
		filepath.Join(root, "esc\x1b]0;title\a", "AGENTS.md"),
		filepath.Join(root, "bidi\u202e", "AGENTS.md"),
	}
	nodes, err := buildScopeHierarchy(root, files)
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := RenderScope(&output, nodes); err != nil {
		t.Fatal(err)
	}
	got := output.String()
	if strings.Count(got, "\n") != len(files) || strings.ContainsAny(got, "\r\x1b\a\u202e") ||
		!strings.Contains(got, `line\nforged/AGENTS.md`) {
		t.Fatalf("scope output contains unescaped path controls: %q", got)
	}
}

func TestScope_usesDefaultExclusionsAndDoesNotFollowDirectorySymlinks(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"AGENTS.md", "node_modules/AGENTS.md", "src/AGENTS.md"} {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, nil, 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(filepath.Join(root, "src"), filepath.Join(root, "linked")); err != nil {
		t.Logf("directory symlinks unavailable: %v", err)
	}
	nodes, err := Scope(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := RenderScope(&output, nodes); err != nil {
		t.Fatal(err)
	}
	if want := "AGENTS.md\n└── src/AGENTS.md\n"; output.String() != want {
		t.Fatalf("scope output = %q, want %q", output.String(), want)
	}
}

func TestScope_acceptsDirectorySymlinkAsScanRoot(t *testing.T) {
	root := t.TempDir()
	real := filepath.Join(root, "real")
	if err := os.Mkdir(real, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(real, "AGENTS.md"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "linked")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("directory symlinks unavailable: %v", err)
	}
	nodes, err := Scope(link, nil)
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := RenderScope(&output, nodes); err != nil {
		t.Fatal(err)
	}
	if output.String() != "AGENTS.md\n" {
		t.Fatalf("scope output = %q, want AGENTS.md", output.String())
	}
}
