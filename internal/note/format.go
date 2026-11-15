package note

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Render returns the content of the note's file. Empty optional fields are
// left out of the header.
func (n Note) Render() string {
	t, _ := json.Marshal(n.Title)
	h := []string{"---", "id: " + n.ID, "type: " + n.Type, "title: " + string(t)}
	add := func(k, v string) {
		if v != "" {
			h = append(h, k+": "+v)
		}
	}
	add("topic", n.Topic)
	add("keywords", n.Keywords)
	add("at", list(n.At))
	add("supersedes", list(n.Supersedes))
	add("created", n.Created)
	add("migrated", n.Migrated)
	return strings.Join(append(h, "---"), "\n") + "\n" + n.Body
}

// list writes a header list as [a, b]. It writes a JSON array instead when an
// item holds a comma, a bracket, a quote, or a leading or trailing space,
// because the plain form would split or trim such an item.
func list(xs []string) string {
	if len(xs) == 0 {
		return ""
	}
	for _, x := range xs {
		if strings.ContainsAny(x, `,[]"`) || x != strings.TrimSpace(x) {
			b, _ := json.Marshal(xs)
			return string(b)
		}
	}
	return "[" + strings.Join(xs, ", ") + "]"
}

// parseList reads either form that list writes.
func parseList(v string) []string {
	var xs []string
	if json.Unmarshal([]byte(v), &xs) == nil {
		return xs
	}
	for x := range strings.SplitSeq(strings.Trim(v, "[]"), ",") {
		if x = strings.TrimSpace(x); x != "" {
			xs = append(xs, x)
		}
	}
	return xs
}

// Format is the note format this gm reads and writes. A note without a format
// key is format 1, so gm writes none.
const Format = "1"

// Parse reads the content of a note file. file is the file's path relative to
// the repository root, kept in Note.File. CRLF line ends are accepted. Unknown
// header keys are ignored, so an older gm can read notes from a newer one. A
// format key other than Format is refused rather than ignored: such a note
// changed in a way this gm cannot read, and reading it as format 1 would be
// wrong.
func Parse(content []byte, file string) (Note, error) {
	s := strings.ReplaceAll(string(content), "\r\n", "\n")
	if !strings.HasPrefix(s, "---\n") {
		return Note{}, errors.New("no header")
	}
	head, body, ok := strings.Cut(s[4:], "\n---\n")
	if !ok {
		return Note{}, errors.New("unterminated header")
	}
	n := Note{Body: body, File: file}
	for line := range strings.SplitSeq(head, "\n") {
		k, v, _ := strings.Cut(line, ": ")
		switch k {
		case "id":
			n.ID = v
		case "type":
			n.Type = v
		case "title":
			if json.Unmarshal([]byte(v), &n.Title) != nil {
				n.Title = v
			}
		case "topic":
			n.Topic = v
		case "keywords":
			n.Keywords = v
		case "at":
			n.At = parseList(v)
		case "supersedes":
			n.Supersedes = parseList(v)
		case "created":
			n.Created = v
		case "migrated":
			n.Migrated = v
		case "format":
			if v != Format {
				return n, fmt.Errorf("note format %s needs a newer gm", v)
			}
		}
	}
	if n.ID == "" {
		return n, errors.New("header has no id")
	}
	return n, nil
}

// Sections splits a body into its sections, keyed by heading. A section runs
// from its "## " line to the next one. Text before the first heading is dropped.
func Sections(body string) map[string]string {
	out := map[string]string{}
	cur, buf := "", []string(nil)
	flush := func() {
		if cur != "" {
			out[cur] = strings.TrimSpace(strings.Join(buf, "\n"))
		}
	}
	for line := range strings.SplitSeq(body, "\n") {
		if heading, ok := strings.CutPrefix(line, "## "); ok {
			flush()
			cur, buf = heading, nil
			continue
		}
		buf = append(buf, line)
	}
	flush()
	return out
}
