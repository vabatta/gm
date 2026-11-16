package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// pathCase is one anchored file of the path matrix: how to make it, how to
// change it, and what gm(1) and gm-note(5) say an anchor to it looks like.
type pathCase struct {
	name    string   // the subtest label
	file    string   // the file name on disk and in git
	spec    string   // what --at gets for the whole file; file when empty
	path    string   // the anchor path that spec names; file when empty
	content string   // the committed content
	changed string   // the content after a change to line 1
	lines   []string // the lines of content
	rng     string   // the line range to anchor; empty when no range fits
	refuse  string   // the refusal save prints, for a path save refuses
	sibling string   // another committed file, changed while the anchored one is not
	word    string   // a word of the path to search for
	link    string   // the target, for a symlink
	crlf    bool     // compare code lines without their CR
}

// lineShape matches a path that ends in a part shaped like :N or :N-M.
var lineShape = regexp.MustCompile(`:[0-9]+(-[0-9]+)?$`)

func pathCases() []pathCase {
	two := []string{"one", "two"}
	plain := func(name, file, word string) pathCase {
		return pathCase{name: name, file: file, content: "one\ntwo\n", changed: "ONE\ntwo\n", lines: two, rng: "1-2", word: word}
	}
	cs := []pathCase{
		plain("a.go", "a.go", "go"),
		plain("dir/sub/a.go", "dir/sub/a.go", "sub"),
		plain("my file.go", "my file.go", "file"),
		plain("a  b.go", "a  b.go", "go"),
		plain(" lead.go", " lead.go", "lead"),
		plain("trail.go ", "trail.go ", "trail"),
		plain(`a\tb.go`, "a\tb.go", "go"),
		plain("a:b.go", "a:b.go", "go"),
		plain("log:12", "log:12", "log"),
		plain("log:1-2", "log:1-2", "log"),
		plain("log:", "log:", "log"),
		plain("x blob", "x blob", "blob"),
		plain("x tree", "x tree", "tree"),
		plain("x commit", "x commit", "commit"),
		plain("x tag", "x tag", "tag"),
		plain("x missing", "x missing", "missing"),
		plain("a[1].go", "a[1].go", "go"),
		plain(`q"x.go`, `q"x.go`, "go"),
		plain("q'x.go", "q'x.go", "go"),
		plain("a,b.go", "a,b.go", "go"),
		plain(`a\b.go`, `a\b.go`, "go"),
		plain("*.go", "*.go", "go"),
		plain("a?.go", "a?.go", "go"),
		plain("café.go NFC", "café.go", "café"),
		plain("café.go NFD", "café.go", "go"),
		plain("emoji", "\U0001F642.go", "go"),
		plain("CJK", "日本語.go", "日本語"),
		plain("200 characters", strings.Repeat("n", 197)+".go", "go"),
		plain(".env.example", ".env.example", "example"),
		plain("-x.go", "-x.go", "go"),
		plain("./a.go", "a.go", "go"),
		plain("symlink", "link.go", "link"),
		plain("empty file", "empty.txt", "empty"),
		plain("trailing blank lines", "blank.txt", "blank"),
		plain("no final newline", "nofinal.txt", "nofinal"),
		plain("CRLF", "crlf.txt", "crlf"),
		plain("binary", "bin.dat", "dat"),
		{name: `a\nb.go`, file: "a\nb.go", content: "one\n", refuse: "line break"},
		{name: `a\rb.go`, file: "a\rb.go", content: "one\n", refuse: "line break"},
		{name: "directory", file: "dir", content: "one\n", refuse: "names a directory"},
	}
	for i := range cs {
		c := &cs[i]
		switch c.name {
		case "*.go":
			c.sibling = "z.go"
		case "a?.go":
			c.sibling = "ab.go"
		case "a[1].go":
			c.sibling = "a1.go"
		case "./a.go":
			c.spec, c.path = "./a.go", "a.go"
		case "symlink":
			c.link, c.content, c.changed, c.lines, c.rng = "a.go", "a.go", "b.go", []string{"a.go"}, "1"
		case "empty file":
			c.content, c.changed, c.lines, c.rng = "", "now\n", nil, ""
		case "trailing blank lines":
			c.content, c.changed, c.lines, c.rng = "one\ntwo\n\n\n", "ONE\ntwo\n\n\n", []string{"one", "two", "", ""}, "1-4"
		case "no final newline":
			c.content, c.changed = "one\ntwo", "ONE\ntwo"
		case "CRLF":
			c.content, c.changed, c.crlf = "one\r\ntwo\r\n", "ONE\r\ntwo\r\n", true
		case "binary":
			c.content, c.changed, c.lines = "\x00bin\n\xfftwo\n", "\x00BIN\n\xfftwo\n", []string{"\x00bin", "\xfftwo"}
		}
		if c.spec == "" {
			c.spec = c.file
		}
		if c.path == "" {
			c.path = c.file
		}
		if c.sibling == "" {
			c.sibling = "other.txt"
		}
	}
	return cs
}

