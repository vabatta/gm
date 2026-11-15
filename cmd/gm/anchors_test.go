package main

import (
	"strings"
	"testing"
)

func TestAnchors(t *testing.T) {
	t.Run("pins the anchor to HEAD with 12 hex characters", func(t *testing.T) {
		r := newRepo(t)
		r.fact("Fetch lives here", "--at", "fetch.go:3", "--at", "./fetch.go")
		has(t, r.read(strings.TrimPrefix(r.notes()[0], r.dir+"/")), "at: ["+r.head()+":fetch.go:3, "+r.head()+":fetch.go]")
	})
	t.Run("refuses a file with uncommitted changes", func(t *testing.T) {
		r := newRepo(t)
		r.write("fetch.go", "package x\n\nfunc Fetch() { retry() }\n")
		refused(t, r.gm("save", "fact", "--title", "t", "--keywords", "a b c", "--fact", "f", "--evidence", "e", "--at", "fetch.go"), "uncommitted changes")
	})
	t.Run("refuses a file that is not committed at HEAD", func(t *testing.T) {
		r := newRepo(t)
		r.write("new.go", "package x\n")
		refused(t, r.gm("save", "fact", "--title", "t", "--keywords", "a b c", "--fact", "f", "--evidence", "e", "--at", "new.go"), "not committed at HEAD")
	})
	t.Run("refuses a path that names a directory", func(t *testing.T) {
		r := newRepo(t)
		r.write("dir/a.go", "package dir\n")
		r.commit("dir")
		refused(t, r.gm("save", "fact", "--title", "t", "--keywords", "a b c", "--fact", "f", "--evidence", "e", "--at", "dir"), "names a directory")
	})
	t.Run("refuses an empty path", func(t *testing.T) {
		refused(t, newRepo(t).gm("save", "fact", "--title", "t", "--keywords", "a b c", "--fact", "f", "--evidence", "e", "--at", ""), "needs a path")
	})
	t.Run("refuses a line range that is malformed or ends past the last line", func(t *testing.T) {
		for spec, want := range map[string]string{"fetch.go:0": "lines must be", "fetch.go:3-2": "lines must be", "fetch.go:2-9": "has 3 lines"} {
			refused(t, newRepo(t).gm("save", "fact", "--title", "t", "--keywords", "a b c", "--fact", "f", "--evidence", "e", "--at", spec), want)
		}
	})
	t.Run("takes a path containing : or , whole when it names a committed file", func(t *testing.T) {
		r := newRepo(t)
		r.write("a:b, c.txt", "1\n2\n3\n")
		r.write("log:12", "x\n")
		r.commit("odd names")
		for spec, shown := range map[string]string{"a:b, c.txt:2-3": "    2  2\n    3  3\n", "log:12": "    1  x\n"} {
			id := r.fact("Odd file "+spec, "--at", spec)
			has(t, r.ok("show", id), shown)
		}
		if out := r.ok("lint"); out != "" {
			t.Fatalf("want a clean lint, got %q", out)
		}
	})
}
