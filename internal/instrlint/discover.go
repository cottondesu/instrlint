package instrlint

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
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
	rules, err := newDirectoryExclusions(excludes)
	if err != nil {
		return nil, err
	}

	root, err := discoveryRoot(path)
	if err != nil {
		return nil, err
	}

	var files []string
	err = filepath.WalkDir(root, func(current string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if current != root {
				relative, err := filepath.Rel(root, current)
				if err != nil {
					return err
				}
				if rules.matches(relative) {
					return filepath.SkipDir
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

func discoveryRoot(path string) (string, error) {
	if info, err := os.Lstat(path); err == nil && info.Mode()&os.ModeSymlink != 0 {
		root, err := filepath.EvalSymlinks(path)
		if err != nil {
			return "", fmt.Errorf("resolve %q: %w", path, err)
		}
		return root, nil
	}
	return path, nil
}
