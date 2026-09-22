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
	if len(args) == 1 && (args[0] == "-h" || args[0] == "--help") {
		fmt.Fprintln(stdout, "Usage: instrlint <file-or-directory> [--exclude <directory>]...\nLint AGENTS.md files for duplicate instructions.\n--exclude <directory>  Skip a directory during recursive scanning; may be specified multiple times.")
		return 0
	}
	var path string
	var excludes []string
	for i := 0; i < len(args); i++ {
		if args[i] == "--exclude" {
			if i+1 == len(args) {
				fmt.Fprintln(stderr, "usage: instrlint <file-or-directory> [--exclude <directory>]... (use -h for help)")
				return 2
			}
			i++
			excludes = append(excludes, args[i])
			continue
		}
		if args[i] == "" || strings.HasPrefix(args[i], "-") || path != "" {
			fmt.Fprintln(stderr, "usage: instrlint <file-or-directory> [--exclude <directory>]... (use -h for help)")
			return 2
		}
		path = args[i]
	}
	if path == "" {
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