// setup commits the case's file and its sibling.
func (c pathCase) setup(t *testing.T) *repo {
	t.Helper()
	r := newRepo(t)
	switch {
	case c.link != "":
		r.write(c.link, "package x\n")
		r.write("b.go", "package y\n")
		if err := os.Symlink(c.link, filepath.Join(r.dir, c.file)); err != nil {
			t.Fatal(err)
		}
	case c.name == "directory":
		r.write(filepath.Join(c.file, "a.go"), c.content)
	default:
		r.write(c.file, c.content)
	}
	r.write(c.sibling, "sibling\n")
	r.commit("code")
	return r
}

// change commits a new line 1 of the anchored file.
func (c pathCase) change(t *testing.T, r *repo) {
	t.Helper()
	if c.link != "" {
		p := filepath.Join(r.dir, c.file)
		if err := os.Remove(p); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(c.changed, p); err != nil {
			t.Fatal(err)
		}
	} else {
		r.write(c.file, c.changed)
	}
	r.commit("change")
}

// specs are the --at values: the whole file, then the line range if one fits.
func (c pathCase) specs() []string {
	if c.rng == "" {
		return []string{c.spec}
	}
	return []string{c.spec, c.spec + ":" + c.rng}
}

// anchors are the anchors gm-note(5) says save writes for specs at rev, with
// the path as git spells it there, as gm(1) says: macOS git stores an NFD
// name in NFC form.
func (c pathCase) anchors(r *repo, rev string) []string {
	head := r.git("rev-parse", "--short=12", rev)
	cmd := exec.Command("git", "ls-tree", "-z", "--name-only", rev, "--", ":(literal)"+c.path)
	cmd.Dir, cmd.Env = r.dir, env
	out, err := cmd.Output()
	if err != nil {
		r.t.Fatalf("git ls-tree %s: %v", c.path, err)
	}
	path := strings.TrimSuffix(string(out), "\x00")
	whole := head + ":" + path
	if strings.HasSuffix(path, ":") || lineShape.MatchString(path) {
		whole += ":"
	}
	if c.rng == "" {
		return []string{whole}
	}
	return []string{whole, head + ":" + path + ":" + c.rng}
}

// rangeLines are the lines the line-range anchor covers.
func (c pathCase) rangeLines() []string {
	from, to := 1, 1
	fmt.Sscanf(c.rng, "%d-%d", &from, &to)
	if to < from {
		to = from
	}
	return c.lines[from-1 : to]
}

// save saves a fact anchored with specs and returns its id.
func (c pathCase) save(r *repo, title string, specs []string, extra ...string) string {
	r.t.Helper()
	for _, s := range specs {
		extra = append(extra, "--at", s)
	}
	return r.fact(title, extra...)
}

// headerAt formats one anchor as the at: line gm-note(5) HEADER describes.
func headerAt(anchor string) string {
	if strings.ContainsAny(anchor, ",[]\"") || strings.TrimSpace(anchor) != anchor {
		b, _ := json.Marshal([]string{anchor})
		return "at: " + string(b)
	}
	return "at: [" + anchor + "]"
}

// shownCode is what show prints for an anchor whose code is lines from from.
func shownCode(anchor string, from int, lines []string) string {
	var b strings.Builder
	b.WriteString("\n# " + anchor + "\n")
	for i, l := range lines {
		fmt.Fprintf(&b, "%5d  %s\n", from+i, l)
	}
	return b.String()
}

type shownJSON struct {
	At   []string
	Code []struct {
		Anchor string
		From   int
		Lines  []string
		Error  string
	}
}

// lineWith returns the line of out that holds s, or fails.
func lineWith(t *testing.T, out, s string) string {
	t.Helper()
	for l := range strings.SplitSeq(out, "\n") {
		if strings.Contains(l, s) {
			return l
		}
	}
	t.Fatalf("want a line with %q in:\n%s", s, out)
	return ""
}

func lintFails(t *testing.T, r *repo) string {
	t.Helper()
	res := r.gm("lint")
	if res.code != 1 {
		t.Fatalf("want lint to exit 1, got %s", res)
	}
	return res.stdout
}

