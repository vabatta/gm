// Package note defines a memory note: its fields, its file format, and the
// rules a note must meet before it is written.
//
// A note file is a header between two --- lines, one "key: value" per line,
// followed by the body: one "## Heading" section per template section.
package note

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// Note is one memory note as stored in a file.
type Note struct {
	ID         string   `json:"id"`
	Type       string   `json:"type"`
	Title      string   `json:"title"`
	Topic      string   `json:"topic,omitempty"`
	Keywords   string   `json:"keywords,omitempty"`
	At         []string `json:"at,omitempty"`
	Supersedes []string `json:"supersedes,omitempty"`
	Created    string   `json:"created"`
	Migrated   string   `json:"migrated,omitempty"`
	Body       string   `json:"body"`
	// File is the note's path relative to the repository root. It is not stored in the note.
	File string `json:"file"`
}

// Section is one heading of a note body and the save flag that fills it.
type Section struct{ Heading, Flag string }

// Templates names the sections each note type must have, in order.
var Templates = map[string][]Section{
	"decision":   {{"Decision", "decision"}, {"Why", "why"}, {"Rejected", "rejected"}},
	"lesson":     {{"What happened", "happened"}, {"Cause", "cause"}, {"Rule now", "rule"}},
	"fact":       {{"Fact", "fact"}, {"Evidence", "evidence"}},
	"retirement": {{"Why", "why"}},
}

// Limits a new note must meet.
const (
	MaxTitle    = 120 // characters
	MaxSection  = 800 // characters per section, so a note holds one idea
	MinKeywords = 3
)

// NewID returns a UTC timestamp plus 6 random hex characters, so parallel agents never pick the same id.
func NewID(t time.Time) string {
	b := make([]byte, 3)
	_, _ = rand.Read(b)
	return t.UTC().Format("20060102T150405Z") + "-" + hex.EncodeToString(b)
}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

// FileName is the note's file name: its id and a slug of its title.
func (n Note) FileName() string {
	s := strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(n.Title), "-"), "-")
	if len(s) > 48 {
		s = strings.TrimRight(s[:48], "-")
	}
	return n.ID + "-" + s + ".md"
}

// Shorten cuts s to at most max characters, marking a cut with "...".
func Shorten(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	return strings.TrimSpace(string([]rune(s)[:max-3])) + "..."
}

// Body renders sections in the order of the type's template.
func Body(typ string, text map[string]string) string {
	var b strings.Builder
	for _, s := range Templates[typ] {
		fmt.Fprintf(&b, "## %s\n%s\n\n", s.Heading, strings.TrimSpace(text[s.Heading]))
	}
	return strings.TrimSpace(b.String()) + "\n"
}

// Render returns the note's file content.
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

// list renders [a, b], or a JSON array when an item holds a comma, bracket, quote or edge space.
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

// Parse reads a note file's content; file is its path relative to the repository root.
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
		}
	}
	if n.ID == "" {
		return n, errors.New("header has no id")
	}
	return n, nil
}

// Sections splits a body into its "## Heading" sections.
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

// Problems returns every rule the note breaks. Migrated notes keep their free-form body.
func (n Note) Problems() []string {
	var p []string
	if n.Title == "" || strings.ContainsAny(n.Title, "\r\n") || utf8.RuneCountInString(n.Title) > MaxTitle {
		p = append(p, fmt.Sprintf("title must be one line of 1 to %d characters", MaxTitle))
	}
	if strings.ContainsAny(n.Keywords, "\r\n") || strings.ContainsAny(n.Topic, " \t\r\n") {
		p = append(p, "keywords must be one line and topic one word, like decision/cache-store")
	}
	if kind, ok := Secret(n.Title + "\n" + n.Keywords + "\n" + n.Body); ok {
		p = append(p, "text looks like it holds a secret ("+kind+"); remove it")
	}
	if n.Migrated != "" {
		return p
	}
	tmpl, ok := Templates[n.Type]
	if !ok {
		return append(p, "type must be decision, lesson, fact or retirement")
	}
	if n.Type != "retirement" && len(strings.Fields(n.Keywords)) < MinKeywords {
		p = append(p, fmt.Sprintf("keywords needs at least %d words; add the synonyms a searcher would use", MinKeywords))
	}
	if n.Type == "retirement" && len(n.Supersedes) == 0 {
		p = append(p, "a retirement must supersede at least one note")
	}
	secs := Sections(n.Body)
	if len(secs) > len(tmpl) {
		p = append(p, `section text must not start a line with "## "`)
	}
	for _, s := range tmpl {
		v := secs[s.Heading]
		switch {
		case v == "":
			p = append(p, "section "+s.Heading+" is empty (flag --"+s.Flag+")")
		case utf8.RuneCountInString(v) > MaxSection:
			p = append(p, fmt.Sprintf("section %s is over %d characters; split the note", s.Heading, MaxSection))
		}
	}
	return p
}

var secrets = []struct {
	kind string
	re   *regexp.Regexp
}{
	{"private key", regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY`)},
	{"AWS access key", regexp.MustCompile(`\b(?:AKIA|ASIA)[0-9A-Z]{16}\b`)},
	{"GitHub token", regexp.MustCompile(`\b(?:gh[pousr]_[A-Za-z0-9]{20,}|github_pat_[A-Za-z0-9_]{22,})`)},
	{"GitLab token", regexp.MustCompile(`\bglpat-[A-Za-z0-9_-]{20,}`)},
	{"Slack token", regexp.MustCompile(`\bxox[abposr]-[A-Za-z0-9-]{10,}`)},
	{"Google API key", regexp.MustCompile(`\bAIza[0-9A-Za-z_-]{35}`)},
	{"Stripe key", regexp.MustCompile(`\b[rs]k_(?:live|test)_[0-9A-Za-z]{16,}`)},
	{"API key", regexp.MustCompile(`\bsk-(?:ant-|proj-)?[A-Za-z0-9_-]{20,}`)},
	{"JWT", regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{10,}\.eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}`)},
	{"password in a URL", regexp.MustCompile(`[a-zA-Z][a-zA-Z0-9+.-]*://[^\s:/@]+:[^\s/@$<{*][^\s/@]{2,}@`)},
}

var assigned = regexp.MustCompile(`(?i)(?:api[_-]?key|secret|token|passw(?:or)?d|pwd|credential)s?["']?\s*[:=]\s*["']?([A-Za-z0-9/+_.=-]{12,})`)
var digit, letter = regexp.MustCompile(`[0-9]`), regexp.MustCompile(`[A-Za-z]`)

// Secret reports whether s holds text shaped like a credential, and which kind.
func Secret(s string) (string, bool) {
	for _, x := range secrets {
		if x.re.MatchString(s) {
			return x.kind, true
		}
	}
	for _, m := range assigned.FindAllStringSubmatch(s, -1) {
		if digit.MatchString(m[1]) && letter.MatchString(m[1]) {
			return "credential assignment", true
		}
	}
	return "", false
}
