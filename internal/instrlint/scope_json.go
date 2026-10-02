package instrlint

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"unicode/utf16"
	"unicode/utf8"
)

const scopeJSONSchemaVersion = 1

type scopeJSONDocument struct {
	SchemaVersion int              `json:"schema_version"`
	Entries       []scopeJSONEntry `json:"entries"`
}

type scopeJSONEntry struct {
	Path   string  `json:"path"`
	Parent *string `json:"parent"`
}

// RenderScopeJSON writes the scope hierarchy as one compact JSON document
// followed by a newline. The document is built and validated before anything
// is written, so a rendering error never leaves partial JSON on output.
func RenderScopeJSON(output io.Writer, roots []*ScopeNode) error {
	document := scopeJSONDocument{SchemaVersion: scopeJSONSchemaVersion, Entries: []scopeJSONEntry{}}
	var flatten func([]*ScopeNode, *string) error
	flatten = func(nodes []*ScopeNode, parent *string) error {
		for _, node := range nodes {
			if !utf8.ValidString(node.Path) {
				return errors.New("cannot render scope JSON: path is not valid UTF-8")
			}
			document.Entries = append(document.Entries, scopeJSONEntry{Path: node.Path, Parent: parent})
			if err := flatten(node.Children, &node.Path); err != nil {
				return err
			}
		}
		return nil
	}
	if err := flatten(roots, nil); err != nil {
		return err
	}

	var encoded bytes.Buffer
	encoder := json.NewEncoder(&encoded)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(document); err != nil {
		return fmt.Errorf("cannot render scope JSON: %w", err)
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

// escapeNonGraphicJSON replaces non-graphic runes that encoding/json leaves
// raw, such as DEL and Unicode bidirectional controls, with \u escapes. The
// input is valid UTF-8 JSON whose only raw non-graphic byte outside strings
// is the trailing newline, so the result decodes to the same values.
func escapeNonGraphicJSON(encoded []byte) []byte {
	body := encoded[:len(encoded)-1]
	escaped := make([]byte, 0, len(encoded))
	for len(body) > 0 {
		r, size := utf8.DecodeRune(body)
		if strconv.IsGraphic(r) {
			escaped = append(escaped, body[:size]...)
		} else if r1, r2 := utf16.EncodeRune(r); r1 != utf8.RuneError {
			escaped = fmt.Appendf(escaped, `\u%04x\u%04x`, r1, r2)
		} else {
			escaped = fmt.Appendf(escaped, `\u%04x`, r)
		}
		body = body[size:]
	}
	return append(escaped, '\n')
}
