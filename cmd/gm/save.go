package main

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/vabatta/gm/internal/memory"
	"github.com/vabatta/gm/internal/note"
)

// save runs gm save TYPE. It defines one flag per section of the type's
// template, builds the body from those flags, and passes the note to
// Store.Save. A positional word means a flag value was not quoted, so save
// refuses it rather than drop it.
func save(st *memory.Store, args []string, w io.Writer) error {
	typ := ""
	if len(args) > 0 {
		typ, args = args[0], args[1:]
	}
	if note.Templates[typ] == nil || typ == "retirement" {
		return errors.New("save needs a type first: decision, lesson or fact")
	}
	n := note.Note{Type: typ}
	fs := flags("save")
	fs.StringVar(&n.Title, "title", "", "")
	fs.StringVar(&n.Keywords, "keywords", "", "")
	fs.StringVar(&n.Topic, "topic", "", "")
	var at []string
	fs.Func("at", "", func(v string) error { at = append(at, v); return nil })
	fs.Func("supersedes", "", func(v string) error { n.Supersedes = append(n.Supersedes, v); return nil })
	vals := map[string]*string{}
	for _, s := range note.Templates[typ] {
		vals[s.Heading] = fs.String(s.Flag, "", "")
	}
	if err := parse(fs, args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected words %q; quote each flag value", fs.Args())
	}
	text := map[string]string{}
	for h, v := range vals {
		text[h] = *v
	}
	n.Body = note.Body(typ, text)
	s, err := st.Save(n, at)
	return saved(s, err, w)
}

// retire runs gm retire ID... --why TEXT.
func retire(st *memory.Store, args []string, w io.Writer) error {
	fs := flags("retire")
	why := fs.String("why", "", "")
	if err := parse(fs, args); err != nil {
		return err
	}
	s, err := st.Retire(fs.Args(), *why)
	return saved(s, err, w)
}

// saved prints the id and path of the note that save or retire wrote. It warns
// when other live notes share the note's topic.
func saved(s memory.Saved, err error, w io.Writer) error {
	if err != nil {
		return err
	}
	fmt.Fprintln(w, s.Note.ID, s.Note.File)
	if len(s.Others) > 0 {
		fmt.Fprintf(w, "gm: topic %s has other live notes: %s; if this note replaces them, save it again with --supersedes for each\n",
			s.Note.Topic, strings.Join(s.Others, ", "))
	}
	return nil
}
