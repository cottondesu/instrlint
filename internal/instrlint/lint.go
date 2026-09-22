package instrlint

import (
	"fmt"
	"os"
	"strings"
)

type Location struct {
	File   string
	Line   int
	Column int
}

type Severity string

const Warning Severity = "warning"

type Diagnostic struct {
	Rule     string
	Severity Severity
	Message  string
	Location
	Related Location
}

func Normalize(text string) string {
	text = strings.TrimSpace(text)
	if item, ok := stripListMarker(text); ok {
		text = item
	}
	text = strings.Join(strings.Fields(text), " ")
	text = strings.TrimRight(text, ".。!！")
	text = strings.TrimSpace(text)
	if strings.IndexByte(text, '`') < 0 {
		return asciiLower(text)
	}
	return asciiLowerOutsideCode(text)
}

func asciiLowerOutsideCode(text string) string {
	var lowered []byte
	var codeDelimiter int
	for i := 0; i < len(text); i++ {
		if text[i] == '`' {
			start := i
			for i+1 < len(text) && text[i+1] == '`' {
				i++
			}
			length := i - start + 1
			if codeDelimiter == 0 {
				codeDelimiter = length
			} else if codeDelimiter == length {
				codeDelimiter = 0
			}
			continue
		}
		if codeDelimiter == 0 && text[i] >= 'A' && text[i] <= 'Z' {
			if lowered == nil {
				lowered = []byte(text)
			}
			lowered[i] = text[i] + ('a' - 'A')
		}
	}
	if lowered == nil {
		return text
	}
	return string(lowered)
}

func asciiLower(text string) string {
	var changed bool
	for i := 0; i < len(text); i++ {
		if text[i] >= 'A' && text[i] <= 'Z' {
			changed = true
			break
		}
	}
	if !changed {
		return text
	}
	bytes := []byte(text)
	for i, c := range bytes {
		if c >= 'A' && c <= 'Z' {
			bytes[i] = c + ('a' - 'A')
		}
	}
	return string(bytes)
}

func Duplicates(path string, instructions []Instruction) []Diagnostic {
	seen := make(map[string]Location, len(instructions))
	var diagnostics []Diagnostic
	for _, instruction := range instructions {
		key := Normalize(instruction.Text)
		if key == "" {
			continue
		}
		location := Location{File: path, Line: instruction.Line, Column: instruction.Column}
		if first, ok := seen[key]; ok {
			diagnostics = append(diagnostics, Diagnostic{
				Rule: "duplicate-instruction", Severity: Warning,
				Message:  "Duplicate instruction: " + instruction.Text,
				Location: location, Related: first,
			})
		} else {
			seen[key] = location
		}
	}
	return diagnostics
}

type Result struct {
	Files       int
	Diagnostics []Diagnostic
}

func Lint(path string) (Result, error) {
	files, err := Discover(path)
	if err != nil {
		return Result{}, err
	}
	var diagnostics []Diagnostic
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return Result{}, fmt.Errorf("read %q: %w", file, err)
		}
		instructions, err := Parse(data)
		if err != nil {
			return Result{}, fmt.Errorf("parse %q: %w", file, err)
		}
		diagnostics = append(diagnostics, Duplicates(file, instructions)...)
	}
	return Result{Files: len(files), Diagnostics: diagnostics}, nil
}
