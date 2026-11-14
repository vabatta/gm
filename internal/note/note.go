// Package note defines a memory note, its file format, and the rules a new
// note must meet.
//
// A note file starts with a header between two "---" lines, one "key: value"
// per line. The title is a JSON string, so any title fits on one line. The at
// and supersedes lists are written as [a, b], or as a JSON array when an item
// needs quoting. The body follows the header, with one "## Heading" section for
// each section in the note type's template.
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
	// ID is the creation time plus 6 random hex characters; see NewID.
	ID string `json:"id"`
	// Type is decision, lesson, fact or retirement, and picks the template in Templates.
	Type string `json:"type"`
	// Title is a one-line claim of at most MaxTitle characters.
	Title string `json:"title"`
	// Topic is an optional stable key, such as decision/cache-store. Lint
	// fails when two live notes share a topic.
	Topic string `json:"topic,omitempty"`
	// Keywords are space-separated search words, including synonyms the text
	// does not use. Search weights them between the title and the body.
	Keywords string `json:"keywords,omitempty"`
	// At lists the anchors, written commit:path[:from[-to]]; see package anchor.
	At []string `json:"at,omitempty"`
	// Supersedes lists the ids of the notes this note replaces. A superseded
	// note stays on disk as history but is no longer live.
	Supersedes []string `json:"supersedes,omitempty"`
	// Created is the creation time in RFC 3339, UTC. gm list and gm context sort by it.
	Created string `json:"created"`
	// Migrated records where an imported note came from, such as
	// "engram obs-1 (decision)". A migrated note keeps the body it was imported
	// with, so Problems skips the template checks for it.
	Migrated string `json:"migrated,omitempty"`
	// Body holds the "## Heading" sections.
	Body string `json:"body"`
	// File is the note's path relative to the repository root. It is not
	// written in the header, because the file's own path holds it.
	File string `json:"file"`
}

// Section is one heading of a note body and the gm save flag that fills it.
type Section struct{ Heading, Flag string }

// Templates lists, for each note type, the sections its body must have, in
// order. gm save defines one flag per section, and Problems refuses a note with
// a missing or empty section. Only gm retire writes a retirement.
var Templates = map[string][]Section{
	"decision":   {{"Decision", "decision"}, {"Why", "why"}, {"Rejected", "rejected"}},
	"lesson":     {{"What happened", "happened"}, {"Cause", "cause"}, {"Rule now", "rule"}},
	"fact":       {{"Fact", "fact"}, {"Evidence", "evidence"}},
	"retirement": {{"Why", "why"}},
}

// Limits a new note must meet. They keep each note to one idea that a search
// result can show in full. Lengths count characters, not bytes.
const (
	MaxTitle    = 120
	MaxSection  = 800 // per section
	MinKeywords = 3   // words, so that synonyms the text does not use can find the note
)

// NewID returns the id of a note created at t: the UTC time as
// 20060102T150405Z, a hyphen, and 6 random hex characters. The random part keeps
// ids unique when agents on parallel branches save in the same second. The time
// part makes ids, and so note file names, sort by creation time.
func NewID(t time.Time) string {
	b := make([]byte, 3)
	_, _ = rand.Read(b)
	return t.UTC().Format("20060102T150405Z") + "-" + hex.EncodeToString(b)
}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

// FileName returns the note's file name: the id, a hyphen, and the title in
// lowercase with each run of other characters turned into a hyphen, cut to 48
// characters. The id alone identifies the note. The title part helps a person
// who reads the directory listing.
func (n Note) FileName() string {
	s := strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(n.Title), "-"), "-")
	if len(s) > 48 {
		s = strings.TrimRight(s[:48], "-")
	}
	return n.ID + "-" + s + ".md"
}

// Shorten cuts s to at most max characters and ends a cut string with "...".
// It counts characters, not bytes, so it never splits a multibyte character.
func Shorten(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	return strings.TrimSpace(string([]rune(s)[:max-3])) + "..."
}

// Body renders the body of a note of type typ. text maps a section heading to
// its text. Sections appear in template order, and a heading missing from text
// renders as an empty section, which Problems then reports.
func Body(typ string, text map[string]string) string {
	var b strings.Builder
	for _, s := range Templates[typ] {
		fmt.Fprintf(&b, "## %s\n%s\n\n", s.Heading, strings.TrimSpace(text[s.Heading]))
	}
	return strings.TrimSpace(b.String()) + "\n"
}

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

// Parse reads the content of a note file. file is the file's path relative to
// the repository root, kept in Note.File. CRLF line ends are accepted. Unknown
// header keys are ignored, so an older gm can read notes from a newer one.
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

// Problems returns every rule the note breaks, so gm save can refuse a note
// with all of its problems at once. Every note is checked for its title, the
// form of its keywords and topic, and secrets. The checks stop there for a
// migrated note, because its body was written before the templates existed.
// Other notes are also checked for their type, keyword count, retirement
// target and sections.
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

// secrets are credential formats with a fixed prefix or shape. The
// password-in-a-URL pattern skips a password that starts with $, <, { or *,
// because that marks a placeholder such as $PASSWORD.
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

// assigned matches a secret-like name given a value of 12 or more characters,
// such as API_KEY=a1b2c3d4e5f6. Secret counts a match only when the value holds
// both a digit and a letter, so prose such as "token: refresh_tokens_are_cached"
// passes.
var assigned = regexp.MustCompile(`(?i)(?:api[_-]?key|secret|token|passw(?:or)?d|pwd|credential)s?["']?\s*[:=]\s*["']?([A-Za-z0-9/+_.=-]{12,})`)
var digit, letter = regexp.MustCompile(`[0-9]`), regexp.MustCompile(`[A-Za-z]`)

// Secret reports whether s holds text shaped like a credential, and names the
// kind. Callers print the kind, never the matched text, so gm does not echo a
// secret.
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
