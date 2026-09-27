package instrlint

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"unicode/utf8"
)

type ScopeNode struct {
	Path     string
	Children []*ScopeNode
}

func Scope(path string, excludes []string) ([]*ScopeNode, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("access %q: %w", path, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("scope target must be a directory: %s", path)
	}
	files, err := DiscoverWithExcludes(path, excludes)
	if err != nil {
		return nil, err
	}
	root, err := discoveryRoot(path)
	if err != nil {
		return nil, err
	}
	return buildScopeHierarchy(root, files)
}

func buildScopeHierarchy(root string, files []string) ([]*ScopeNode, error) {
	byDirectory := make(map[string]*ScopeNode, len(files))
	for _, file := range files {
		relative, err := filepath.Rel(root, file)
		if err != nil {
			return nil, fmt.Errorf("relative path for %q: %w", file, err)
		}
		byDirectory[filepath.Dir(relative)] = &ScopeNode{Path: filepath.ToSlash(relative)}
	}

	var roots []*ScopeNode
	for directory, node := range byDirectory {
		if directory == "." {
			roots = append(roots, node)
			continue
		}
		for ancestor := filepath.Dir(directory); ; ancestor = filepath.Dir(ancestor) {
			if parent, found := byDirectory[ancestor]; found {
				parent.Children = append(parent.Children, node)
				break
			}
			if ancestor == "." {
				roots = append(roots, node)
				break
			}
		}
	}

	var sortNodes func([]*ScopeNode)
	sortNodes = func(nodes []*ScopeNode) {
		sort.Slice(nodes, func(i, j int) bool { return nodes[i].Path < nodes[j].Path })
		for _, node := range nodes {
			sortNodes(node.Children)
		}
	}
	sortNodes(roots)
	return roots, nil
}

func RenderScope(output io.Writer, roots []*ScopeNode) error {
	var renderChildren func([]*ScopeNode, string) error
	renderChildren = func(children []*ScopeNode, prefix string) error {
		for i, child := range children {
			last := i == len(children)-1
			branch, continuation := "├── ", "│   "
			if last {
				branch, continuation = "└── ", "    "
			}
			if _, err := fmt.Fprintln(output, prefix+branch+SafeScopeText(child.Path)); err != nil {
				return err
			}
			if err := renderChildren(child.Children, prefix+continuation); err != nil {
				return err
			}
		}
		return nil
	}
	for _, root := range roots {
		if _, err := fmt.Fprintln(output, SafeScopeText(root.Path)); err != nil {
			return err
		}
		if err := renderChildren(root.Children, ""); err != nil {
			return err
		}
	}
	return nil
}

func SafeScopeText(value string) string {
	if !utf8.ValidString(value) {
		return strconv.QuoteToGraphic(value)
	}
	for _, r := range value {
		if !strconv.IsGraphic(r) {
			return strconv.QuoteToGraphic(value)
		}
	}
	return value
}
