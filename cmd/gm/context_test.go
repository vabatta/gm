package main

import (
	"fmt"
	"strings"
	"testing"
)

func TestContext(t *testing.T) {
	t.Run("prints nothing and exits 0 outside a git worktree", func(t *testing.T) {
		if res := gmIn(t, t.TempDir(), "context"); res.code != 0 || res.stdout+res.stderr != "" {
			t.Fatalf("want silence and exit 0, got %s", res)
		}
	})
	t.Run("prints nothing and exits 0 in a repository without notes", func(t *testing.T) {
		if res := newRepo(t).gm("context"); res.code != 0 || res.stdout+res.stderr != "" {
			t.Fatalf("want silence and exit 0, got %s", res)
		}
	})
	t.Run("prints a line about gm, then the 15 newest live notes", func(t *testing.T) {
		r := newRepo(t)
		for day := 1; day <= 16; day++ {
			r.note(fmt.Sprintf("202001%02dT000000Z-0000%02d", day, day), fmt.Sprintf("type: fact\ntitle: \"Note %d\"\nkeywords: a b c\ncreated: 2020-01-%02dT00:00:00Z\n", day, day), "## Fact\nf\n\n## Evidence\ne\n")
		}
		out := r.ok("context")
		lines := strings.Split(strings.TrimSpace(out), "\n")
		if len(lines) != 16 || !strings.Contains(lines[0], "gm search") || !strings.Contains(lines[1], "Note 16") || strings.Contains(out, "Note 1 ") {
			t.Fatalf("want a header and notes 16 down to 2, got\n%s", out)
		}
		if n := len(strings.Split(strings.TrimSpace(r.ok("context", "-n", "3")), "\n")); n != 4 {
			t.Fatalf("want a header and 3 notes with -n 3, got %d lines", n)
		}
	})
	t.Run("exits 0 even when the notes are broken", func(t *testing.T) {
		r := newRepo(t)
		r.write(".memory/notes/stray.md", "hello\n")
		if res := r.gm("context", "--bogus"); res.code != 0 {
			t.Fatalf("want exit 0, got %s", res)
		}
	})
}
