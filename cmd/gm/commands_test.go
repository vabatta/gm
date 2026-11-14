package main

import (
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
	t.Run("an unknown command exits 2", func(t *testing.T) {
		refused(t, newRepo(t).gm("frobnicate"), "frobnicate")
	})
	t.Run("a command outside a git worktree exits 2", func(t *testing.T) {
		refused(t, gmIn(t, t.TempDir(), "list"), "git worktree")
	})
}
