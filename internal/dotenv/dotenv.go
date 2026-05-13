// Package dotenv reads/writes .env files preserving key order, and computes
// diffs between two key/value sets for the push/pull/diff commands.
package dotenv

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strings"
)

// Entry is one parsed line from a .env file.
type Entry struct {
	Key   string
	Value string
}

// Read parses a .env file. Returns an empty slice if the file doesn't exist.
func Read(path string) ([]Entry, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var out []Entry
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimPrefix(line, "export ")
		}
		i := strings.IndexByte(line, '=')
		if i <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:i])
		val := strings.TrimSpace(line[i+1:])
		val = unquote(val)
		out = append(out, Entry{Key: key, Value: val})
	}
	return out, sc.Err()
}

// Write writes entries to path. If sorted is true, keys are alphabetised.
// Existing file is overwritten.
func Write(path string, entries []Entry, sorted bool) error {
	if sorted {
		entries = append([]Entry(nil), entries...)
		sort.Slice(entries, func(i, j int) bool { return entries[i].Key < entries[j].Key })
	}
	var sb strings.Builder
	for _, e := range entries {
		sb.WriteString(e.Key)
		sb.WriteByte('=')
		sb.WriteString(quote(e.Value))
		sb.WriteByte('\n')
	}
	return os.WriteFile(path, []byte(sb.String()), 0o600)
}

// ToMap returns entries as a map (last write wins on duplicate keys).
func ToMap(entries []Entry) map[string]string {
	m := make(map[string]string, len(entries))
	for _, e := range entries {
		m[e.Key] = e.Value
	}
	return m
}

// FromMap turns a map into a sorted Entry slice.
func FromMap(m map[string]string) []Entry {
	out := make([]Entry, 0, len(m))
	for k, v := range m {
		out = append(out, Entry{Key: k, Value: v})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

// Diff describes the change set between local and remote.
type Diff struct {
	Added    []Entry          // present locally, missing remote (push) or vice versa (pull)
	Removed  []string         // present remote, missing local (push) or vice versa (pull)
	Changed  []ChangedEntry   // key present on both with different values
}

type ChangedEntry struct {
	Key   string
	Local string
	Other string
}

// Compare returns Added/Removed/Changed treating `from` as the source-of-truth.
// "Added" = in from but not in to. "Removed" = in to but not in from.
func Compare(from, to map[string]string) Diff {
	var d Diff
	for k, v := range from {
		other, ok := to[k]
		if !ok {
			d.Added = append(d.Added, Entry{Key: k, Value: v})
		} else if other != v {
			d.Changed = append(d.Changed, ChangedEntry{Key: k, Local: v, Other: other})
		}
	}
	for k := range to {
		if _, ok := from[k]; !ok {
			d.Removed = append(d.Removed, k)
		}
	}
	sort.Slice(d.Added, func(i, j int) bool { return d.Added[i].Key < d.Added[j].Key })
	sort.Slice(d.Changed, func(i, j int) bool { return d.Changed[i].Key < d.Changed[j].Key })
	sort.Strings(d.Removed)
	return d
}

// Empty reports whether d has any changes.
func (d Diff) Empty() bool {
	return len(d.Added) == 0 && len(d.Removed) == 0 && len(d.Changed) == 0
}

// Render produces a human-readable summary of the diff.
func (d Diff) Render() string {
	if d.Empty() {
		return "no differences"
	}
	var sb strings.Builder
	for _, e := range d.Added {
		fmt.Fprintf(&sb, "+ %s=%s\n", e.Key, mask(e.Value))
	}
	for _, c := range d.Changed {
		fmt.Fprintf(&sb, "~ %s: %s -> %s\n", c.Key, mask(c.Local), mask(c.Other))
	}
	for _, k := range d.Removed {
		fmt.Fprintf(&sb, "- %s\n", k)
	}
	return sb.String()
}

func mask(v string) string {
	if len(v) <= 4 {
		return strings.Repeat("*", len(v))
	}
	return v[:2] + strings.Repeat("*", len(v)-4) + v[len(v)-2:]
}

func unquote(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			s = s[1 : len(s)-1]
		}
	}
	s = strings.ReplaceAll(s, `\n`, "\n")
	s = strings.ReplaceAll(s, `\t`, "\t")
	return s
}

func quote(v string) string {
	if v == "" {
		return ""
	}
	needsQuote := strings.ContainsAny(v, " \t#\"'$`\n")
	if !needsQuote {
		return v
	}
	escaped := strings.ReplaceAll(v, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	escaped = strings.ReplaceAll(escaped, "\n", `\n`)
	return `"` + escaped + `"`
}
