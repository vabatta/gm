// Command gm keeps project memory in the repository it is about, as one
// markdown file per note in .memory/notes. Notes are committed with the work
// and merged by git like code. gm help prints the commands.
package main

import (
	"cmp"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/vabatta/gm/internal/anchor"
	"github.com/vabatta/gm/internal/engram"
	"github.com/vabatta/gm/internal/memory"
	"github.com/vabatta/gm/internal/note"
	"github.com/vabatta/gm/internal/search"
)

// usage is the text that gm help prints.
const usage = `gm: project memory as note files in .memory/notes, merged by git.

gm save decision --title T --keywords "a b c" --decision D --why W --rejected R [options]
gm save lesson   --title T --keywords "a b c" --happened H --cause C --rule R [options]
gm save fact     --title T --keywords "a b c" --fact F --evidence E [options]
    --at path[:from[-to]]  code the note is about, pinned to HEAD; path from the repo root; repeatable
    --topic K              stable key, like decision/cache-store, when the note may be revised
    --supersedes ID        a note this one replaces; repeatable
gm retire ID... --why W  withdraw notes that have no successor
gm search WORDS... [-n 5] [--type T] [--all] [--json]
gm show ID [--json]      the note and the code its anchors point at, read from their commit
gm lint [--review] [--json]  exit 1 on broken notes; --review also lists anchors whose code changed
gm list [--by type|time] [-n N] [--all] [--json]   the catalogue or the log of live notes
gm context [-n 15]       recent live notes for a session-start hook; silent outside a repo with notes
gm import-engram FILE    one-time migration from an Engram JSON export
gm --version

The manual is gm(1); the note format is gm-note(5).

Exit status: 0 ok, 1 lint found problems, 2 error.
`

// version is the gm version, set at build time with -ldflags "-X main.version=...".
var version = "dev"

func main() { os.Exit(run(os.Args[1:], ".", os.Stdout, os.Stderr)) }

// run executes one gm command in dir and returns the exit status: 0 on
// success, 1 when lint finds problems, 2 on any other error.
//
// gm context always returns 0, and it prints nothing outside a repository
// with notes. Claude Code runs it as a SessionStart hook in every directory,
// and a failing hook would get in the way of the session.
func run(args []string, dir string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		fmt.Fprint(stdout, usage)
		return 0
	}
	if args[0] == "--version" {
		fmt.Fprintln(stdout, "gm", version)
		return 0
	}
	cmd, args := args[0], args[1:]
	st, err := memory.Open(dir)
	if cmd == "context" {
		if err == nil && len(st.Files()) > 0 {
			_ = context(st, args, stdout)
		}
		return 0
	}
	if err == nil {
		err = dispatch(st, cmd, args, stdout)
	}
	var lintErr lintFailed
	switch {
	case errors.As(err, &lintErr):
		return 1
	case err != nil:
		fmt.Fprintln(stderr, "gm:", err)
		return 2
	}
	return 0
}

// lintFailed is the error dispatch returns when lint finds problems, so run
// exits 1 instead of 2. lint has already printed the problems.
type lintFailed int

func (n lintFailed) Error() string { return fmt.Sprintf("%d problems", int(n)) }

// dispatch runs the command cmd with args against st.
func dispatch(st *memory.Store, cmd string, args []string, w io.Writer) error {
	switch cmd {
	case "save":
		return save(st, args, w)
	case "retire":
		fs := flags("retire")
		why := fs.String("why", "", "why the notes no longer hold")
		if err := parse(fs, args); err != nil {
			return err
		}
		s, err := st.Retire(fs.Args(), *why)
		return saved(s, err, w)
	case "search":
		return find(st, args, w)
	case "show":
		return show(st, args, w)
	case "lint":
		fs := flags("lint")
		review, asJSON := fs.Bool("review", false, "also list anchors whose code changed since"), jsonFlag(fs)
		if err := parse(fs, args); err != nil {
			return err
		}
		rep := st.Lint(*review)
		if *asJSON {
			if rep.Problems == nil {
				rep.Problems = []string{}
			}
			emit(w, rep)
		} else {
			for _, p := range rep.Problems {
				fmt.Fprintln(w, p)
			}
			for _, r := range rep.Review {
				fmt.Fprintln(w, "review:", r)
			}
			for _, r := range rep.Warnings {
				fmt.Fprintln(w, "warning:", r)
			}
		}
		if len(rep.Problems) > 0 {
			return lintFailed(len(rep.Problems))
		}
		return nil
	case "list":
		return list(st, args, w)
	case "import-engram":
		if len(args) != 1 {
			return errors.New("import-engram needs the export file")
		}
		res, err := engram.Import(st, args[0])
		if err != nil {
			return err
		}
		fmt.Fprintf(w, "imported %d notes with %d supersedes links; other relations dropped\n", res.Notes, res.Links)
		for _, r := range res.Refused {
			fmt.Fprintln(w, "not imported, looks like a secret:", r)
		}
		return nil
	}
	return fmt.Errorf("unknown command %q; run gm help", cmd)
}

// flags returns a flag set that returns parse errors instead of printing
// usage and exiting, so run reports every error the same way.
func flags(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	return fs
}

// jsonFlag adds the --json flag to fs.
func jsonFlag(fs *flag.FlagSet) *bool { return fs.Bool("json", false, "print JSON") }

