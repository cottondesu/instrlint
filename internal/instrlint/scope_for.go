package instrlint

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ScopeFor selects discovered AGENTS.md files on the target's directory
// ancestry. The target is relative to the scan root, not the working directory.
// It reports filesystem ancestry only, without interpreting file contents.
func ScopeFor(root string, excludes []string, target string) ([]*ScopeNode, error) {
	nodes, err := Scope(root, excludes)
	if err != nil {
		return nil, err
	}
	root, err = discoveryRoot(root)
	if err != nil {
		return nil, err
	}
	rules, err := newDirectoryExclusions(excludes)
	if err != nil {
		return nil, err
	}
	directory, err := scopeTargetDirectory(root, rules, target)
	if err != nil {
		return nil, fmt.Errorf("scope target %q: %w", target, err)
	}
	return selectScopeChain(nodes, directory), nil
}

func scopeTargetDirectory(root string, rules directoryExclusions, target string) (string, error) {
	if target == "" || os.IsPathSeparator(target[0]) || filepath.IsAbs(target) || filepath.VolumeName(target) != "" {
		return "", fmt.Errorf("expected a scan-root-relative path")
	}
	components := strings.FieldsFunc(target, func(r rune) bool {
		return r == '/' || r == rune(filepath.Separator)
	})
	for _, component := range components {
		if component == ".." {
			return "", fmt.Errorf("parent traversal is not allowed")
		}
	}
	directoryStyle := os.IsPathSeparator(target[len(target)-1]) ||
		len(components) > 0 && components[len(components)-1] == "."
	clean := filepath.Clean(target)
	if clean == "." {
		return ".", nil
	}
	components = strings.Split(clean, string(filepath.Separator))
	relative := "."
	for i, component := range components {
		parent := filepath.Join(root, relative)
		candidate := filepath.Join(parent, component)
		info, err := os.Lstat(candidate)
		last := i == len(components)-1
		if os.IsNotExist(err) && last && !directoryStyle {
			return relative, nil
		}
		if err != nil {
			return "", fmt.Errorf("access %q: %w", candidate, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("symlink components are not supported")
		}
		if !info.IsDir() && (!last || directoryStyle || !info.Mode().IsRegular()) {
			return "", fmt.Errorf("expected a directory or regular file")
		}
		name, err := scopeComponentName(parent, component, info)
		if err != nil {
			return "", err
		}
		if !info.IsDir() {
			return relative, nil
		}
		relative = filepath.Join(relative, name)
		if rules.matches(relative) {
			return "", fmt.Errorf("target is inside an excluded directory")
		}
	}
	return relative, nil
}

// Resolve the directory-entry spelling used by discovery. On case-insensitive
// filesystems Lstat alone may retain the input spelling; identity handles that
// case (and filesystem Unicode normalization) without guessing case rules.
func scopeComponentName(parent, component string, info os.FileInfo) (string, error) {
	entries, err := os.ReadDir(parent)
	if err != nil {
		return "", fmt.Errorf("read target ancestor %q: %w", parent, err)
	}
	for _, entry := range entries {
		if entry.Name() == component {
			actual, err := entry.Info()
			if err != nil {
				return "", fmt.Errorf("inspect target ancestor entry: %w", err)
			}
			if os.SameFile(info, actual) {
				return entry.Name(), nil
			}
		}
	}
	for _, entry := range entries {
		actual, err := entry.Info()
		if err != nil {
			return "", fmt.Errorf("inspect target ancestor entry: %w", err)
		}
		if os.SameFile(info, actual) {
			return entry.Name(), nil
		}
	}
	return "", fmt.Errorf("cannot resolve the filesystem spelling of %q", component)
}

func selectScopeChain(nodes []*ScopeNode, directory string) []*ScopeNode {
	byDirectory := make(map[string]*ScopeNode)
	var index func([]*ScopeNode)
	index = func(nodes []*ScopeNode) {
		for _, node := range nodes {
			byDirectory[filepath.Dir(filepath.FromSlash(node.Path))] = node
			index(node.Children)
		}
	}
	index(nodes)
	var chain []*ScopeNode
	for ancestor := directory; ; ancestor = filepath.Dir(ancestor) {
		if node := byDirectory[ancestor]; node != nil {
			clone := &ScopeNode{Path: node.Path, Children: chain}
			chain = []*ScopeNode{clone}
		}
		if ancestor == "." {
			return chain
		}
	}
}
