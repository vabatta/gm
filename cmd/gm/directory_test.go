package main

import (
	"path/filepath"
	"testing"
)

// setDir writes .gmconfig at the root of r with gm.dir set to dir.
func (r *repo) setDir(dir string) {
	r.t.Helper()
	r.git("config", "--file", ".gmconfig", "gm.dir", dir)
}

// saveFact saves a sound fact and returns what gm save printed.
func (r *repo) saveFact(title string) string {
	r.t.Helper()
	return r.ok("save", "fact", "--title", title, "--keywords", "fetch retry backoff", "--fact", "f", "--evidence", "e")
}

func TestDirectory(t *testing.T) {
	t.Run("is .memory by default", func(t *testing.T) {
		r := newRepo(t)
		has(t, r.saveFact("Fetch retries twice"), " .memory/notes/")
	})
	t.Run("is the dir that .gmconfig names", func(t *testing.T) {
		for _, dir := range []string{".agents/memory", "docs/memory", "notes dir"} {
			r := newRepo(t)
			r.setDir(dir)
			has(t, r.saveFact("Fetch retries twice"), " "+dir+"/notes/")
			if files, _ := filepath.Glob(filepath.Join(r.dir, dir, "notes", "*.md")); len(files) != 1 {
				t.Fatalf("want one note in %s/notes, got %v", dir, files)
			}
			has(t, r.ok("search", "fetch"), "Fetch retries twice", dir+"/notes/")
		}
	})
	t.Run("a dir given as ./x/ names x", func(t *testing.T) {
		r := newRepo(t)
		r.setDir("./.agents/memory/")
		has(t, r.saveFact("Fetch retries twice"), " .agents/memory/notes/")
	})
	t.Run("committed .gmconfig makes a clone use the same directory", func(t *testing.T) {
		r := newRepo(t)
		r.setDir(".agents/memory")
		id := r.fact("Fetch retries twice")
		r.commit("notes")
		c := &repo{t, t.TempDir()}
		r.git("clone", "-q", r.dir, c.dir)
		has(t, c.ok("show", id), "Fetch retries twice")
		has(t, c.saveFact("Fetch uses a proxy"), " .agents/memory/notes/")
	})
	t.Run("gm.dir in the git configuration of a clone is not read", func(t *testing.T) {
		r := newRepo(t)
		r.git("config", "gm.dir", ".agents/memory")
		has(t, r.saveFact("Fetch retries twice"), " .memory/notes/")
	})
	t.Run("refuses to run when dir is empty, absolute or leads out of the worktree", func(t *testing.T) {
		for _, dir := range []string{"", ".", "/tmp/memory", "../memory", "a/../../memory"} {
			r := newRepo(t)
			r.setDir(dir)
			refused(t, r.gm("list"), ".gmconfig", "gm.dir")
		}
	})
	t.Run("lint reports each note left in .memory/notes when dir names another directory", func(t *testing.T) {
		r := newRepo(t)
		id := r.fact("Fetch retries twice")
		r.commit("notes")
		r.setDir(".agents/memory")
		res := r.gm("lint")
		if res.code != 1 {
			t.Fatalf("want exit 1, got %s", res)
		}
		has(t, res.stdout, ".memory/notes/"+id+"-", ".agents/memory/notes")
	})
	t.Run("after a move from .memory, show still finds a note pruned before the move", func(t *testing.T) {
		r := newRepo(t)
		old := r.fact("Fetch retries twice")
		r.fact("Fetch retries five times", "--supersedes", old)
		r.commit("notes")
		r.ok("prune")
		r.commit("prune")
		moveTo(r, ".agents/memory")
		has(t, r.ok("show", old), "Fetch retries twice", "pruned in")
		if res := r.gm("lint"); res.code != 0 {
			t.Fatalf("want lint clean after the move, got %s", res)
		}
	})
}
