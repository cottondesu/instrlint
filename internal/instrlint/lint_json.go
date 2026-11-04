package instrlint

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"unicode/utf8"
)

const lintJSONSchemaVersion = 1

// lintJSONDocument is the wire representation of a lint diagnostics report.
// The field declaration order fixes the JSON field order.
type lintJSONDocument struct {
	SchemaVersion int                  `json:"schema_version"`
	Diagnostics   []lintJSONDiagnostic `json:"diagnostics"`
}

type lintJSONDiagnostic struct {
	Path     string           `json:"path"`
	Line     int              `json:"line"`
	Column   int              `json:"column"`
	Severity string           `json:"severity"`
	Rule     string           `json:"rule"`
	Message  string           `json:"message"`
	Related  *lintJSONRelated `json:"related,omitempty"`
}

type lintJSONRelated struct {
	Path   string `json:"path"`
	Line   int    `json:"line"`
	Column int    `json:"column"`
}

// RenderLintJSON writes diagnostics as one compact JSON document followed by
// a newline. Diagnostics are rendered in the order given, which callers are
// expected to have sorted already. The document is built and validated before
// anything is written, so a rendering error never leaves partial JSON on
// output. Strings that are not valid UTF-8 are rejected instead of being
// silently replaced with U+FFFD.
func RenderLintJSON(output io.Writer, diagnostics []Diagnostic) error {
	document := lintJSONDocument{
		SchemaVersion: lintJSONSchemaVersion,
		Diagnostics:   make([]lintJSONDiagnostic, 0, len(diagnostics)),
	}
	for _, diagnostic := range diagnostics {
		if !utf8.ValidString(diagnostic.File) {
			return errors.New("cannot render lint JSON: path is not valid UTF-8")
		}
		if !utf8.ValidString(diagnostic.Message) {
			return errors.New("cannot render lint JSON: message is not valid UTF-8")
		}
		if !utf8.ValidString(string(diagnostic.Severity)) {
			return errors.New("cannot render lint JSON: severity is not valid UTF-8")
		}
		if !utf8.ValidString(diagnostic.Rule) {
			return errors.New("cannot render lint JSON: rule is not valid UTF-8")
		}
		entry := lintJSONDiagnostic{
			Path:     diagnostic.File,
			Line:     diagnostic.Line,
			Column:   diagnostic.Column,
			Severity: string(diagnostic.Severity),
			Rule:     diagnostic.Rule,
			Message:  diagnostic.Message,
		}
		if related := diagnostic.Related; related != (Location{}) {
			if !utf8.ValidString(related.File) {
				return errors.New("cannot render lint JSON: related path is not valid UTF-8")
			}
			entry.Related = &lintJSONRelated{
				Path:   related.File,
				Line:   related.Line,
				Column: related.Column,
			}
		}
		document.Diagnostics = append(document.Diagnostics, entry)
	}

	var encoded bytes.Buffer
	encoder := json.NewEncoder(&encoded)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(document); err != nil {
		return fmt.Errorf("cannot render lint JSON: %w", err)
	}
	data := escapeNonGraphicJSON(encoded.Bytes())
	written, err := output.Write(data)
	if err == nil && written != len(data) {
		err = io.ErrShortWrite
	}
	if err != nil {
		return fmt.Errorf("write output: %w", err)
	}
	return nil
}