func TestMatrixPaths(t *testing.T) {
	type cell struct {
		name, refusedName string
		run               func(t *testing.T, c pathCase)
	}
	cells := []cell{
		{"save with a whole-file --at writes the note", "save with a whole-file --at refuses", func(t *testing.T, c pathCase) {
			r := c.setup(t)
			out := r.ok("save", "fact", "--title", "Anchored note", "--keywords", "fetch retry backoff", "--fact", "f", "--evidence", "e", "--at", c.spec)
			if len(r.notes()) != 1 || !strings.Contains(out, ".agents/memory/") {
				t.Fatalf("want one note and its path, got %q and %d files", out, len(r.notes()))
			}
		}},
		{"save with a line-range --at writes the note", "save with a line-range --at refuses", func(t *testing.T, c pathCase) {
			r := c.setup(t)
			if c.rng == "" {
				refused(t, r.gm("save", "fact", "--title", "t", "--keywords", "a b c", "--fact", "f", "--evidence", "e", "--at", c.spec+":1"))
				if len(r.notes()) != 0 {
					t.Fatal("want no note written")
				}
				return
			}
			c.save(r, "Anchored note", []string{c.spec + ":" + c.rng})
		}},
		{"header holds the anchor so it reads back as the same path", "header is not written", func(t *testing.T, c pathCase) {
			for i, spec := range c.specs() {
				r := c.setup(t)
				id := c.save(r, "Anchored note", []string{spec})
				anchor := c.anchors(r, "HEAD")[i]
				has(t, r.read(strings.TrimPrefix(r.notes()[0], r.dir+"/")), "\n"+headerAt(anchor)+"\n")
				var got shownJSON
				if err := json.Unmarshal([]byte(r.ok("show", id, "--json")), &got); err != nil {
					t.Fatal(err)
				}
				equal(t, got.At, []string{anchor})
			}
		}},
		{"show prints the file's code under the anchor heading", "show has no note to print", func(t *testing.T, c pathCase) {
			r := c.setup(t)
			id := c.save(r, "Anchored note", c.specs())
			out := r.ok("show", id)
			if c.crlf {
				out = strings.ReplaceAll(out, "\r", "")
			}
			as := c.anchors(r, "HEAD")
			has(t, out, shownCode(as[0], 1, c.lines))
			if len(as) > 1 {
				has(t, out, shownCode(as[1], 1, c.rangeLines()))
			}
		}},
		{"show --json returns the anchor unchanged and the code lines", "show --json has no note to print", func(t *testing.T, c pathCase) {
			r := c.setup(t)
			id := c.save(r, "Anchored note", c.specs())
			var got shownJSON
			if err := json.Unmarshal([]byte(r.ok("show", id, "--json")), &got); err != nil {
				t.Fatal(err)
			}
			as := c.anchors(r, "HEAD")
			equal(t, got.At, as)
			if len(got.Code) != len(as) {
				t.Fatalf("want %d code entries, got %+v", len(as), got.Code)
			}
			want := [][]string{c.lines, nil}
			if len(as) > 1 {
				want[1] = c.rangeLines()
			}
			for i, code := range got.Code {
				if code.Anchor != as[i] || code.From != 1 || code.Error != "" {
					t.Fatalf("want anchor %q from 1 with no error, got %+v", as[i], code)
				}
				var w, g []string
				for _, l := range want[i] {
					w = append(w, strings.ToValidUTF8(l, "�"))
				}
				for _, l := range code.Lines {
					if c.crlf {
						l = strings.TrimSuffix(l, "\r")
					}
					g = append(g, l)
				}
				if !slices.Equal(g, w) {
					t.Fatalf("anchor %q: want lines %q, got %q", as[i], w, g)
				}
			}
		}},
		{"lint is clean while the file is unchanged", "lint is clean with no note", func(t *testing.T, c pathCase) {
			r := c.setup(t)
			c.save(r, "Anchored note", c.specs())
			r.commit("note")
			r.write(c.sibling, "sibling changed\n")
			r.commit("sibling")
			for _, args := range [][]string{{"lint"}, {"lint", "--review"}} {
				if out := r.ok(args...); out != "" {
					t.Fatalf("gm %v: want no output, got %q", args, out)
				}
			}
		}},
		{"lint --review prints a review line after the anchored lines change", "lint --review has no anchor to review", func(t *testing.T, c pathCase) {
			r := c.setup(t)
			c.save(r, "Anchored note", c.specs())
			r.commit("note")
			c.change(t, r)
			out := r.ok("lint", "--review")
			for _, a := range c.anchors(r, "HEAD~2") {
				has(t, lineWith(t, out, a+" changed"), "review: ")
			}
		}},
		{"lint after delete reports the file gone", "lint after delete has no anchor to check", func(t *testing.T, c pathCase) {
			r := c.setup(t)
			c.save(r, "Anchored note", c.specs())
			r.commit("note")
			r.git("--literal-pathspecs", "rm", "-q", "--", c.file)
			r.commit("remove")
			out := lintFails(t, r)
			for _, a := range c.anchors(r, "HEAD~2") {
				has(t, out, a+": the file no longer exists at HEAD")
			}
		}},
		{"lint after rename names the new path", "lint after rename has no anchor to check", func(t *testing.T, c pathCase) {
			r := c.setup(t)
			c.save(r, "Anchored note", c.specs())
			r.commit("note")
			r.git("--literal-pathspecs", "mv", "--", c.file, "moved.go")
			r.commit("move")
			out := lintFails(t, r)
			for _, a := range c.anchors(r, "HEAD~2") {
				has(t, lineWith(t, out, a+": "), "moved.go")
			}
		}},
		{"search finds the note by a word of the path", "search finds no note", func(t *testing.T, c pathCase) {
			r := c.setup(t)
			id := c.save(r, "Anchored note", []string{c.spec})
			equal(t, ids(t, r.ok("search", c.word, "--json")), []string{id})
		}},
		{"prune keeps a superseded note until it is committed, then prunes it", "prune has no note to prune", func(t *testing.T, c pathCase) {
			r := c.setup(t)
			old := c.save(r, "Anchored note", []string{c.spec})
			c.save(r, "Anchored note revised", []string{c.spec}, "--supersedes", old)
			has(t, r.ok("prune"), "kept .agents/memory/"+old+"-")
			r.commit("notes")
			anchor := c.anchors(r, "HEAD~1")[0]
			out := r.ok("prune")
			has(t, out, "pruned .agents/memory/"+old+"-")
			if len(r.notes()) != 1 {
				t.Fatalf("want the revision left, got %d files", len(r.notes()))
			}
			r.commit("prune")
			has(t, r.ok("show", old), "# pruned in "+r.head()+"\n", "\n# "+anchor+"\n")
		}},
	}
	for _, c := range pathCases() {
		t.Run(fmt.Sprintf("path %q", c.name), func(t *testing.T) {
			t.Parallel()
			for _, cl := range cells {
				if c.refuse == "" {
					t.Run(cl.name, func(t *testing.T) { cl.run(t, c) })
					continue
				}
				t.Run(cl.refusedName, func(t *testing.T) {
					r := c.setup(t)
					for _, spec := range []string{c.spec, c.spec + ":1"} {
						refused(t, r.gm("save", "fact", "--title", "t", "--keywords", "a b c", "--fact", "f", "--evidence", "e", "--at", spec), c.refuse)
					}
					if len(r.notes()) != 0 {
						t.Fatalf("want no note written, got %v", r.notes())
					}
					if out := r.ok("lint"); out != "" {
						t.Fatalf("want a clean lint, got %q", out)
					}
					if out := r.ok("list"); strings.TrimSpace(out) != "" {
						t.Fatalf("want no notes listed, got %q", out)
					}
				})
			}
		})
	}
}

