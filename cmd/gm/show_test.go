package main

import (
	"fmt"
	"strings"
	"testing"
)

func TestShow(t *testing.T) {
	t.Run("prints the note, then the numbered anchored lines read at the anchor's commit", func(t *testing.T) {
		r := newRepo(t)
		id := r.fact("Fetch is empty", "--at", "fetch.go:3")
		r.write("fetch.go", "package x\n\nfunc Fetch() { retry() }\n")
		r.commit("retry")
		out := r.ok("show", id)
		has(t, out, "title: \"Fetch is empty\"", "\n# "+r.git("rev-parse", "--short=12", "HEAD~1")+":fetch.go:3\n    3  func Fetch() {}\n")
		lacks(t, out, "retry()")
	})
	t.Run("prints the first 40 lines of a whole-file anchor", func(t *testing.T) {
		r := newRepo(t)
		var b strings.Builder
		for i := 1; i <= 50; i++ {
			fmt.Fprintf(&b, "line %d\n", i)
		}
		r.write("long.txt", b.String())
		r.commit("long")
		out := r.ok("show", r.fact("Long file", "--at", "long.txt"))
		has(t, out, "   40  line 40\n")
		lacks(t, out, "line 41")
	})
	t.Run("prints the reason when git cannot read an anchor's code", func(t *testing.T) {
		r := newRepo(t)
		r.note("20200101T000000Z-aaaaaa", "type: fact\ntitle: \"Gone\"\nkeywords: a b c\nat: [deadbeefdead:gone.go]\ncreated: 2020-01-01T00:00:00Z\n", "## Fact\nf\n\n## Evidence\ne\n")
		has(t, r.ok("show", "20200101T000000Z-aaaaaa"), "\n# deadbeefdead:gone.go: ")
	})
	t.Run("prints the reason when the file at the base has fewer lines than the anchor", func(t *testing.T) {
		r := newRepo(t)
		r.note("20200101T000000Z-aaaaaa", "type: fact\ntitle: \"Short\"\nkeywords: a b c\nat: ["+r.head()+":fetch.go:8-9]\ncreated: 2020-01-01T00:00:00Z\n", "## Fact\nf\n\n## Evidence\ne\n")
		has(t, r.ok("show", "20200101T000000Z-aaaaaa"), "\n# "+r.head()+":fetch.go:8-9: ", "3 lines")
	})
	t.Run("names HEAD as the base, and why, for a note not committed yet whose anchor commit is gone", func(t *testing.T) {
		r := newRepo(t)
		id := r.fact("Fetch", "--at", "fetch.go:3")
		r.git("commit", "-q", "--amend", "-m", "code, reworded")
		has(t, r.ok("show", id), "(read at HEAD, because the note is not committed yet)\n", "    3  func Fetch() {}")
		has(t, r.ok("show", id, "--json"), `"read_at": "HEAD"`)
	})
	t.Run("names HEAD as the base, and why, in a shallow clone that lacks the anchor's commit", func(t *testing.T) {
		r := newRepo(t)
		id := r.fact("Fetch", "--at", "fetch.go:3")
		r.commit("note")
		r.write("other.go", "package x\n")
		r.commit("more")
		clone := &repo{t, t.TempDir()}
		clone.git("clone", "-q", "--depth", "1", "file://"+r.dir, ".")
		has(t, clone.ok("show", id), "(read at HEAD, because this shallow clone lacks commit ")
	})
	t.Run("--json reports read_at after a squash merge, and error when git cannot read the code", func(t *testing.T) {
		clone, id := merged(t, "squash")
		has(t, clone.ok("show", id, "--json"), `"read_at": "`+clone.head()+`"`)
		r := newRepo(t)
		r.note("20200101T000000Z-aaaaaa", "type: fact\ntitle: \"Gone\"\nkeywords: a b c\nat: [deadbeefdead:gone.go]\ncreated: 2020-01-01T00:00:00Z\n", "## Fact\nf\n\n## Evidence\ne\n")
		has(t, r.ok("show", "20200101T000000Z-aaaaaa", "--json"), `"error": "`)
	})
	t.Run("exits 2 for an unknown id", func(t *testing.T) {
		refused(t, newRepo(t).gm("show", "20200101T000000Z-000000"), "no note")
	})
	t.Run("--json prints the note's fields and its code", func(t *testing.T) {
		r := newRepo(t)
		out := r.ok("show", r.fact("Fetch", "--at", "fetch.go:3"), "--json")
		has(t, out, `"title": "Fetch"`, `"code": [`, `"from": 3`, `"func Fetch() {}"`)
	})
}
