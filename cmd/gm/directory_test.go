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

// inDir reports whether dir, relative to the root of r, holds exactly one
// .md file.
func inDir(r *repo, dir string) bool {
	files, _ := filepath.Glob(filepath.Join(r.dir, dir, "*.md"))
	return len(files) == 1
}

func TestDirectory(t *testing.T) {
	t.Run("is .agents/memory by default, which holds the note files directly", func(t *testing.T) {
		r := newRepo(t)
		has(t, r.saveFact("Fetch retries twice"), " .agents/memory/2")
		if !inDir(r, ".agents/memory") {
			t.Fatal("want the note directly in .agents/memory")
		}
	})
	t.Run("is the dir that .gmconfig names", func(t *testing.T) {
		for _, dir := range []string{"docs/memory", "notes dir", ".memory/notes"} {
			r := newRepo(t)
			r.setDir(dir)
			has(t, r.saveFact("Fetch retries twice"), " "+dir+"/2")
			if !inDir(r, dir) {
				t.Fatalf("want the note directly in %s", dir)
			}
			has(t, r.ok("search", "fetch"), "Fetch retries twice", dir+"/2")
		}
	})
	t.Run("a dir given as ./x/ names x", func(t *testing.T) {
		r := newRepo(t)
		r.setDir("./docs/memory/")
		has(t, r.saveFact("Fetch retries twice"), " docs/memory/2")
	})
	t.Run("committed .gmconfig makes a clone use the same directory", func(t *testing.T) {
		r := newRepo(t)
		r.setDir("docs/memory")
		id := r.fact("Fetch retries twice")
		r.commit("notes")
		c := &repo{t, t.TempDir()}
		r.git("clone", "-q", r.dir, c.dir)
		has(t, c.ok("show", id), "Fetch retries twice")
		has(t, c.saveFact("Fetch uses a proxy"), " docs/memory/2")
	})
	t.Run("gm.dir in the git configuration of a clone is not read", func(t *testing.T) {
		r := newRepo(t)
		r.git("config", "gm.dir", "docs/memory")
		has(t, r.saveFact("Fetch retries twice"), " .agents/memory/2")
	})
	t.Run("refuses to run when dir is empty, absolute or leads out of the worktree", func(t *testing.T) {
		for _, dir := range []string{"", ".", "/tmp/memory", "../memory", "a/../../memory"} {
			r := newRepo(t)
			r.setDir(dir)
			refused(t, r.gm("list"), ".gmconfig", "gm.dir")
		}
	})
	t.Run("after a move, show no longer finds a note pruned in the old directory", func(t *testing.T) {
		r := newRepo(t)
		old := r.fact("Fetch retries twice")
		r.fact("Fetch retries five times", "--supersedes", old)
		r.commit("notes")
		r.ok("prune")
		r.commit("prune")
		r.setDir("docs/memory")
		r.write("docs/.keep", "")
		r.git("mv", ".agents/memory", "docs/memory")
		r.commit("move")
		refused(t, r.gm("show", old), "no note "+old)
	})
}
