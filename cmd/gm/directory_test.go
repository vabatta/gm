package main

import (
	"os"
	"path/filepath"
	"testing"
)

// agentsRepo makes a repository whose memory directory is .agents/memory.
func agentsRepo(t *testing.T) *repo {
	t.Helper()
	r := newRepo(t)
	if err := os.MkdirAll(filepath.Join(r.dir, ".agents", "memory"), 0o755); err != nil {
		t.Fatal(err)
	}
	return r
}

func TestDirectory(t *testing.T) {
	t.Run("is .agents/memory when that directory exists", func(t *testing.T) {
		r := agentsRepo(t)
		out := r.ok("save", "fact", "--title", "Fetch retries twice", "--keywords", "fetch retry backoff", "--fact", "f", "--evidence", "e")
		has(t, out, " .agents/memory/notes/")
		if files, _ := filepath.Glob(filepath.Join(r.dir, ".agents/memory/notes/*.md")); len(files) != 1 {
			t.Fatalf("want one note in .agents/memory/notes, got %v", files)
		}
		has(t, r.ok("search", "fetch"), "Fetch retries twice", ".agents/memory/notes/")
	})
	t.Run("is .memory otherwise", func(t *testing.T) {
		r := newRepo(t)
		r.fact("Fetch retries twice")
		if len(r.notes()) != 1 {
			t.Fatal("want the note in .memory/notes")
		}
	})
	t.Run("lives in the commit, so a clone uses the same directory", func(t *testing.T) {
		r := agentsRepo(t)
		id := r.fact("Fetch retries twice")
		r.commit("notes")
		c := &repo{t, t.TempDir()}
		r.git("clone", "-q", r.dir, c.dir)
		has(t, c.ok("show", id), "Fetch retries twice")
		has(t, c.ok("save", "fact", "--title", "Fetch uses a proxy", "--keywords", "fetch proxy upstream", "--fact", "f", "--evidence", "e"), " .agents/memory/notes/")
	})
	t.Run("lint reports each note left in .memory/notes when .agents/memory is the memory directory", func(t *testing.T) {
		r := newRepo(t)
		id := r.fact("Fetch retries twice")
		r.commit("notes")
		if err := os.MkdirAll(filepath.Join(r.dir, ".agents", "memory"), 0o755); err != nil {
			t.Fatal(err)
		}
		res := r.gm("lint")
		if res.code != 1 {
			t.Fatalf("want exit 1, got %s", res)
		}
		has(t, res.stdout, ".memory/notes/"+id+"-", ".agents/memory")
	})
	t.Run("after a move, show still finds a note pruned before the move", func(t *testing.T) {
		r := newRepo(t)
		old := r.fact("Fetch retries twice")
		r.fact("Fetch retries five times", "--supersedes", old)
		r.commit("notes")
		r.ok("prune")
		r.commit("prune")
		r.write(".agents/.keep", "")
		r.git("mv", ".memory", ".agents/memory")
		r.commit("move")
		has(t, r.ok("show", old), "Fetch retries twice", "pruned in")
		if res := r.gm("lint"); res.code != 0 {
			t.Fatalf("want lint clean after the move, got %s", res)
		}
	})
}