// parse parses args with fs and accepts flags and positional words in any
// order, so gm search words -n 3 works. The flag package stops at the first
// positional word, so parse moves each flag, with the value of a non-boolean
// flag, to the front. Everything after -- stays positional.
func parse(fs *flag.FlagSet, args []string) error {
	var fl, rest []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			rest = append(rest, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(a, "-") || a == "-" {
			rest = append(rest, a)
			continue
		}
		fl = append(fl, a)
		name := strings.TrimLeft(a, "-")
		if f := fs.Lookup(name); f != nil && !isBool(f) && i+1 < len(args) {
			fl = append(fl, args[i+1])
			i++
		}
	}
	return fs.Parse(append(append(fl, "--"), rest...))
}

// isBool reports whether f takes no value, the way the flag package decides it.
func isBool(f *flag.Flag) bool {
	b, ok := f.Value.(interface{ IsBoolFlag() bool })
	return ok && b.IsBoolFlag()
}

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

// find runs gm search.
func find(st *memory.Store, args []string, w io.Writer) error {
	fs := flags("search")
	q := search.Query{}
	fs.IntVar(&q.Limit, "n", 5, "")
	fs.StringVar(&q.Type, "type", "", "")
	fs.BoolVar(&q.All, "all", false, "")
	asJSON := jsonFlag(fs)
	if err := parse(fs, args); err != nil {
		return err
	}
	q.Words = strings.Join(fs.Args(), " ")
	hits, err := search.Find(st, q)
	if err != nil {
		return err
	}
	if *asJSON {
		emit(w, append([]search.Hit{}, hits...))
		return nil
	}
	for _, h := range hits {
		mark := ""
		if h.Superseded {
			mark = "  (superseded)"
		}
		fmt.Fprintf(w, "%s  %s %s  %s%s\n    %s\n", h.ID, h.Type, day(h.Created), h.Title, mark, h.File)
	}
	return nil
}

// code is the code of one anchor as gm show prints it. From is the number of
// the first line in Lines. When git cannot read the code, for example because
// the commit is gone, Error is set instead.
type code struct {
	Anchor string   `json:"anchor"`
	From   int      `json:"from,omitempty"`
	Lines  []string `json:"lines,omitempty"`
	Error  string   `json:"error,omitempty"`
}

// show runs gm show. It prints the note file, then the code of each anchor,
// read at the anchor's commit. For a line range it prints those lines. For a
// whole file it prints the first 40 lines.
func show(st *memory.Store, args []string, w io.Writer) error {
	fs := flags("show")
	asJSON := jsonFlag(fs)
	if err := parse(fs, args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("show needs one note id")
	}
	n, err := st.Get(fs.Arg(0))
	if err != nil {
		return err
	}
	var codes []code
	for _, s := range n.At {
		c := code{Anchor: s, From: 1}
		a, err := anchor.Parse(s)
		if err == nil {
			c.Lines, err = anchor.Code(st.Git, a)
		}
		switch {
		case err != nil:
			c.Error = err.Error()
		case a.From > 0:
			c.From, c.Lines = a.From, c.Lines[a.From-1:]
		case len(c.Lines) > 40:
			c.Lines = c.Lines[:40]
		}
		codes = append(codes, c)
	}
	if *asJSON {
		emit(w, struct {
			note.Note
			Code []code `json:"code"`
		}{n, codes})
		return nil
	}
	fmt.Fprint(w, n.Render())
	for _, c := range codes {
		if c.Error != "" {
			fmt.Fprintf(w, "\n# %s: %s\n", c.Anchor, c.Error)
			continue
		}
		fmt.Fprintf(w, "\n# %s\n", c.Anchor)
		for i, line := range c.Lines {
			fmt.Fprintf(w, "%5d  %s\n", c.From+i, line)
		}
	}
	return nil
}

// list runs gm list. It shows live notes unless --all is set. --by type groups
// them by type, newest first in each group. --by time shows them newest first.
// --json leaves out the bodies, because gm show prints a body.
func list(st *memory.Store, args []string, w io.Writer) error {
	fs := flags("list")
	by := fs.String("by", "type", "")
	limit := fs.Int("n", 0, "")
	all := fs.Bool("all", false, "")
	asJSON := jsonFlag(fs)
	if err := parse(fs, args); err != nil {
		return err
	}
	if *by != "type" && *by != "time" {
		return errors.New("--by must be type or time")
	}
	ns, _ := st.Load()
	if !*all {
		ns = memory.Live(ns)
	}
	slices.SortStableFunc(ns, func(a, b note.Note) int {
		byType := 0
		if *by == "type" {
			byType = cmp.Compare(a.Type, b.Type)
		}
		return cmp.Or(byType, cmp.Compare(b.Created, a.Created))
	})
	if *limit > 0 && len(ns) > *limit {
		ns = ns[:*limit]
	}
	if *asJSON {
		for i := range ns {
			ns[i].Body = ""
		}
		emit(w, append([]note.Note{}, ns...))
		return nil
	}
	cur := ""
	for _, n := range ns {
		if *by == "type" && n.Type != cur {
			cur = n.Type
			fmt.Fprintf(w, "\n%s\n", cur)
		}
		fmt.Fprintf(w, "- %s  %s  (%s)\n", day(n.Created), n.Title, n.ID)
	}
	return nil
}

// context runs gm context. It prints one line on how to use gm, then the 15
// newest live notes. An -n flag in args replaces the 15.
func context(st *memory.Store, args []string, w io.Writer) error {
	fmt.Fprintln(w, "Project memory (gm), recent notes. Search with `gm search WORDS`, read one with `gm show ID`; the memory skill says when to save.")
	return list(st, append([]string{"--by", "time", "-n", "15"}, args...), w)
}

// emit writes v to w as indented JSON.
func emit(w io.Writer, v any) {
	e := json.NewEncoder(w)
	e.SetIndent("", "  ")
	_ = e.Encode(v)
}

// day returns the date part of an RFC 3339 time.
func day(created string) string { return created[:min(10, len(created))] }
