package main

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestCLIScopeFor_invalidUTF8SelectedPath(t *testing.T) {
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		t.Skipf("%s cannot create invalid UTF-8 filenames", runtime.GOOS)
	}
	root := t.TempDir()
	name := "bad\xff\x1b"
	if err := os.Mkdir(filepath.Join(root, name), 0755); err != nil {
		t.Skipf("invalid UTF-8 filenames unavailable: %v", err)
	}
	for _, file := range []string{filepath.Join(root, "AGENTS.md"), filepath.Join(root, name, "AGENTS.md")} {
		if err := os.WriteFile(file, nil, 0644); err != nil {
			t.Fatal(err)
		}
	}
	for _, tt := range []struct {
		format, stdout, stderr string
		code                   int
	}{
		{"tree", "AGENTS.md\n└── \"bad\\xff\\x1b/AGENTS.md\"\n", "", 0},
		{"json", "", "instrlint: cannot render scope JSON: path is not valid UTF-8\n", 2},
	} {
		var stdout, stderr bytes.Buffer
		code := run([]string{"scope", root, "--for", name + "/new.go", "--format", tt.format}, &stdout, &stderr)
		if code != tt.code || stdout.String() != tt.stdout || stderr.String() != tt.stderr {
			t.Fatalf("format=%s code=%d stdout=%q stderr=%q", tt.format, code, stdout.String(), stderr.String())
		}
	}
}
