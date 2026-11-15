package note

import (
	"embed"
	"fmt"
	"io/fs"
	"strings"
	"text/template"
	"text/template/parse"
)

// files holds one text/template per note type, templates/TYPE.tmpl. A template
// is the note body: a "## Heading" line, then a {{.flag}} action that the gm
// save flag of that name fills, for each section in order.
//
//go:embed templates/*.tmpl
var files embed.FS

// Section is one heading of a note body and the gm save flag that fills it.
type Section struct{ Heading, Flag string }

// Templates lists, for each note type, the sections its body must have, in
// order. It is read from the template files, so a file is the only place a
// section is defined. gm save defines one flag per section, and Problems
// refuses a note with a missing or empty section. Only gm retire writes a
// retirement.
var Templates = map[string][]Section{}

// bodies holds the parsed template of each note type, which Body executes.
var bodies = map[string]*template.Template{}

func init() {
	names, _ := fs.Glob(files, "templates/*.tmpl")
	for _, name := range names {
		typ := strings.TrimSuffix(strings.TrimPrefix(name, "templates/"), ".tmpl")
		t := template.Must(template.New(typ).ParseFS(files, name)).Lookup(typ + ".tmpl")
		bodies[typ] = t
		Templates[typ] = sections(t)
	}
}

// sections reads the sections of a template from its parse tree: each
// {{.flag}} action, with the last "## " line of the text before it as the
// heading. A template of another shape is a mistake in this package, so it
// panics at start-up, where any test run catches it.
func sections(t *template.Template) []Section {
	var out []Section
	heading := ""
	for _, n := range t.Tree.Root.Nodes {
		switch n := n.(type) {
		case *parse.TextNode:
			for line := range strings.SplitSeq(string(n.Text), "\n") {
				if h, ok := strings.CutPrefix(line, "## "); ok {
					heading = h
				}
			}
		case *parse.ActionNode:
			f, ok := n.Pipe.Cmds[0].Args[0].(*parse.FieldNode)
			if !ok || heading == "" || len(n.Pipe.Cmds) != 1 || len(f.Ident) != 1 {
				panic(fmt.Sprintf("template %s: want a heading line before each {{.flag}}, got %s", t.Name(), n))
			}
			out = append(out, Section{heading, f.Ident[0]})
			heading = ""
		default:
			panic(fmt.Sprintf("template %s: only headings and {{.flag}} actions are allowed, got %s", t.Name(), n))
		}
	}
	return out
}

// Body renders the body of a note of type typ. text maps a section heading to
// its text. Sections appear in template order, and a heading missing from text
// renders as an empty section, which Problems then reports.
func Body(typ string, text map[string]string) string {
	data := map[string]string{}
	for _, s := range Templates[typ] {
		data[s.Flag] = strings.TrimSpace(text[s.Heading])
	}
	var b strings.Builder
	if err := bodies[typ].Execute(&b, data); err != nil {
		panic(err) // the data has a key for every action, so Execute cannot fail
	}
	return strings.TrimSpace(b.String()) + "\n"
}
