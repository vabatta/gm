package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// move sets gm.dir to to, moves the notes from from with git mv, and commits.
func move(r *repo, from, to string) {
	r.t.Helper()
	r.setDir(to)
	r.write(filepath.Join(filepath.Dir(to), ".keep"), "")
	r.git("mv", from, to)
	r.commit("move notes to " + to)
}

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
	t.Run("refuses to run when dir is empty, absolute, leads out, has a .git component or holds a quote, backslash or control character, quoting dir as written", func(t *testing.T) {
		for _, dir := range []string{"", ".", "/tmp/memory", "../memory", "a/../../memory", ".git/memory", "a/.GIT/memory", `a"b`, `a\b`, "a\tb"} {
			r := newRepo(t)
			r.setDir(dir)
			refused(t, r.gm("list"), ".gmconfig", "gm.dir", fmt.Sprintf("%q", dir))
		}
	})
	t.Run("refuses to run when dir leads out of the worktree through a symbolic link", func(t *testing.T) {
		r := newRepo(t)
		if err := os.Symlink(t.TempDir(), filepath.Join(r.dir, "out")); err != nil {
			t.Fatal(err)
		}
		r.setDir("out/memory")
		refused(t, r.gm("list"), ".gmconfig", "gm.dir", `"out/memory"`)
	})
	t.Run("an unknown command is reported before a bad dir", func(t *testing.T) {
		r := newRepo(t)
		r.setDir("../memory")
		refused(t, r.gm("frobnicate"), "unknown command")
	})
	t.Run("earlier releases' .memory/notes is not read", func(t *testing.T) {
		r := newRepo(t)
		r.setDir(".memory/notes")
		r.fact("Fetch retries twice")
		r.git("config", "--file", ".gmconfig", "--unset", "gm.dir")
		if out := r.ok("list"); strings.Contains(out, "Fetch retries twice") {
			t.Fatalf("want .memory/notes unread, got\n%s", out)
		}
	})
	t.Run("after a move, history reads cover every directory .gmconfig named", func(t *testing.T) {
		r := newRepo(t)
		old := r.fact("Fetch retries twice")
		revision := r.fact("Fetch retries five times", "--supersedes", old)
		r.commit("notes")
		r.ok("prune")
		r.commit("prune")
		move(r, ".agents/memory", "docs/memory")
		move(r, "docs/memory", "kept/notes")
		has(t, r.ok("show", old), "Fetch retries twice", "pruned in")
		has(t, r.ok("search", "--all", "fetch"), old)
		if res := r.gm("lint"); res.code != 0 {
			t.Fatalf("want lint to accept the supersedes entry of %s, got %s", revision, res)
		}
	})
	t.Run("a branch that only moves notes does not count them as new", func(t *testing.T) {
		r := newRepo(t)
		setLine3(r, "fetch.go", "func Fetch() { plain() }")
		r.commit("code")
		r.fact("Fetch is plain", "--at", "fetch.go:3")
		r.commit("note")
		setLine3(r, "fetch.go", "func Fetch() { retry() }")
		r.commit("change")
		r.git("checkout", "-qb", "move")
		move(r, ".agents/memory", "docs/memory")
		if res := r.gm("lint"); res.code != 0 {
			t.Fatalf("want the moved note to count as merged, got %s", res)
		}
	})
}