// TestMatrixNotesDir runs every command against notes directories of unusual
// shapes, set with gm.dir in .gmconfig, from DIRECTORY in gm(1).
func TestMatrixNotesDir(t *testing.T) {
	dirs := []string{".agents/memory", "docs/memory", "notes dir", "mémoire", "日本語/メモ", "a'b", "a:b", "-dash", "a*b", "a[1]", "lead/ trail "}
	for _, dir := range dirs {
		t.Run(dir, func(t *testing.T) {
			t.Parallel()
			r := newRepo(t)
			r.setDir(dir)
			r.commit("config")
			prefix := strings.TrimSuffix(filepath.ToSlash(filepath.Clean(dir)), "/") + "/"
			old := r.fact("Fetch retries twice", "--at", "fetch.go:3")
			r.commit("note")
			has(t, r.ok("search", "fetch"), old, prefix+old)
			has(t, r.ok("show", old), "Fetch retries twice", "func Fetch() {}")
			has(t, r.ok("list"), old)
			if res := r.gm("lint"); res.code != 0 {
				t.Fatalf("lint: want exit 0, got %s", res)
			}
			rev := r.fact("Fetch retries five times", "--supersedes", old)
			r.commit("revise")
			has(t, r.ok("prune"), "pruned "+prefix+old)
			r.commit("prune")
			has(t, r.ok("show", old), "pruned in")
			has(t, r.ok("search", "--all", "fetch"), old)
			has(t, r.ok("list", "--all"), old)
			r.ok("retire", rev, "--why", "Fetch no longer retries at all")
			r.fact("Fetch retries again", "--supersedes", old)
			r.commit("more")
			if res := r.gm("lint"); res.code != 0 {
				t.Fatalf("lint after prune: want exit 0, got %s", res)
			}
		})
	}
}
