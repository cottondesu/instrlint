package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLIScopeFor_windowsNamespaceCollision(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "vendor"), 0755); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"AGENTS.md", "vendor/AGENTS.md", "vendor/file.go"} {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(path)), nil, 0644); err != nil {
			t.Fatal(err)
		}
	}
	extendedRoot := `\\?\` + root
	if strings.HasPrefix(root, `\\`) {
		extendedRoot = `\\?\UNC\` + strings.TrimPrefix(root, `\\`)
	}
	collision := filepath.Join(extendedRoot, "vendor.")
	if err := os.WriteFile(collision, nil, 0644); err != nil {
		t.Skipf("cannot create literal vendor. through the extended Windows namespace: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Remove(collision); err != nil {
			t.Errorf("remove literal namespace collision: %v", err)
		}
	})
	vendorInfo, err := os.Lstat(filepath.Join(root, "vendor"))
	if err != nil {
		t.Fatal(err)
	}
	literalInfo, err := os.Lstat(collision)
	if err != nil || !literalInfo.Mode().IsRegular() || os.SameFile(vendorInfo, literalInfo) {
		t.Fatalf("namespace fixture did not create a distinct regular file: info=%v err=%v", literalInfo, err)
	}
	ordinaryInfo, lookupErr := os.Lstat(filepath.Join(root, "vendor."))
	literalTarget := lookupErr == nil && os.SameFile(ordinaryInfo, literalInfo)
	if literalTarget {
		t.Log("ordinary lookup resolves the literal regular file; excluded-directory alias is not reproduced")
	} else if lookupErr == nil && !os.SameFile(ordinaryInfo, vendorInfo) {
		t.Fatalf("ordinary lookup resolved an unexpected object: %v", ordinaryInfo)
	}
	for _, target := range []string{"vendor.", "vendor", "vendor/file.go"} {
		for _, format := range []string{"tree", "json"} {
			t.Run(target+"/"+format, func(t *testing.T) {
				wantCode, wantOutput := 2, ""
				if target == "vendor." && literalTarget {
					wantCode, wantOutput = 0, "AGENTS.md\n"
					if format == "json" {
						wantOutput = `{"schema_version":1,"entries":[{"path":"AGENTS.md","parent":null}]}` + "\n"
					}
				}
				var stdout, stderr bytes.Buffer

				code := run([]string{"scope", root, "--for", target, "--format", format}, &stdout, &stderr)

				if code != wantCode || stdout.String() != wantOutput ||
					wantCode == 0 && stderr.Len() != 0 || wantCode == 2 && stderr.Len() == 0 {
					t.Fatalf("code=%d stdout=%q stderr=%q wantCode=%d wantOutput=%q", code, stdout.String(), stderr.String(), wantCode, wantOutput)
				}
			})
		}
	}
}
