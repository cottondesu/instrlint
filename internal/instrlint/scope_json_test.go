package instrlint

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"
)

type decodedScopeEntry struct {
	Path   string
	Parent *string
}

// decodeScopeJSON strictly decodes a scope JSON document, rejecting unknown
// fields and anything after the single document.
func decodeScopeJSON(t *testing.T, raw []byte) (int, []decodedScopeEntry) {
	t.Helper()
	var document struct {
		SchemaVersion *int `json:"schema_version"`
		Entries       []struct {
			Path   *string `json:"path"`
			Parent *string `json:"parent"`
		} `json:"entries"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&document); err != nil {
		t.Fatalf("decode %q: %v", raw, err)
	}
	if decoder.More() {
		t.Fatalf("trailing data after JSON document: %q", raw)
	}
	if document.SchemaVersion == nil || document.Entries == nil {
		t.Fatalf("missing schema_version or entries: %q", raw)
	}
	entries := make([]decodedScopeEntry, 0, len(document.Entries))
	for _, entry := range document.Entries {
		if entry.Path == nil {
			t.Fatalf("entry without path: %q", raw)
		}
		entries = append(entries, decodedScopeEntry{Path: *entry.Path, Parent: entry.Parent})
	}
	return *document.SchemaVersion, entries
}

func assertScopeJSONSafe(t *testing.T, raw []byte) {
	t.Helper()
	if !utf8.Valid(raw) || !bytes.HasSuffix(raw, []byte("\n")) || bytes.Count(raw, []byte("\n")) != 1 {
		t.Fatalf("JSON output must be one valid UTF-8 line: %q", raw)
	}
	for _, r := range string(raw[:len(raw)-1]) {
		if !strconv.IsGraphic(r) {
			t.Fatalf("JSON output contains raw non-graphic rune %U: %q", r, raw)
		}
	}
}

func renderScopeJSONString(t *testing.T, roots []*ScopeNode) string {
	t.Helper()
	var output bytes.Buffer
	if err := RenderScopeJSON(&output, roots); err != nil {
		t.Fatal(err)
	}
	assertScopeJSONSafe(t, output.Bytes())
	return output.String()
}

func TestRenderScopeJSON_fixtures(t *testing.T) {
	tests := []struct {
		name     string
		fixture  string
		excludes []string
		want     string
	}{
		{"root only", "root-only", nil, `{"schema_version":1,"entries":[{"path":"AGENTS.md","parent":null}]}`},
		{"root with siblings", "root-with-siblings", nil, `{"schema_version":1,"entries":[{"path":"AGENTS.md","parent":null},{"path":"backend/AGENTS.md","parent":"AGENTS.md"},{"path":"frontend/AGENTS.md","parent":"AGENTS.md"}]}`},
		{"deep nesting", "deep-nesting", nil, `{"schema_version":1,"entries":[{"path":"AGENTS.md","parent":null},{"path":"be/AGENTS.md","parent":"AGENTS.md"},{"path":"be/src/exprs_ext/AGENTS.md","parent":"be/AGENTS.md"}]}`},
		{"four levels and missing intermediate file", "four-level", nil, `{"schema_version":1,"entries":[{"path":"AGENTS.md","parent":null},{"path":"a/AGENTS.md","parent":"AGENTS.md"},{"path":"a/b/AGENTS.md","parent":"a/AGENTS.md"},{"path":"a/b/c/AGENTS.md","parent":"a/b/AGENTS.md"},{"path":"services/app/internal/AGENTS.md","parent":"AGENTS.md"}]}`},
		{"rootless forest", "rootless-forest", nil, `{"schema_version":1,"entries":[{"path":"apps/application/AGENTS.md","parent":null},{"path":"packages/reporter/AGENTS.md","parent":null}]}`},
		{"multiple roots", "multiple-roots", nil, `{"schema_version":1,"entries":[{"path":"apps/AGENTS.md","parent":null},{"path":"apps/web/AGENTS.md","parent":"apps/AGENTS.md"},{"path":"packages/AGENTS.md","parent":null},{"path":"packages/cli/AGENTS.md","parent":"packages/AGENTS.md"}]}`},
		{"excluded subtree visible", "excluded-subtree", nil, `{"schema_version":1,"entries":[{"path":"AGENTS.md","parent":null},{"path":".omx/generated/AGENTS.md","parent":"AGENTS.md"},{"path":"src/AGENTS.md","parent":"AGENTS.md"}]}`},
		{"excluded subtree hidden", "excluded-subtree", []string{".omx"}, `{"schema_version":1,"entries":[{"path":"AGENTS.md","parent":null},{"path":"src/AGENTS.md","parent":"AGENTS.md"}]}`},
		{"similar path prefixes remain siblings", "similar-path-prefix", nil, `{"schema_version":1,"entries":[{"path":"foo/AGENTS.md","parent":null},{"path":"foobar/AGENTS.md","parent":null}]}`},
		{"relative excluded subtree", "relative-excluded-subtree", []string{"tools/cache"}, `{"schema_version":1,"entries":[{"path":"AGENTS.md","parent":null},{"path":"tools/source/AGENTS.md","parent":"AGENTS.md"}]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nodes, err := Scope(filepath.Join("testdata", "scope", tt.fixture), tt.excludes)
			if err != nil {
				t.Fatal(err)
			}
			got := renderScopeJSONString(t, nodes)
			if got != tt.want+"\n" {
				t.Fatalf("scope JSON = %q, want %q", got, tt.want+"\n")
			}
			version, entries := decodeScopeJSON(t, []byte(got))
			if version != 1 {
				t.Fatalf("schema_version = %d, want 1", version)
			}
			seen := make(map[string]bool, len(entries))
			for _, entry := range entries {
				if strings.HasPrefix(entry.Path, "/") || strings.Contains(entry.Path, `\`) || !strings.HasSuffix(entry.Path, "AGENTS.md") {
					t.Fatalf("path %q is not a scan-root-relative slash path", entry.Path)
				}
				if entry.Parent != nil && !seen[*entry.Parent] {
					t.Fatalf("parent %q of %q does not name an earlier entry", *entry.Parent, entry.Path)
				}
				seen[entry.Path] = true
			}
		})
	}
}

func TestRenderScopeJSON_emptyEntriesIsArray(t *testing.T) {
	for _, roots := range [][]*ScopeNode{nil, {}} {
		if got, want := renderScopeJSONString(t, roots), "{\"schema_version\":1,\"entries\":[]}\n"; got != want {
			t.Fatalf("empty scope JSON = %q, want %q", got, want)
		}
	}
}

func TestRenderScopeJSON_isDeterministic(t *testing.T) {
	nodes, err := Scope(filepath.Join("testdata", "scope", "four-level"), nil)
	if err != nil {
		t.Fatal(err)
	}
	first := renderScopeJSONString(t, nodes)
	for i := 0; i < 20; i++ {
		again, err := Scope(filepath.Join("testdata", "scope", "four-level"), nil)
		if err != nil {
			t.Fatal(err)
		}
		if got := renderScopeJSONString(t, again); got != first {
			t.Fatalf("render %d = %q, want %q", i, got, first)
		}
		if got := renderScopeJSONString(t, nodes); got != first {
			t.Fatalf("re-render %d = %q, want %q", i, got, first)
		}
	}
}

func TestRenderScopeJSON_escapesUnsafeRunesLosslessly(t *testing.T) {
	root := "repo"
	for _, tt := range []struct{ label, name string }{
		{"newline", "line\nforged"},
		{"carriage return", "carriage\rreturn"},
		{"terminal escape", "esc\x1b]0;title\a"},
		{"bell", "bell\a"},
		{"delete", "del\x7f"},
		{"bidirectional control", "bidi‮evil"},
		{"bidirectional isolate", "isolate⁦x⁩"},
		{"line separator", "sep x"},
		{"supplementary tag", "tag\U000E0041"},
	} {
		t.Run(tt.label, func(t *testing.T) {
			ancestor := tt.name + "/AGENTS.md"
			child := tt.name + "/child/AGENTS.md"
			files := []string{
				filepath.Join(root, tt.name, "AGENTS.md"),
				filepath.Join(root, tt.name, "child", "AGENTS.md"),
			}
			nodes, err := buildScopeHierarchy(root, files)
			if err != nil {
				t.Fatal(err)
			}
			raw := renderScopeJSONString(t, nodes)
			if strings.Contains(raw, `\"`) {
				t.Fatalf("display quoting leaked into JSON data: %q", raw)
			}
			version, entries := decodeScopeJSON(t, []byte(raw))
			if version != 1 || len(entries) != 2 ||
				entries[0].Path != ancestor || entries[0].Parent != nil ||
				entries[1].Path != child || entries[1].Parent == nil || *entries[1].Parent != ancestor {
				t.Fatalf("decoded entries = %+v from %q", entries, raw)
			}
		})
	}
}

