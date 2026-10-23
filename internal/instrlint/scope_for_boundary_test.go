package instrlint

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestScopeFor_targetValidation(t *testing.T) {
	root := t.TempDir()
	writeScopeForFile(t, root, "AGENTS.md")
	writeScopeForFile(t, root, "backend/api/user.go")
	writeScopeForFile(t, root, "-name")
	valid := []string{"backend/api/user.go", "./backend/api/user.go", "backend//api/user.go", "backend/api", "backend/api/", "backend/api/.", ".", "./", "AGENTS.md", "backend/api/new.go", "./-name"}
	if runtime.GOOS == "windows" {
		valid = append(valid, `backend\api\user.go`, `backend\api\new.go`, `backend\api\`, `.\-name`)
	}
	for _, target := range valid {
		t.Run("valid/"+target, func(t *testing.T) {
			nodes, err := ScopeFor(root, nil, target)
			if err != nil {
				t.Fatal(err)
			}
			assertScopeForChain(t, nodes, []string{"AGENTS.md"})
		})
	}
	invalid := []string{"", root, "../outside.go", "backend/../../outside.go", "backend/../AGENTS.md", "missing/new.go", "missing/", "missing/.", "backend/api/new.go/", "backend/api/user.go/", "backend/api/user.go/.", "backend/api/user.go/new.go", "bad\x00name"}
	if runtime.GOOS == "windows" {
		invalid = append(invalid, `C:\outside.go`, `C:outside.go`, `\\server\share\file.go`, `\outside.go`, `/outside.go`, `backend\..\AGENTS.md`)
	}
	for _, target := range invalid {
		t.Run("invalid/"+target, func(t *testing.T) {
			if nodes, err := ScopeFor(root, nil, target); err == nil || len(nodes) != 0 {
				t.Fatalf("target=%q nodes=%v err=%v", target, nodes, err)
			}
		})
	}
}

func TestScopeFor_exclusionsShareDiscoveryRules(t *testing.T) {
	root := t.TempDir()
	writeScopeForFile(t, root, "AGENTS.md")
	for name := range excludedDirectories {
		writeScopeForFile(t, root, name+"/AGENTS.md")
		for _, target := range []string{name, name + "/AGENTS.md", name + "/new.go"} {
			if _, err := ScopeFor(root, nil, target); err == nil {
				t.Fatalf("default exclusion accepted %q", target)
			}
		}
	}
	for _, name := range []string{"backend/api/AGENTS.md", "tools/cache/AGENTS.md", "tools/cache-other/AGENTS.md", "nested/tools/cache/AGENTS.md", ".omx/AGENTS.md", "other/AGENTS.md"} {
		writeScopeForFile(t, root, name)
	}
	for _, tt := range []struct {
		excludes []string
		target   string
	}{
		{[]string{"backend"}, "backend/api/new.go"},
		{[]string{"tools/cache"}, "tools/cache/new.go"},
		{[]string{"tools/./cache/"}, "tools/cache/new.go"},
		{[]string{"cache"}, "nested/tools/cache/new.go"},
	} {
		if _, err := ScopeFor(root, tt.excludes, tt.target); err == nil {
			t.Fatalf("excludes=%q target=%q accepted", tt.excludes, tt.target)
		}
	}
	for _, target := range []string{"tools/cache-other", "nested/tools/cache", ".omx", "other"} {
		nodes, err := ScopeFor(root, []string{"tools/cache"}, target)
		if err != nil {
			t.Fatal(err)
		}
		assertScopeForChain(t, nodes, []string{"AGENTS.md", target + "/AGENTS.md"})
	}
	for _, name := range []string{"vendor", "backend"} {
		scanRoot := filepath.Join(t.TempDir(), name)
		writeScopeForFile(t, scanRoot, "AGENTS.md")
		nodes, err := ScopeFor(scanRoot, []string{name}, ".")
		if err != nil {
			t.Fatal(err)
		}
		assertScopeForChain(t, nodes, []string{"AGENTS.md"})
		files, err := DiscoverWithExcludes(scanRoot, []string{name})
		if err != nil || len(files) != 1 || files[0] != filepath.Join(scanRoot, "AGENTS.md") {
			t.Fatalf("root exemption changed: files=%q err=%v", files, err)
		}
	}
}

func TestScopeFor_symlinkPolicy(t *testing.T) {
	root := t.TempDir()
	writeScopeForFile(t, root, "AGENTS.md")
	writeScopeForFile(t, root, "backend/AGENTS.md")
	writeScopeForFile(t, root, "backend/user.go")
	for _, tt := range []struct{ name, destination string }{
		{"file-link", filepath.Join(root, "backend", "user.go")},
		{"directory-link", filepath.Join(root, "backend")},
		{"outside-link", t.TempDir()},
		{"dangling-link", filepath.Join(root, "missing")},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if err := os.Symlink(tt.destination, filepath.Join(root, tt.name)); err != nil {
				t.Skipf("symlinks unavailable: %v", err)
			}
			for _, target := range []string{tt.name, tt.name + "/new.go", tt.name + "/."} {
				if _, err := ScopeFor(root, nil, target); err == nil {
					t.Fatalf("symlink target %q accepted", target)
				}
			}
		})
	}
	link := filepath.Join(t.TempDir(), "project-link")
	if err := os.Symlink(root, link); err != nil {
		t.Skipf("scan-root symlink unavailable: %v", err)
	}
	nodes, err := ScopeFor(link, nil, "backend/user.go")
	if err != nil {
		t.Fatal(err)
	}
	assertScopeForChain(t, nodes, []string{"AGENTS.md", "backend/AGENTS.md"})
}

func TestScopeFor_filesystemSpelling(t *testing.T) {
	root := t.TempDir()
	writeScopeForFile(t, root, "AGENTS.md")
	writeScopeForFile(t, root, "Backend/Api/AGENTS.md")
	writeScopeForFile(t, root, "Backend/Api/User.go")
	_, lookupErr := os.Lstat(filepath.Join(root, "backend", "api"))
	nodes, err := ScopeFor(root, nil, "backend/api/user.go")
	if lookupErr != nil {
		if err == nil {
			t.Fatal("unresolved case spelling accepted")
		}
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	assertScopeForChain(t, nodes, []string{"AGENTS.md", "Backend/Api/AGENTS.md"})
	for _, excludes := range [][]string{{"Backend"}, {"Backend/Api"}} {
		if _, err := ScopeFor(root, excludes, "backend/api/new.go"); err == nil {
			t.Fatalf("case variant bypassed exclusion %q", excludes)
		}
	}
}
