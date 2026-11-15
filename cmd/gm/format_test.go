package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestNoteFormat(t *testing.T) {
	t.Run("reads CRLF line ends and ignores unknown header keys", func(t *testing.T) {
		r := newRepo(t)
		r.write(".memory/notes/20200101T000000Z-aaaaaa-crlf.md", strings.ReplaceAll("---\nid: 20200101T000000Z-aaaaaa\ntype: fact\ntitle: \"Proxy budget\"\n"+
			"keywords: a b c\ncolor: blue\ncreated: 2020-01-01T00:00:00Z\n---\n## Fact\nf\n\n## Evidence\ne\n", "\n", "\r\n"))
		equal(t, ids(t, r.ok("search", "proxy", "--json")), []string{"20200101T000000Z-aaaaaa"})
		r.ok("lint")
	})
	t.Run("reads a note that names format 1", func(t *testing.T) {
		r := newRepo(t)
		r.note("20200101T000000Z-aaaaaa", "type: fact\ntitle: \"Proxy budget\"\nkeywords: a b c\nformat: 1\ncreated: 2020-01-01T00:00:00Z\n", "## Fact\nf\n\n## Evidence\ne\n")
		equal(t, ids(t, r.ok("search", "proxy", "--json")), []string{"20200101T000000Z-aaaaaa"})
		r.ok("lint")
	})
	t.Run("leaves out a note of a format it does not know, and lint names the file and the format", func(t *testing.T) {
		r := newRepo(t)
		r.note("20200101T000000Z-aaaaaa", "type: fact\ntitle: \"Proxy budget\"\nkeywords: a b c\nformat: 2\ncreated: 2020-01-01T00:00:00Z\n", "## Fact\nf\n\n## Evidence\ne\n")
		lacks(t, r.ok("search", "proxy"), "20200101T000000Z-aaaaaa")
		lacks(t, r.ok("list"), "Proxy budget")
		res := r.gm("lint")
		if res.code != 1 {
			t.Fatalf("want lint to exit 1, got %s", res)
		}
		has(t, res.stdout, "20200101T000000Z-aaaaaa", "note format 2 needs a newer gm")
	})
	t.Run("writes a list as a JSON array when an item holds a bracket, a quote, or a trailing space", func(t *testing.T) {
		for _, name := range []string{"a[1].go", `q"x.go`, "trail.go "} {
			r := newRepo(t)
			r.write(name, "x\n")
			r.commit("odd")
			r.fact("Odd", "--at", name)
			has(t, r.read(strings.TrimPrefix(r.notes()[0], r.dir+"/")), "at: [\"")
		}
	})
	t.Run("names the file after the id and the title in lowercase, cut to 48 characters", func(t *testing.T) {
		r := newRepo(t)
		id := r.fact("Fetch Retries With Exponential Backoff And Jitter On Every Call")
		equal(t, []string{filepath.Base(r.notes()[0])}, []string{id + "-fetch-retries-with-exponential-backoff-and-jitte.md"})
	})
	t.Run("ends an anchor with one more : when its path ends in a line-range shape", func(t *testing.T) {
		r := newRepo(t)
		r.write("log:12", "one\ntwo\n")
		r.commit("odd name")
		id := r.fact("Odd", "--at", "log:12")
		has(t, r.read(strings.TrimPrefix(r.notes()[0], r.dir+"/")), "at: ["+r.head()+":log:12:]")
		has(t, r.ok("show", id), "\n# "+r.head()+":log:12:\n", "one")
	})
	t.Run("writes a list as a JSON array when an item holds a comma", func(t *testing.T) {
		r := newRepo(t)
		r.write("a, b.txt", "x\n")
		r.commit("comma")
		r.fact("Comma", "--at", "a, b.txt")
		has(t, r.read(strings.TrimPrefix(r.notes()[0], r.dir+"/")), `at: ["`+r.head()+`:a, b.txt"]`)
	})
}
