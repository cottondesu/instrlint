package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/cottondesu/instrlint/internal/instrlint"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "scope" {
		return runScope(args[1:], stdout, stderr)
	}
	if len(args) == 1 && (args[0] == "-h" || args[0] == "--help") {
		fmt.Fprintln(stdout, "Usage:\n  instrlint <file-or-directory> [--exclude <directory>]...\n  instrlint scope <directory> [--exclude <directory>]...\n\nCommands:\n  scope    Show the discovered AGENTS.md hierarchy.\n\nOptions:\n  --exclude <directory>  Skip a directory during recursive scanning; may be specified multiple times.\n  -h, --help             Show help.")
		return 0
	}
	path, excludes, ok := parseArgs(args)
	if !ok {
		fmt.Fprintln(stderr, "usage: instrlint <file-or-directory> [--exclude <directory>]... (use -h for help)")
		return 2
	}

	result, err := instrlint.LintWithExcludes(path, excludes)
	if err != nil {
		fmt.Fprintln(stderr, "instrlint:", err)
		return 2
	}
	if result.Files == 0 {
		fmt.Fprintln(stdout, "no supported instruction files found")
		return 0
	}
	if err := instrlint.Report(stdout, result.Diagnostics); err != nil {
		fmt.Fprintln(stderr, "instrlint: write output:", err)
		return 2
	}
	if len(result.Diagnostics) > 0 {
		return 1
	}
	return 0
}

func runScope(args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 && (args[0] == "-h" || args[0] == "--help") {
		fmt.Fprintln(stdout, "Usage:\n  instrlint scope <directory> [--exclude <directory>]...\n\nShow the discovered AGENTS.md hierarchy under a directory.\n\nThis command reports file hierarchy only.\nIt does not lint, merge, or interpret instructions.")
		return 0
	}
	path, excludes, ok := parseArgs(args)
	if !ok {
		fmt.Fprintln(stderr, "usage: instrlint scope <directory> [--exclude <directory>]... (use -h for help)")
		return 2
	}
	nodes, err := instrlint.Scope(path, excludes)
	if err != nil {
		fmt.Fprintln(stderr, "instrlint:", instrlint.SafeScopeText(err.Error()))
		return 2
	}
	if len(nodes) == 0 {
		fmt.Fprintln(stdout, "no supported instruction files found")
		return 0
	}
	if err := instrlint.RenderScope(stdout, nodes); err != nil {
		fmt.Fprintln(stderr, "instrlint: write output:", err)
		return 2
	}
	return 0
}

func parseArgs(args []string) (string, []string, bool) {
	var path string
	var excludes []string
	for i := 0; i < len(args); i++ {
		if args[i] == "--exclude" {
			if i+1 == len(args) {
				return "", nil, false
			}
			i++
			excludes = append(excludes, args[i])
			continue
		}
		if args[i] == "" || strings.HasPrefix(args[i], "-") || path != "" {
			return "", nil, false
		}
		path = args[i]
	}
	if path == "" {
		return "", nil, false
	}
	return path, excludes, true
}
