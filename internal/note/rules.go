package note

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

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
