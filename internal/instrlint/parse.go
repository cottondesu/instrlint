package instrlint

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

type Instruction struct {
	Text   string
	Line   int
	Column int
}

func Parse(data []byte) ([]Instruction, error) {
	if !utf8.Valid(data) {
		return nil, fmt.Errorf("invalid UTF-8")
	}
	var instructions []Instruction
	var fence byte
	var fenceLength int
	var fenceIndent int
	var inComment bool
	var inList bool
	var listIndent int
	var current Instruction
	flush := func() {
		if current.Text != "" {
			instructions = append(instructions, current)
			current = Instruction{}
		}
	}
	lines := strings.Split(string(data), "\n")
	var listContentIndent int
	for i, raw := range lines {
		line := strings.TrimSuffix(raw, "\r")
		indent := len(line) - len(strings.TrimLeft(line, " \t"))
		trimmed := strings.TrimSpace(line)
		if fence != 0 {
			if marker, length := fenceMarker(trimmed); marker == fence && length >= fenceLength &&
				((fenceIndent <= 3 && indent <= 3) || (fenceIndent > 3 && indent >= fenceIndent && indent <= fenceIndent+3)) &&
				strings.TrimSpace(trimmed[length:]) == "" {
				fence = 0
			}
			continue
		}
		line, inComment = stripHTMLComments(line, inComment)
		trimmed = strings.TrimSpace(line)
		if trimmed == "" {
			flush()
			inList = false
			continue
		}
		if marker, length := fenceMarker(trimmed); marker != 0 && (indent <= 3 || inList) {
			flush()
			inList = false
			fence, fenceLength, fenceIndent = marker, length, indent
			continue
		}
		if strings.HasPrefix(trimmed, "#") || isHorizontalRule(trimmed) ||
			strings.HasPrefix(trimmed, ">") || strings.HasPrefix(trimmed, "|") ||
			(strings.HasPrefix(trimmed, "`") && strings.HasSuffix(trimmed, "`")) {
			flush()
			inList = false
			continue
		}
		if item, markerWidth, ok := stripListMarker(trimmed); ok && (indent <= 3 || inList && indent <= listContentIndent+3) {
			flush()
			inList, listIndent = true, indent
			listContentIndent = indent + markerWidth
			if item != "" && !strings.HasSuffix(item, ":") &&
				!(strings.HasPrefix(item, "`") && strings.HasSuffix(item, "`")) {
				current = Instruction{Text: item, Line: i + 1, Column: indent + 1}
			}
			continue
		}
		if inList && current.Text != "" && indent > listIndent {
			current.Text += " " + trimmed
			continue
		}
		flush()
		inList = false
		if indent >= 4 {
			continue
		}
		if isInstructionParagraph(trimmed) && !nextLineIsSetextUnderline(lines, i) {
			instructions = append(instructions, Instruction{Text: trimmed, Line: i + 1, Column: indent + 1})
		}
	}
	flush()
	return instructions, nil
}

func stripHTMLComments(line string, inComment bool) (string, bool) {
	if !inComment && !strings.Contains(line, "<!--") {
		return line, false
	}
	var visible strings.Builder
	for line != "" {
		if inComment {
			end := strings.Index(line, "-->")
			if end < 0 {
				return visible.String(), true
			}
			line = line[end+3:]
			inComment = false
			continue
		}
		start := strings.Index(line, "<!--")
		if start < 0 {
			visible.WriteString(line)
			break
		}
		visible.WriteString(line[:start])
		line = line[start+4:]
		inComment = true
	}
	return visible.String(), inComment
}

func nextLineIsSetextUnderline(lines []string, i int) bool {
	if i+1 >= len(lines) {
		return false
	}
	next := strings.TrimSuffix(lines[i+1], "\r")
	if len(next)-len(strings.TrimLeft(next, " \t")) > 3 {
		return false
	}
	underline := strings.TrimSpace(next)
	return underline != "" && (strings.Trim(underline, "=") == "" || strings.Trim(underline, "-") == "")
}

func fenceMarker(line string) (byte, int) {
	if line == "" {
		return 0, 0
	}
	if line[0] != '`' && line[0] != '~' {
		return 0, 0
	}
	i := 0
	for i < len(line) && line[i] == line[0] {
		i++
	}
	if i < 3 {
		return 0, 0
	}
	return line[0], i
}

func isHorizontalRule(line string) bool {
	compact := strings.ReplaceAll(strings.ReplaceAll(line, " ", ""), "\t", "")
	if len(compact) < 3 {
		return false
	}
	for _, marker := range []byte{'-', '*', '_'} {
		if strings.Trim(compact, string(marker)) == "" {
			return true
		}
	}
	return false
}

func stripListMarker(line string) (string, int, bool) {
	if len(line) >= 2 && (line[0] == '-' || line[0] == '*' || line[0] == '+') && line[1] == ' ' {
		body := line[2:]
		return strings.TrimSpace(body), len(line) - len(strings.TrimLeft(body, " \t")), true
	}
	i := 0
	for i < len(line) && line[i] >= '0' && line[i] <= '9' {
		i++
	}
	if i > 0 && i+1 < len(line) && (line[i] == '.' || line[i] == ')') && line[i+1] == ' ' {
		body := line[i+2:]
		return strings.TrimSpace(body), len(line) - len(strings.TrimLeft(body, " \t")), true
	}
	return line, 0, false
}

func isInstructionParagraph(line string) bool {
	lower := asciiLower(line)
	for _, prefix := range []string{
		"always ", "never ", "do ", "don't ", "use ", "run ",
		"avoid ", "prefer ", "ensure ", "keep ", "must ",
		"should ", "check ", "write ", "follow ", "include ",
		"add ", "commit ", "delete ", "edit ", "execute ",
		"install ", "modify ", "remove ",
	} {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return strings.HasPrefix(line, "必ず") || strings.HasPrefix(line, "禁止") ||
		strings.HasSuffix(line, "してください。") || strings.HasSuffix(line, "してください") ||
		strings.HasSuffix(line, "すること。") || strings.HasSuffix(line, "すること") ||
		strings.HasSuffix(line, "してはいけません。")
}
