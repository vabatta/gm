package main

import (
	"testing"
)

func TestRevisions(t *testing.T) {
	t.Run("branches that save in parallel merge without conflicts and both notes stay live", func(t *testing.T) {
		r := newRepo(t)
		for _, b := range []string{"a", "b"} {
			r.git("checkout", "-qb", b, "main")
			r.fact("Note from "+b, "--at", "fetch.go")
			r.commit("note " + b)
		}
		r.git("checkout", "-q", "main")
		r.git("merge", "-q", "--no-edit", "a")
		r.git("merge", "-q", "--no-edit", "b")
		if n := len(ids(t, r.ok("list", "--json"))); n != 2 {
			t.Fatalf("want 2 live notes, got %d", n)
		}
		r.ok("lint")
	})
	t.Run("a revision hides the note it supersedes from search, list and context", func(t *testing.T) {
		r := newRepo(t)
		old := r.fact("Fetch retries twice")
		r.fact("Fetch retries three times", "--supersedes", old)
		for _, args := range [][]string{{"search", "fetch"}, {"list"}, {"context"}} {
			lacks(t, r.ok(args...), old)
		}
	})
	t.Run("parallel revisions of one topic fail lint until one note supersedes both", func(t *testing.T) {
		r := newRepo(t)
		a, b := r.decision("Cache uses Postgres"), r.decision("Cache uses Redis")
		res := r.gm("lint")
		if res.code != 1 {
			t.Fatalf("want exit 1, got %s", res)
		}
		has(t, res.stdout, "topic decision/cache has 2 live notes", a, b)
		r.decision("Cache uses Postgres behind Redis", a, b)
		r.ok("lint")
	})
}