func TestRenderScopeJSON_preservesNormalUnicode(t *testing.T) {
	root := "repo"
	paths := []string{"café/AGENTS.md", "équipe/AGENTS.md", "日本語/AGENTS.md", "日本語/サブ/AGENTS.md", "a&b<c>/AGENTS.md"}
	var files []string
	for _, path := range paths {
		files = append(files, filepath.Join(root, filepath.FromSlash(path)))
	}
	nodes, err := buildScopeHierarchy(root, files)
	if err != nil {
		t.Fatal(err)
	}
	raw := renderScopeJSONString(t, nodes)
	for _, path := range paths {
		if !strings.Contains(raw, `"`+path+`"`) {
			t.Fatalf("graphic Unicode path %q was escaped: %q", path, raw)
		}
	}
	_, entries := decodeScopeJSON(t, []byte(raw))
	want := []decodedScopeEntry{
		{Path: "a&b<c>/AGENTS.md"},
		{Path: "café/AGENTS.md"},
		{Path: "équipe/AGENTS.md"},
		{Path: "日本語/AGENTS.md"},
		{Path: "日本語/サブ/AGENTS.md", Parent: &paths[2]},
	}
	if len(entries) != len(want) {
		t.Fatalf("decoded entries = %+v", entries)
	}
	for i := range want {
		if entries[i].Path != want[i].Path || (entries[i].Parent == nil) != (want[i].Parent == nil) ||
			entries[i].Parent != nil && *entries[i].Parent != *want[i].Parent {
			t.Fatalf("entry %d = %+v, want %+v", i, entries[i], want[i])
		}
	}
}

