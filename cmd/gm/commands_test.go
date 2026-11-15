package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCommands(t *testing.T) {
	t.Run("help, -h and --help print a summary of the commands", func(t *testing.T) {
		for _, arg := range []string{"help", "-h", "--help"} {
			res := gmIn(t, t.TempDir(), arg)
			if res.code != 0 {
				t.Fatalf("%s: %s", arg, res)
			}
			has(t, res.stdout, "gm save", "gm search", "gm lint")
		}
	})
	t.Run("--version prints the version", func(t *testing.T) {
		res := gmIn(t, t.TempDir(), "--version")
		if res.code != 0 || strings.TrimSpace(res.stdout) == "" {
			t.Fatalf("want a version, got %s", res)
		}
	})
	t.Run("takes anchor paths from the worktree root when run in a subdirectory", func(t *testing.T) {
		r := newRepo(t)
		r.write("sub/a.go", "package sub\n")
		r.commit("sub")
		res := gmIn(t, filepath.Join(r.dir, "sub"), "save", "fact", "--title", "Sub", "--keywords", "a b c", "--fact", "f", "--evidence", "e", "--at", "sub/a.go")
		if res.code != 0 {
			t.Fatalf("want exit 0, got %s", res)
		}
		has(t, r.read(strings.TrimPrefix(r.notes()[0], r.dir+"/")), ":sub/a.go]")
	})
	t.Run("an unknown command exits 2", func(t *testing.T) {
		refused(t, newRepo(t).gm("frobnicate"), "frobnicate")
	})
	t.Run("a command outside a git worktree exits 2", func(t *testing.T) {
		refused(t, gmIn(t, t.TempDir(), "list"), "git worktree")
	})
}
