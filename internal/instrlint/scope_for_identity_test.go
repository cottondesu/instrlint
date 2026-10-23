package instrlint

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScopeComponentName_identity(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "vendor"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "vendor-alias"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	for _, tt := range []struct {
		name, parent, component, object, want string
		wantError                             bool
	}{
		{"mismatched exact name", root, "vendor-alias", filepath.Join(root, "vendor"), "vendor", false},
		{"exact directory", root, "vendor", filepath.Join(root, "vendor"), "vendor", false},
		{"exact regular file", root, "vendor-alias", filepath.Join(root, "vendor-alias"), "vendor-alias", false},
		{"unresolved identity", root, "vendor-alias", outside, "", true},
		{"missing parent", filepath.Join(root, "missing"), "vendor", filepath.Join(root, "vendor"), "", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			info, err := os.Lstat(tt.object)
			if err != nil {
				t.Fatal(err)
			}

			name, err := scopeComponentName(tt.parent, tt.component, info)

			if (err != nil) != tt.wantError || name != tt.want {
				t.Fatalf("name=%q err=%v want=%q wantError=%v", name, err, tt.want, tt.wantError)
			}
		})
	}
}
