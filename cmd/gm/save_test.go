package main

import (
	"regexp"
	"strings"
	"testing"
)

func TestSave(t *testing.T) {
	t.Run("writes .memory/notes/ID-SLUG.md and prints the id and the path", func(t *testing.T) {
		r := newRepo(t)
		out := r.ok("save", "fact", "--title", "Fetch retries 3 times!", "--keywords", "a b c", "--fact", "f", "--evidence", "e")
		m := regexp.MustCompile(`^(\d{8}T\d{6}Z-[0-9a-f]{6}) (\.memory/notes/\d{8}T\d{6}Z-[0-9a-f]{6}-fetch-retries-3-times\.md)\n$`).FindStringSubmatch(out)
		if m == nil || !strings.HasPrefix(m[2], ".memory/notes/"+m[1]) {
			t.Fatalf("want ID and .memory/notes/ID-SLUG.md, got %q", out)
		}
		r.read(m[2])
	})
	t.Run("renders the header and one section per flag in template order", func(t *testing.T) {
		r := newRepo(t)
		out := r.ok("save", "decision", "--rejected", "Redis", "--why", "One database", "--decision", "Postgres",
			"--title", `Cache uses "Postgres"`, "--keywords", "cache store database", "--topic", "decision/cache")
		id, file := strings.Fields(out)[0], strings.Fields(out)[1]
		got := regexp.MustCompile(`created: \S+`).ReplaceAllString(r.read(file), "created: T")
		want := "---\nid: " + id + "\ntype: decision\ntitle: \"Cache uses \\\"Postgres\\\"\"\ntopic: decision/cache\n" +
			"keywords: cache store database\ncreated: T\n---\n## Decision\nPostgres\n\n## Why\nOne database\n\n## Rejected\nRedis\n"
		if got != want {
			t.Fatalf("want\n%s\ngot\n%s", want, got)
		}
	})
	t.Run("refuses an unknown type", func(t *testing.T) {
		refused(t, newRepo(t).gm("save", "idea", "--title", "x"), "decision, lesson or fact")
	})
	t.Run("refuses a word that belongs to no flag", func(t *testing.T) {
		r := newRepo(t)
		refused(t, r.gm("save", "fact", "--title", "Fetch", "retries", "--keywords", "a b c", "--fact", "f", "--evidence", "e"), "quote")
		if len(r.notes()) != 0 {
			t.Fatal("want nothing written")
		}
	})
	t.Run("refuses a note with every broken rule at once and writes nothing", func(t *testing.T) {
		r := newRepo(t)
		res := r.gm("save", "decision", "--title", strings.Repeat("t", 121), "--keywords", "two words", "--topic", "two words",
			"--decision", "d", "--why", strings.Repeat("w", 801), "--rejected", "## not a heading")
		refused(t, res, "title", "at least 3 words", "topic", "section Why is over 800", `"## "`, "section Rejected")
		if len(r.notes()) != 0 {
			t.Fatal("want nothing written")
		}
	})
	t.Run("refuses an empty section and names its flag", func(t *testing.T) {
		refused(t, newRepo(t).gm("save", "lesson", "--title", "t", "--keywords", "a b c", "--happened", "h", "--cause", "c"), "section Rule now is empty (flag --rule)")
	})
	t.Run("refuses a supersedes id that names no note", func(t *testing.T) {
		refused(t, newRepo(t).gm("save", "fact", "--title", "t", "--keywords", "a b c", "--fact", "f", "--evidence", "e",
			"--supersedes", "20200101T000000Z-000000"), "supersedes unknown note 20200101T000000Z-000000")
	})
	t.Run("warns but saves when other live notes share the topic", func(t *testing.T) {
		r := newRepo(t)
		first := r.decision("Cache uses Postgres")
		res := r.gm("save", "decision", "--title", "Cache uses Redis", "--keywords", "cache store choice", "--topic", "decision/cache",
			"--decision", "d", "--why", "w", "--rejected", "r")
		if res.code != 0 || len(r.notes()) != 2 {
			t.Fatalf("want the note saved, got %s", res)
		}
		has(t, res.stdout+res.stderr, "topic decision/cache has other live notes: "+first)
	})
}
