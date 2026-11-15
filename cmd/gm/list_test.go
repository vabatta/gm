package main

import (
	"strings"
	"testing"
)

func TestList(t *testing.T) {
	hand := func(r *repo, id, typ, created string) {
		body := map[string]string{"fact": "## Fact\nf\n\n## Evidence\ne\n", "decision": "## Decision\nd\n\n## Why\nw\n\n## Rejected\nr\n"}[typ]
		r.note(id, "type: "+typ+"\ntitle: \""+id+"\"\nkeywords: a b c\ncreated: "+created+"\n", body)
	}
	seed := func(t *testing.T) *repo {
		r := newRepo(t)
		hand(r, "20200101T000000Z-000001", "fact", "2020-01-01T00:00:00Z")
		hand(r, "20200102T000000Z-000002", "decision", "2020-01-02T00:00:00Z")
		hand(r, "20200103T000000Z-000003", "fact", "2020-01-03T00:00:00Z")
		return r
	}
	t.Run("groups live notes by type, newest first in each group", func(t *testing.T) {
		got := seed(t).ok("list")
		want := "\ndecision\n- 2020-01-02  20200102T000000Z-000002  (20200102T000000Z-000002)\n" +
			"\nfact\n- 2020-01-03  20200103T000000Z-000003  (20200103T000000Z-000003)\n- 2020-01-01  20200101T000000Z-000001  (20200101T000000Z-000001)\n"
		if got != want {
			t.Fatalf("want\n%q\ngot\n%q", want, got)
		}
	})
	t.Run("--by time lists newest first without groups", func(t *testing.T) {
		equal(t, ids(t, seed(t).ok("list", "--by", "time", "--json")), []string{"20200103T000000Z-000003", "20200102T000000Z-000002", "20200101T000000Z-000001"})
	})
	t.Run("-n lists at most N notes", func(t *testing.T) {
		equal(t, ids(t, seed(t).ok("list", "--by", "time", "-n", "1", "--json")), []string{"20200103T000000Z-000003"})
	})
	t.Run("--by with another value than type or time exits 2", func(t *testing.T) {
		refused(t, newRepo(t).gm("list", "--by", "size"), "--by")
	})
	t.Run("a negative -n exits 2", func(t *testing.T) {
		refused(t, newRepo(t).gm("list", "-n", "-1"), "-n")
	})
	t.Run("lists retirements, which are live, and --all adds superseded notes", func(t *testing.T) {
		r := newRepo(t)
		id := r.fact("Old")
		retirement := strings.Fields(r.ok("retire", id, "--why", "gone"))[0]
		equal(t, ids(t, r.ok("list", "--json")), []string{retirement})
		if n := len(ids(t, r.ok("list", "--all", "--json"))); n != 2 {
			t.Fatalf("want the note and its retirement, got %d", n)
		}
	})
	t.Run("--json lists notes without their bodies", func(t *testing.T) {
		lacks(t, seed(t).ok("list", "--json"), "## Fact")
	})
}
