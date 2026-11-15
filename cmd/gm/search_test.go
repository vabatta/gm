package main

import (
	"fmt"
	"strings"
	"testing"
)

func TestSearch(t *testing.T) {
	search := func(r *repo, args ...string) []string {
		return ids(t, r.ok(append([]string{"search", "--json"}, args...)...))
	}
	t.Run("matches notes that hold any query word and ranks title matches first", func(t *testing.T) {
		r := newRepo(t)
		body := r.ok("save", "fact", "--title", "Workers share one queue", "--keywords", "jobs backlog pool", "--fact", "the proxy budget is per job", "--evidence", "e")
		title := r.fact("Proxy budget ignores bytes")
		r.fact("Unrelated")
		equal(t, search(r, "proxy", "budget"), []string{title, strings.Fields(body)[0]})
	})
	t.Run("ignores case and common English words", func(t *testing.T) {
		r := newRepo(t)
		id := r.fact("Proxy budget ignores bytes")
		equal(t, search(r, "WHAT", "is", "the", "PROXY"), []string{id})
	})
	t.Run("exits 2 when the query keeps no word", func(t *testing.T) {
		refused(t, newRepo(t).gm("search", "what", "is", "the"), "empty query")
	})
	t.Run("finds notes by topic and anchor path words", func(t *testing.T) {
		r := newRepo(t)
		id := r.ok("save", "decision", "--title", "T", "--keywords", "a b c", "--topic", "decision/storage-engine",
			"--decision", "d", "--why", "w", "--rejected", "r", "--at", "fetch.go")
		equal(t, search(r, "engine"), []string{strings.Fields(id)[0]})
		equal(t, search(r, "fetch"), []string{strings.Fields(id)[0]})
	})
	t.Run("ranks notes with equal scores oldest first", func(t *testing.T) {
		r := newRepo(t)
		r.note("20200101T000000Z-bbbbbb", "type: fact\ntitle: \"Proxy\"\nkeywords: a b c\ncreated: 2020-01-01T00:00:00Z\n", "## Fact\nf\n\n## Evidence\ne\n")
		r.note("20210101T000000Z-aaaaaa", "type: fact\ntitle: \"Proxy\"\nkeywords: a b c\ncreated: 2021-01-01T00:00:00Z\n", "## Fact\nf\n\n## Evidence\ne\n")
		equal(t, search(r, "proxy"), []string{"20200101T000000Z-bbbbbb", "20210101T000000Z-aaaaaa"})
	})
	t.Run("--all includes superseded notes and marks them", func(t *testing.T) {
		r := newRepo(t)
		old := r.fact("Proxy budget v1")
		r.fact("Proxy budget v2", "--supersedes", old)
		has(t, r.ok("search", "proxy", "--all"), old, "(superseded)")
	})
	t.Run("finds a retirement without --all and does not mark it superseded", func(t *testing.T) {
		r := newRepo(t)
		r.ok("retire", r.fact("Flags in Flagsmith"), "--why", "Flags now live in the config table")
		out := r.ok("search", "config", "table")
		has(t, out, "retirement", "Retired: Flags in Flagsmith")
		lacks(t, out, "(superseded)")
	})
	t.Run("--type matches only notes of that type", func(t *testing.T) {
		r := newRepo(t)
		r.fact("Cache facts")
		id := r.decision("Cache uses Postgres")
		equal(t, search(r, "cache", "--type", "decision"), []string{id})
	})
	t.Run("-n prints at most N results and the default is 5", func(t *testing.T) {
		r := newRepo(t)
		for i := range 7 {
			r.fact(fmt.Sprintf("Proxy note %d", i))
		}
		if n := len(search(r, "proxy")); n != 5 {
			t.Fatalf("want 5, got %d", n)
		}
		if n := len(search(r, "proxy", "-n", "2")); n != 2 {
			t.Fatalf("want 2, got %d", n)
		}
	})
	t.Run("a negative -n exits 2", func(t *testing.T) {
		r := newRepo(t)
		r.fact("Fetch")
		refused(t, r.gm("search", "fetch", "-n", "-1"), "-n")
	})
	t.Run("--all marks a note deleted without a successor as pruned, not superseded", func(t *testing.T) {
		r := newRepo(t)
		r.fact("Fetch retries twice")
		r.commit("note")
		r.git("rm", "-q", strings.TrimPrefix(r.notes()[0], r.dir+"/"))
		r.commit("delete")
		out := r.ok("search", "fetch", "--all")
		has(t, out, "(pruned in "+r.head()+")")
		lacks(t, out, "superseded")
	})
	t.Run("--json marks a superseded result", func(t *testing.T) {
		r := newRepo(t)
		old := r.fact("Fetch retries twice")
		r.fact("Fetch retries five times", "--supersedes", old)
		has(t, r.ok("search", "fetch", "--all", "--json"), `"superseded": true`)
	})
	t.Run("takes flags and query words in any order", func(t *testing.T) {
		r := newRepo(t)
		r.fact("Fetch retries twice")
		r.fact("Fetch retries five times")
		equal(t, ids(t, r.ok("search", "fetch", "-n", "1", "--json", "retries")), ids(t, r.ok("search", "--json", "-n", "1", "fetch", "retries")))
		if n := len(ids(t, r.ok("search", "fetch", "-n", "1", "--json", "retries"))); n != 1 {
			t.Fatalf("want 1 result, got %d", n)
		}
	})
	t.Run("takes every argument after -- as a query word", func(t *testing.T) {
		r := newRepo(t)
		id := r.fact("Fetch retries twice")
		equal(t, ids(t, r.ok("search", "--json", "--", "fetch")), []string{id})
	})
	t.Run("prints the id, type, date and title, then the file path", func(t *testing.T) {
		r := newRepo(t)
		out := r.ok("save", "fact", "--title", "Proxy budget", "--keywords", "a b c", "--fact", "f", "--evidence", "e")
		id, file := strings.Fields(out)[0], strings.Fields(out)[1]
		if got, want := r.ok("search", "proxy"), id+"  fact "+id[:4]+"-"+id[4:6]+"-"+id[6:8]+"  Proxy budget\n    "+file+"\n"; got != want {
			t.Fatalf("want %q, got %q", want, got)
		}
	})
	t.Run("--json prints id, type, created, title, file and score", func(t *testing.T) {
		r := newRepo(t)
		r.fact("Proxy budget")
		has(t, r.ok("search", "proxy", "--json"), `"id"`, `"type": "fact"`, `"created"`, `"title": "Proxy budget"`, `"file": ".agents/memory/`, `"score"`)
	})
}
