package instrlint

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestScopeFor_unicodeAndControls(t *testing.T) {
	for _, name := range []string{"line\nfeed", "carriage\rreturn", "escape\x1b", "bell\a", "delete\x7f", "bidi\u202e", "isolate\u2066x\u2069", "separator\u2028", "tag\U000e0041", "日本語", "café", "quote\"", "back\\slash"} {
		t.Run(name, func(t *testing.T) {
			if runtime.GOOS == "windows" && strings.ContainsAny(name, "\n\r\x1b\a\"\\") {
				t.Skip("Windows does not support this directory name")
			}
			root := t.TempDir()
			writeScopeForFile(t, root, "AGENTS.md")
			writeScopeForFile(t, root, name+"/AGENTS.md")
			writeScopeForFile(t, root, name+"/child/AGENTS.md")
			nodes, err := ScopeFor(root, nil, name+"/child/new.go")
			if err != nil {
				t.Fatal(err)
			}
			assertScopeForChain(t, nodes, []string{"AGENTS.md", name + "/AGENTS.md", name + "/child/AGENTS.md"})
			raw := renderScopeJSONString(t, nodes)
			var tree bytes.Buffer
			if err := RenderScope(&tree, nodes); err != nil {
				t.Fatal(err)
			}
			if strings.Count(raw, "\n") != 1 || strings.Count(tree.String(), "\n") != 3 ||
				strings.ContainsAny(raw+tree.String(), "\r\x1b\a\x7f\u202e\u2066\u2069\u2028\U000e0041") {
				t.Fatalf("unsafe characters in tree=%q JSON=%q", tree.String(), raw)
			}
		})
	}
}

func TestScopeFor_invalidUTF8SelectedPath(t *testing.T) {
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		t.Skipf("%s cannot create invalid UTF-8 filenames", runtime.GOOS)
	}
	root := t.TempDir()
	directory := filepath.Join(root, "bad\xff\x1b")
	if err := os.Mkdir(directory, 0755); err != nil {
		t.Skipf("invalid UTF-8 filenames unavailable: %v", err)
	}
	writeScopeForFile(t, root, "AGENTS.md")
	writeScopeForFile(t, root, "bad\xff\x1b/AGENTS.md")
	nodes, err := ScopeFor(root, nil, "bad\xff\x1b/new.go")
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := RenderScopeJSON(&output, nodes); err == nil || output.Len() != 0 {
		t.Fatalf("JSON output=%q err=%v", output.String(), err)
	}
	if err := RenderScope(&output, nodes); err != nil || output.String() != "AGENTS.md\n└── \"bad\\xff\\x1b/AGENTS.md\"\n" {
		t.Fatalf("tree output=%q err=%v", output.String(), err)
	}
}
