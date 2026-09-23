package instrlint

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParseConflictInstruction_extractsPolarityActionObjectAndScope(t *testing.T) {
	tests := []struct {
		name string
		text string
		want conflictInstruction
	}{
		{
			name: "always is positive",
			text: "Always use npm for frontend dependencies.",
			want: conflictInstruction{polarity: positivePolarity, action: "use", object: "npm", scope: "frontend dependencies"},
		},
		{
			name: "never is negative",
			text: "Never use npm for frontend dependencies.",
			want: conflictInstruction{polarity: negativePolarity, action: "use", object: "npm", scope: "frontend dependencies"},
		},
		{
			name: "explicit prohibition is negative",
			text: "Do not edit generated files directly.",
			want: conflictInstruction{polarity: negativePolarity, action: "edit", object: "generated files directly"},
		},
		{
			name: "contracted prohibition is negative",
			text: "Don't edit generated files directly.",
			want: conflictInstruction{polarity: negativePolarity, action: "edit", object: "generated files directly"},
		},
		{
			name: "bare imperative is positive",
			text: "Edit generated files directly.",
			want: conflictInstruction{polarity: positivePolarity, action: "edit", object: "generated files directly"},
		},
		{
			name: "must not is negative",
			text: "Must not run tests before committing.",
			want: conflictInstruction{polarity: negativePolarity, action: "run", object: "tests before committing"},
		},
		{
			name: "should not is negative",
			text: "Should not remove tests.",
			want: conflictInstruction{polarity: negativePolarity, action: "remove", object: "tests"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When
			got, ok := parseConflictInstruction(tt.text)

			// Then
			if !ok || !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("parseConflictInstruction(%q) = %#v, %t, want %#v, true", tt.text, got, ok, tt.want)
			}
		})
	}
}

func TestParseConflictInstruction_rejectsUnsupportedText(t *testing.T) {
	for _, text := range []string{
		"Prefer npm.",
		"必ず npm を使用してください。",
		"Use `npm` for frontend dependencies.",
	} {
		t.Run(text, func(t *testing.T) {
			// When
			_, ok := parseConflictInstruction(text)

			// Then
			if ok {
				t.Fatalf("parseConflictInstruction(%q) unexpectedly succeeded", text)
			}
		})
	}
}

func TestConflicts_detectsHighConfidenceOpposites(t *testing.T) {
	tests := []struct {
		name         string
		instructions []Instruction
	}{
		{
			name: "always versus never",
			instructions: []Instruction{
				{Text: "Always use npm.", Line: 1, Column: 1},
				{Text: "Never use npm.", Line: 2, Column: 1},
			},
		},
		{
			name: "positive versus explicit prohibition",
			instructions: []Instruction{
				{Text: "Edit generated files directly.", Line: 3, Column: 1},
				{Text: "Do not edit generated files directly.", Line: 8, Column: 1},
			},
		},
		{
			name: "run versus do not run",
			instructions: []Instruction{
				{Text: "Run tests before committing.", Line: 4, Column: 1},
				{Text: "Do not run tests before committing.", Line: 9, Column: 1},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When
			got := Conflicts("AGENTS.md", tt.instructions)

			// Then
			if len(got) != 1 {
				t.Fatalf("Conflicts() = %#v, want one diagnostic", got)
			}
			wantCurrent := tt.instructions[1]
			wantRelated := tt.instructions[0]
			if got[0].Rule != "conflicting-instruction" || got[0].Severity != Warning ||
				got[0].Line != wantCurrent.Line || got[0].Related.Line != wantRelated.Line {
				t.Fatalf("Conflicts() diagnostic = %#v", got[0])
			}
		})
	}
}

func TestConflicts_detectsSameScopeExclusivePackageManagers(t *testing.T) {
	// Given
	instructions := []Instruction{
		{Text: "Use npm for frontend dependencies.", Line: 2, Column: 1},
		{Text: "Use pnpm for frontend dependencies.", Line: 7, Column: 1},
	}

	// When
	got := Conflicts("AGENTS.md", instructions)

	// Then
	if len(got) != 1 || got[0].Line != 7 || got[0].Related.Line != 2 {
		t.Fatalf("Conflicts() = %#v, want line 7 related to line 2", got)
	}
}

