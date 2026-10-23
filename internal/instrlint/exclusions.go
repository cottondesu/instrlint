package instrlint

import (
	"fmt"
	"path/filepath"
	"strings"
)

type directoryExclusions struct {
	names map[string]bool
	paths map[string]bool
}

func newDirectoryExclusions(excludes []string) (directoryExclusions, error) {
	rules := directoryExclusions{names: make(map[string]bool), paths: make(map[string]bool)}
	for _, exclude := range excludes {
		isPath := strings.ContainsRune(exclude, '/') || strings.ContainsRune(exclude, filepath.Separator)
		clean := filepath.Clean(exclude)
		if exclude == "" || clean == "." || clean == ".." || filepath.IsAbs(clean) ||
			filepath.VolumeName(clean) != "" || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return directoryExclusions{}, fmt.Errorf("invalid exclude %q: expected a directory name or scan-root-relative path", exclude)
		}
		if isPath {
			rules.paths[clean] = true
		} else {
			rules.names[clean] = true
		}
	}
	return rules, nil
}

func (rules directoryExclusions) matches(relative string) bool {
	if relative == "." {
		return false
	}
	name := filepath.Base(relative)
	return excludedDirectories[name] || rules.names[name] || rules.paths[relative]
}
