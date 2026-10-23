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
		fmt.Fprintln(stdout, "Usage:\n  instrlint <file-or-directory> [--exclude <directory>]...\n  instrlint scope <directory> [--exclude <directory>]... [--format <tree|json>] [--for <path>]\n\nCommands:\n  scope    Show the discovered AGENTS.md hierarchy.\n\nOptions:\n  --exclude <directory>  Skip a directory during recursive scanning; may be specified multiple times.\n  --format <tree|json>   Output format for scope only. Default: tree.\n  --for <path>           Select discovered ancestors of a scan-root-relative target (scope only).\n  -h, --help             Show help.")
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
		fmt.Fprintln(stdout, "Usage:\n  instrlint scope <directory> [--exclude <directory>]... [--format <tree|json>] [--for <path>]\n\nShow the discovered AGENTS.md hierarchy under a directory.\n\nOptions:\n  --exclude <directory>  Skip a directory during recursive scanning; may be specified multiple times.\n  --format <tree|json>   Output format. Default: tree.\n  --for <path>           Select discovered ancestors of a scan-root-relative target.\n                        Accepts an existing file/directory or a planned file with an existing parent.\n                        Directories include their own AGENTS.md; . selects only the scan root.\n                        Excluded targets and descendant symlinks are errors.\n  -h, --help             Show help.\n\nThis command reports file hierarchy only.\nIt does not lint, merge, or interpret instructions.")
		return 0
	}
	path, excludes, format, target, ok := parseScopeArgs(args)
	if !ok {
		fmt.Fprintln(stderr, "usage: instrlint scope <directory> [--exclude <directory>]... [--format <tree|json>] [--for <path>] (use -h for help)")
		return 2
	}
	var nodes []*instrlint.ScopeNode
	var err error
	if target != "" {
		nodes, err = instrlint.ScopeFor(path, excludes, target)
		stdout = scopeOutputWriter{stdout}
	} else {
		nodes, err = instrlint.Scope(path, excludes)
	}
	if err != nil {
		fmt.Fprintln(stderr, "instrlint:", instrlint.SafeScopeText(err.Error()))
		return 2
	}
	if format == "json" {
		if err := instrlint.RenderScopeJSON(stdout, nodes); err != nil {
			fmt.Fprintln(stderr, "instrlint:", instrlint.SafeScopeText(err.Error()))
			return 2
		}
		return 0
	}
	if len(nodes) == 0 {
		if target == "" {
			fmt.Fprintln(stdout, "no supported instruction files found")
			return 0
		}
		if _, err := fmt.Fprintln(stdout, "no matching instruction files found"); err != nil {
			fmt.Fprintln(stderr, "instrlint: write output:", instrlint.SafeScopeText(err.Error()))
			return 2
		}
		return 0
	}
	if err := instrlint.RenderScope(stdout, nodes); err != nil {
		fmt.Fprintln(stderr, "instrlint: write output:", instrlint.SafeScopeText(err.Error()))
		return 2
	}
	return 0
}

// parseScopeArgs adds scope-only --format and --for options to the lint syntax.
func parseScopeArgs(args []string) (string, []string, string, string, bool) {
	var path, format, target string
	var excludes []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--exclude":
			if i+1 == len(args) {
				return "", nil, "", "", false
			}
			i++
			excludes = append(excludes, args[i])
			continue
		case "--format":
			if i+1 == len(args) || format != "" || args[i+1] != "tree" && args[i+1] != "json" {
				return "", nil, "", "", false
			}
			i++
			format = args[i]
			continue
		case "--for":
			if i+1 == len(args) || target != "" || args[i+1] == "" || strings.HasPrefix(args[i+1], "-") {
				return "", nil, "", "", false
			}
			i++
			target = args[i]
			continue
		}
		if args[i] == "" || strings.HasPrefix(args[i], "-") || path != "" {
			return "", nil, "", "", false
		}
		path = args[i]
	}
	if path == "" {
		return "", nil, "", "", false
	}
	if format == "" {
		format = "tree"
	}
	return path, excludes, format, target, true
}

// Enforce short-write errors for --for output while reusing both renderers.
type scopeOutputWriter struct{ io.Writer }

func (w scopeOutputWriter) Write(p []byte) (int, error) {
	n, err := w.Writer.Write(p)
	if err == nil && n != len(p) {
		err = io.ErrShortWrite
	}
	return n, err
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
