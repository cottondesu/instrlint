package instrlint

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var excludedDirectories = map[string]bool{
	".git": true, "node_modules": true, "vendor": true,
	"dist": true, "build": true, "out": true,
	"coverage": true, "tmp": true, ".cache": true,
}

func Discover(path string) ([]string, error) {
	return DiscoverWithExcludes(path, nil)
}

func DiscoverWithExcludes(path string, excludes []string) ([]string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("access %q: %w", path, err)
	}
	if !info.IsDir() {
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("%q is not a regular file", path)
		}
		return []string{path}, nil
	}
	excludedNames := make(map[string]bool)
	excludedPaths := make(map[string]bool)
	for _, exclude := range excludes {
		isPath := strings.ContainsRune(exclude, '/') || strings.ContainsRune(exclude, filepath.Separator)
		clean := filepath.Clean(exclude)
		if exclude == "" || clean == "." || clean == ".." || filepath.IsAbs(clean) ||
			filepath.VolumeName(clean) != "" || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return nil, fmt.Errorf("invalid exclude %q: expected a directory name or scan-root-relative path", exclude)
		}
		if isPath {
			excludedPaths[clean] = true
		} else {
			excludedNames[clean] = true
		}
	}

	root := path
	if info, err := os.Lstat(path); err == nil && info.Mode()&os.ModeSymlink != 0 {
		root, err = filepath.EvalSymlinks(path)
		if err != nil {
			return nil, fmt.Errorf("resolve %q: %w", path, err)
		}
	}

	var files []string
	err = filepath.WalkDir(root, func(current string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if current != root {
				if excludedDirectories[entry.Name()] || excludedNames[entry.Name()] {
					return filepath.SkipDir
				}
				if len(excludedPaths) > 0 {
					relative, err := filepath.Rel(root, current)
					if err != nil {
						return err
					}
					if excludedPaths[relative] {
						return filepath.SkipDir
					}
				}
			}
			return nil
		}
		if entry.Type().IsRegular() && entry.Name() == "AGENTS.md" {
			files = append(files, current)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("discover in %q: %w", path, err)
	}
	sort.Strings(files)
	return files, nil
}
