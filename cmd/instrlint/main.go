package main

import (
	"fmt"
	"io"
	"os"

	"github.com/cottondesu/instrlint/internal/instrlint"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 && (args[0] == "-h" || args[0] == "--help") {
		fmt.Fprintln(stdout, "Usage: instrlint <file-or-directory>\nLint AGENTS.md files for duplicate instructions.")
		return 0
	}
	if len(args) != 1 || args[0] == "" || args[0][0] == '-' {
		fmt.Fprintln(stderr, "usage: instrlint <file-or-directory> (use -h for help)")
		return 2
	}

	result, err := instrlint.Lint(args[0])
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
