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
		t.Logf("FIXTURE_ERROR mkdir vendor: %v", err)
		t.Fatal(err)
	}
	t.Logf("FIXTURE_CREATED directory=%q", filepath.Join(root, "vendor"))
	for _, path := range []string{"AGENTS.md", "vendor/AGENTS.md", "vendor/file.go"} {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(path)), nil, 0644); err != nil {
			t.Logf("FIXTURE_ERROR write %q: %v", path, err)
			t.Fatal(err)
		}
		t.Logf("FIXTURE_CREATED file=%q", filepath.Join(root, filepath.FromSlash(path)))
	}
	extendedRoot := `\\?\` + root
	if strings.HasPrefix(root, `\\`) {
		extendedRoot = `\\?\UNC\` + strings.TrimPrefix(root, `\\`)
	}
	collision := filepath.Join(extendedRoot, "vendor.")
	if err := os.WriteFile(collision, nil, 0644); err != nil {
		t.Logf("FIXTURE_SKIP reason=cannot create literal vendor. through the extended Windows namespace: %v", err)
		t.Skipf("cannot create literal vendor. through the extended Windows namespace: %v", err)
	}
	t.Logf("FIXTURE_CREATED literal_collision=%q", collision)
	t.Cleanup(func() {
		if err := os.Remove(collision); err != nil {
			t.Errorf("remove literal namespace collision: %v", err)
		}
	})
	vendorInfo, err := os.Lstat(filepath.Join(root, "vendor"))
	if err != nil {
		t.Logf("FIXTURE_ERROR lstat vendor: %v", err)
		t.Fatal(err)
	}
	literalInfo, err := os.Lstat(collision)
	if err != nil || !literalInfo.Mode().IsRegular() || os.SameFile(vendorInfo, literalInfo) {
		t.Logf("FIXTURE_ERROR collision identity validation: info=%v err=%v same_file=%t", literalInfo, err, err == nil && os.SameFile(vendorInfo, literalInfo))
		t.Fatalf("namespace fixture did not create a distinct regular file: info=%v err=%v", literalInfo, err)
	}
	t.Logf("OBJECTS_DISTINCT vendor=%q collision=%q same_file=%t", filepath.Join(root, "vendor"), collision, os.SameFile(vendorInfo, literalInfo))
	ordinaryInfo, lookupErr := os.Lstat(filepath.Join(root, "vendor."))
	literalTarget := lookupErr == nil && os.SameFile(ordinaryInfo, literalInfo)
	if literalTarget {
		t.Log("ordinary lookup resolves the literal regular file; excluded-directory alias is not reproduced")
	} else if lookupErr == nil && !os.SameFile(ordinaryInfo, vendorInfo) {
		t.Fatalf("ordinary lookup resolved an unexpected object: %v", ordinaryInfo)
	}
	t.Logf("ORDINARY_LOOKUP=%s lookup_error=%v same_vendor_directory=%t same_literal_file=%t", map[[2]bool]string{{true, false}: "ERROR", {false, true}: "LITERAL_FILE", {false, false}: "EXCLUDED_VENDOR_ALIAS"}[[2]bool{lookupErr != nil, literalTarget}], lookupErr, lookupErr == nil && os.SameFile(ordinaryInfo, vendorInfo), lookupErr == nil && os.SameFile(ordinaryInfo, literalInfo))
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
				t.Logf("CLI_RESULT target=%q format=%q exit_code=%d stdout=%q stderr=%q", target, format, code, stdout.String(), stderr.String())
			})
		}
	}
}
