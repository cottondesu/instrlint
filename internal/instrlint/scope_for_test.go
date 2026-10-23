package instrlint

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func writeScopeForFile(t *testing.T, root, name string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte{0xff}, 0644); err != nil {
		t.Fatal(err)
	}
}

func assertScopeForChain(t *testing.T, nodes []*ScopeNode, want []string) {
	t.Helper()
	var expectedTree strings.Builder
	for i, path := range want {
		if i > 0 {
			expectedTree.WriteString(strings.Repeat("    ", i-1) + "└── ")
		}
		expectedTree.WriteString(SafeScopeText(path) + "\n")
	}
	var tree bytes.Buffer
	if err := RenderScope(&tree, nodes); err != nil || tree.String() != expectedTree.String() {
		t.Fatalf("tree=%q want=%q err=%v", tree.String(), expectedTree.String(), err)
	}
	raw := renderScopeJSONString(t, nodes)
	if second := renderScopeJSONString(t, nodes); raw != second {
		t.Fatalf("JSON is not deterministic: %q != %q", raw, second)
	}
	version, entries := decodeScopeJSON(t, []byte(raw))
	if version != 1 || len(entries) != len(want) {
		t.Fatalf("version=%d entries=%+v want=%q", version, entries, want)
	}
	for i, entry := range entries {
		if entry.Path != want[i] || i == 0 && entry.Parent != nil ||
			i > 0 && (entry.Parent == nil || *entry.Parent != want[i-1]) {
			t.Fatalf("entry %d=%+v want path=%q with preceding parent", i, entry, want[i])
		}
	}
}

func TestScopeFor_fixtures(t *testing.T) {
	for _, tt := range []struct {
		fixture, target string
		want            []string
	}{
		{"root-only", "new.go", []string{"AGENTS.md"}},
		{"root-only", ".", []string{"AGENTS.md"}},
		{"root-with-siblings", "backend", []string{"AGENTS.md", "backend/AGENTS.md"}},
		{"root-with-siblings", "backend/AGENTS.md", []string{"AGENTS.md", "backend/AGENTS.md"}},
		{"root-with-siblings", "./backend/new.go", []string{"AGENTS.md", "backend/AGENTS.md"}},
		{"root-with-siblings", "frontend/AGENTS.md", []string{"AGENTS.md", "frontend/AGENTS.md"}},
		{"deep-nesting", "be/src/exprs_ext/new.go", []string{"AGENTS.md", "be/AGENTS.md", "be/src/exprs_ext/AGENTS.md"}},
		{"four-level", "services/app/internal", []string{"AGENTS.md", "services/app/internal/AGENTS.md"}},
		{"four-level", "a/b/c/new.go", []string{"AGENTS.md", "a/AGENTS.md", "a/b/AGENTS.md", "a/b/c/AGENTS.md"}},
		{"rootless-forest", "apps/application/new.go", []string{"apps/application/AGENTS.md"}},
		{"rootless-forest", ".", nil},
		{"multiple-roots", "apps/web/AGENTS.md", []string{"apps/AGENTS.md", "apps/web/AGENTS.md"}},
		{"multiple-roots", "packages/cli/new.go", []string{"packages/AGENTS.md", "packages/cli/AGENTS.md"}},
		{"similar-path-prefix", "foobar/new.go", []string{"foobar/AGENTS.md"}},
	} {
		t.Run(tt.fixture+"/"+tt.target, func(t *testing.T) {
			nodes, err := ScopeFor(filepath.Join("testdata", "scope", tt.fixture), nil, tt.target)
			if err != nil {
				t.Fatal(err)
			}
			assertScopeForChain(t, nodes, tt.want)
		})
	}
}

func TestScopeFor_independentGroundTruth(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"AGENTS.md", "a/AGENTS.md", "a/b/c/AGENTS.md", "a/b/c/target.go", "ab/AGENTS.md"} {
		writeScopeForFile(t, root, name)
	}
	nodes, err := ScopeFor(root, nil, "a/b/c/target.go")
	if err != nil {
		t.Fatal(err)
	}
	assertScopeForChain(t, nodes, []string{"AGENTS.md", "a/AGENTS.md", "a/b/c/AGENTS.md"})
}

func TestSelectScopeChain_preservesOriginalNodes(t *testing.T) {
	nodes, err := Scope(filepath.Join("testdata", "scope", "root-with-siblings"), nil)
	if err != nil {
		t.Fatal(err)
	}
	before := renderScopeJSONString(t, nodes)
	selected := selectScopeChain(nodes, "backend")
	if selected[0] == nodes[0] || selected[0].Children[0] == nodes[0].Children[0] {
		t.Fatal("selected nodes share pointers with original hierarchy")
	}
	selected[0].Path = "changed"
	selected[0].Children[0].Path = "changed child"
	selected[0].Children = nil
	if after := renderScopeJSONString(t, nodes); before != after {
		t.Fatalf("original hierarchy mutated: %q != %q", before, after)
	}
}

func TestScopeFor_emptyAncestors(t *testing.T) {
	root := t.TempDir()
	for _, target := range []string{".", "new.go"} {
		nodes, err := ScopeFor(root, nil, target)
		if err != nil || len(nodes) != 0 {
			t.Fatalf("target=%q nodes=%v err=%v", target, nodes, err)
		}
	}
	writeScopeForFile(t, root, "other/AGENTS.md")
	writeScopeForFile(t, root, "target/file.go")
	nodes, err := ScopeFor(root, nil, "target/file.go")
	if err != nil || !reflect.DeepEqual(nodes, []*ScopeNode(nil)) {
		t.Fatalf("nodes=%v err=%v", nodes, err)
	}
}
