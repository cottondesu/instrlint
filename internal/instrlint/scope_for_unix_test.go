//go:build unix

package instrlint

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestScopeFor_specialFile(t *testing.T) {
	root := t.TempDir()
	if err := syscall.Mkfifo(filepath.Join(root, "pipe"), 0600); err != nil {
		t.Skipf("FIFO unavailable: %v", err)
	}
	if _, err := ScopeFor(root, nil, "pipe"); err == nil {
		t.Fatal("special file accepted")
	}
}

func TestScopeFor_permissionFailure(t *testing.T) {
	root := t.TempDir()
	writeScopeForFile(t, root, "locked/AGENTS.md")
	directory := filepath.Join(root, "locked")
	if err := os.Chmod(directory, 0000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(directory, 0755); err != nil {
			t.Error(err)
		}
	})
	if _, err := os.ReadDir(directory); err == nil {
		t.Skip("current user can read mode-000 directories")
	}
	if _, err := ScopeFor(root, nil, "locked/new.go"); !errors.Is(err, os.ErrPermission) {
		t.Fatalf("err=%v, want permission error", err)
	}
}
