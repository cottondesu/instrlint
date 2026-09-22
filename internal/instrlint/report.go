package instrlint

import (
	"fmt"
	"io"
	"strings"
)

func Report(w io.Writer, diagnostics []Diagnostic) error {
	for _, diagnostic := range diagnostics {
		if _, err := fmt.Fprintf(w, "%s:%d:%d: %s %s: %s (first at %s:%d)\n",
			diagnostic.File, diagnostic.Line, diagnostic.Column,
			diagnostic.Severity, diagnostic.Rule, diagnostic.Message,
			diagnostic.Related.File, diagnostic.Related.Line); err != nil {
			return err
		}
	}
	if len(diagnostics) > 0 {
		_, err := fmt.Fprintf(w, "\n%d %s\n", len(diagnostics), plural("warning", len(diagnostics)))
		return err
	}
	return nil
}

func plural(word string, count int) string {
	if count == 1 {
		return word
	}
	return strings.TrimSpace(word) + "s"
}