func TestRenderScopeJSON_rejectsInvalidUTF8PathWithoutOutput(t *testing.T) {
	root := "repo"
	files := []string{
		filepath.Join(root, "AGENTS.md"),
		filepath.Join(root, "bad\xff", "AGENTS.md"),
	}
	nodes, err := buildScopeHierarchy(root, files)
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	err = RenderScopeJSON(&output, nodes)
	if err == nil || output.Len() != 0 || !strings.Contains(err.Error(), "not valid UTF-8") || !utf8.ValidString(err.Error()) {
		t.Fatalf("err=%v output=%q", err, output.String())
	}
}

type scopeTestWriter struct {
	limit int
	err   error
}

func (w scopeTestWriter) Write(p []byte) (int, error) {
	return min(w.limit, len(p)), w.err
}

func TestRenderScopeJSON_returnsWriteErrors(t *testing.T) {
	nodes, err := Scope(filepath.Join("testdata", "scope", "root-only"), nil)
	if err != nil {
		t.Fatal(err)
	}
	diskFull := errors.New("disk full")
	for _, tt := range []struct {
		name   string
		writer scopeTestWriter
		want   error
	}{
		{"immediate error", scopeTestWriter{0, diskFull}, diskFull},
		{"partial write with error", scopeTestWriter{5, diskFull}, diskFull},
		{"partial write without error", scopeTestWriter{5, nil}, io.ErrShortWrite},
		{"empty write without error", scopeTestWriter{0, nil}, io.ErrShortWrite},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if err := RenderScopeJSON(tt.writer, nodes); !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
		})
	}
}