func TestConflicts_ignoresNonConflictingInstructions(t *testing.T) {
	tests := []struct {
		name         string
		instructions []Instruction
	}{
		{
			name: "different scope",
			instructions: []Instruction{
				{Text: "Use npm for publishing.", Line: 1},
				{Text: "Use pnpm for local development.", Line: 2},
			},
		},
		{
			name: "different action",
			instructions: []Instruction{
				{Text: "Install npm.", Line: 1},
				{Text: "Use pnpm.", Line: 2},
			},
		},
		{
			name: "unrelated negation",
			instructions: []Instruction{
				{Text: "Do not remove tests.", Line: 1},
				{Text: "Remove unused imports.", Line: 2},
			},
		},
		{
			name: "similar but not opposite",
			instructions: []Instruction{
				{Text: "Prefer npm.", Line: 1},
				{Text: "Use npm.", Line: 2},
			},
		},
		{
			name: "strength difference only",
			instructions: []Instruction{
				{Text: "Use npm.", Line: 1},
				{Text: "Always use npm.", Line: 2},
			},
		},
		{
			name: "unscoped tool choice",
			instructions: []Instruction{
				{Text: "Use npm.", Line: 1},
				{Text: "Use pnpm.", Line: 2},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When
			got := Conflicts("AGENTS.md", tt.instructions)

			// Then
			if len(got) != 0 {
				t.Fatalf("Conflicts() = %#v, want no diagnostics", got)
			}
		})
	}
}

func TestConflicts_reportsOnlyFirstSemanticPair_whenDuplicatesRepeat(t *testing.T) {
	// Given
	instructions := []Instruction{
		{Text: "Always use npm.", Line: 1},
		{Text: "always use npm", Line: 2},
		{Text: "Never use npm.", Line: 3},
		{Text: "never use npm", Line: 4},
	}

	// When
	got := Conflicts("AGENTS.md", instructions)

	// Then
	if len(got) != 1 || got[0].Line != 3 || got[0].Related.Line != 1 {
		t.Fatalf("Conflicts() = %#v, want one diagnostic for lines 3 and 1", got)
	}
}

func TestLint_ordersDuplicateAndConflictDiagnosticsByPrimaryLine(t *testing.T) {
	// Given
	path := filepath.Join(t.TempDir(), "AGENTS.md")
	data := []byte("Always run tests.\n- always run tests\n- Never run tests.\n")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	// When
	result, err := Lint(path)

	// Then
	if err != nil {
		t.Fatal(err)
	}
	want := []struct {
		rule    string
		line    int
		related int
	}{
		{rule: "duplicate-instruction", line: 2, related: 1},
		{rule: "conflicting-instruction", line: 3, related: 1},
	}
	if len(result.Diagnostics) != len(want) {
		t.Fatalf("Lint() = %#v, want %d diagnostics", result, len(want))
	}
	for i, diagnostic := range result.Diagnostics {
		if diagnostic.Rule != want[i].rule || diagnostic.Line != want[i].line || diagnostic.Related.Line != want[i].related {
			t.Fatalf("diagnostic %d = %#v, want %#v", i, diagnostic, want[i])
		}
	}
}

func TestReport_labelsConflictRelatedLocation(t *testing.T) {
	// Given
	diagnostics := []Diagnostic{{
		Rule: "conflicting-instruction", Severity: Warning,
		Message:  "Conflicting instruction: Never use npm.",
		Location: Location{File: "AGENTS.md", Line: 2, Column: 1},
		Related:  Location{File: "AGENTS.md", Line: 1, Column: 1},
	}}
	var output bytes.Buffer

	// When
	err := Report(&output, diagnostics)

	// Then
	if err != nil {
		t.Fatal(err)
	}
	want := "AGENTS.md:2:1: warning conflicting-instruction: Conflicting instruction: Never use npm. (conflicts with AGENTS.md:1)\n\n1 warning\n"
	if output.String() != want {
		t.Fatalf("Report() = %q, want %q", output.String(), want)
	}
}

func BenchmarkConflictRule1000(b *testing.B) {
	instructions := make([]Instruction, 1000)
	for i := range instructions {
		instructions[i] = Instruction{Text: "Use npm for scope " + string(rune('a'+i%26)) + ".", Line: i + 1, Column: 1}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Conflicts("AGENTS.md", instructions)
	}
}
