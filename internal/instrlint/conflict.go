package instrlint

import "strings"

type conflictPolarity uint8

const (
	positivePolarity conflictPolarity = iota + 1
	negativePolarity
)

type conflictInstruction struct {
	polarity conflictPolarity
	action   string
	object   string
	scope    string
}

type conflictCandidate struct {
	text     string
	location Location
}

var conflictActions = map[string]bool{
	"add": true, "check": true, "commit": true, "delete": true,
	"edit": true, "execute": true, "follow": true, "include": true,
	"install": true, "keep": true, "modify": true, "remove": true,
	"run": true, "use": true, "write": true,
}

var exclusivePackageManagers = map[string]bool{
	"bun": true, "npm": true, "pnpm": true, "yarn": true,
}

func parseConflictInstruction(text string) (conflictInstruction, bool) {
	normalized := Normalize(text)
	if normalized == "" || strings.Contains(normalized, "`") || !isASCII(normalized) {
		return conflictInstruction{}, false
	}

	polarity := positivePolarity
	core := normalized
	for _, prefix := range []struct {
		text     string
		polarity conflictPolarity
	}{
		{text: "always ", polarity: positivePolarity},
		{text: "never ", polarity: negativePolarity},
		{text: "do not ", polarity: negativePolarity},
		{text: "don't ", polarity: negativePolarity},
		{text: "must not ", polarity: negativePolarity},
		{text: "should not ", polarity: negativePolarity},
		{text: "must ", polarity: positivePolarity},
		{text: "should ", polarity: positivePolarity},
		{text: "do ", polarity: positivePolarity},
	} {
		if strings.HasPrefix(core, prefix.text) {
			polarity = prefix.polarity
			core = strings.TrimSpace(strings.TrimPrefix(core, prefix.text))
			break
		}
	}

	fields := strings.Fields(core)
	if len(fields) < 2 || !conflictActions[fields[0]] {
		return conflictInstruction{}, false
	}

	objectFields := fields[1:]
	var scope string
	for i, field := range objectFields {
		if field == "for" && i > 0 && i+1 < len(objectFields) {
			scope = strings.Join(objectFields[i+1:], " ")
			objectFields = objectFields[:i]
			break
		}
	}
	object := strings.Join(objectFields, " ")
	if object == "" {
		return conflictInstruction{}, false
	}

	return conflictInstruction{
		polarity: polarity,
		action:   fields[0],
		object:   object,
		scope:    scope,
	}, true
}

func isASCII(text string) bool {
	for i := 0; i < len(text); i++ {
		if text[i] >= 0x80 {
			return false
		}
	}
	return true
}

func conflictSemanticKey(instruction conflictInstruction) string {
	return instruction.action + "\x00" + instruction.object + "\x00" + instruction.scope
}

func conflictScopeKey(instruction conflictInstruction) string {
	return instruction.action + "\x00" + instruction.scope
}

func Conflicts(path string, instructions []Instruction) []Diagnostic {
	positive := make(map[string]conflictCandidate, len(instructions))
	negative := make(map[string]conflictCandidate, len(instructions))
	exclusive := make(map[string]struct {
		object    string
		candidate conflictCandidate
	})
	var diagnostics []Diagnostic

	for _, instruction := range instructions {
		parsed, ok := parseConflictInstruction(instruction.Text)
		if !ok {
			continue
		}
		key := conflictSemanticKey(parsed)
		candidate := conflictCandidate{
			text: instruction.Text,
			location: Location{
				File: path, Line: instruction.Line, Column: instruction.Column,
			},
		}

		current, opposite := positive, negative
		if parsed.polarity == negativePolarity {
			current, opposite = negative, positive
		}
		if _, exists := current[key]; exists {
			continue
		}
		if related, exists := opposite[key]; exists {
			diagnostics = append(diagnostics, conflictingDiagnostic(candidate, related))
		}
		current[key] = candidate

		if parsed.polarity != positivePolarity || parsed.action != "use" || parsed.scope == "" ||
			!exclusivePackageManagers[parsed.object] {
			continue
		}
		scopeKey := conflictScopeKey(parsed)
		if related, exists := exclusive[scopeKey]; exists {
			if related.object != parsed.object {
				diagnostics = append(diagnostics, conflictingDiagnostic(candidate, related.candidate))
			}
			continue
		}
		exclusive[scopeKey] = struct {
			object    string
			candidate conflictCandidate
		}{object: parsed.object, candidate: candidate}
	}

	return diagnostics
}

func conflictingDiagnostic(current, related conflictCandidate) Diagnostic {
	return Diagnostic{
		Rule:     "conflicting-instruction",
		Severity: Warning,
		Message:  "Conflicting instruction: " + current.text,
		Location: current.location,
		Related:  related.location,
	}
}
