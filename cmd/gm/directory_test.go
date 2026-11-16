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
	t.Run("refuses to run when dir passes through a symbolic link", func(t *testing.T) {
		for _, c := range []struct{ link, target, dir string }{
			{"out", "", "out/memory"},
			{"docs", "real", "docs/memory"},
			{".agents/memory", "..", ".agents/memory"},
		} {
			r := newRepo(t)
			target := c.target
			if target == "" {
				target = t.TempDir()
			} else if target != ".." {
				r.write(target+"/.keep", "")
			}
			os.MkdirAll(filepath.Dir(filepath.Join(r.dir, c.link)), 0o755)
			if err := os.Symlink(target, filepath.Join(r.dir, c.link)); err != nil {
				t.Fatal(err)
			}
			if c.dir != ".agents/memory" {
				r.setDir(c.dir)
			}
			refused(t, r.gm("list"), ".gmconfig", "gm.dir", fmt.Sprintf("%q", c.dir))
		}
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
	t.Run("after the upgrade from .memory/notes, history reads still cover it", func(t *testing.T) {
		r := newRepo(t)
		r.setDir(".memory/notes")
		old := r.fact("Fetch retries twice")
		r.fact("Fetch retries five times", "--supersedes", old)
		os.Remove(filepath.Join(r.dir, ".gmconfig"))
		r.commit("notes from an earlier release")
		r.git("rm", "-q", ".memory/notes/"+old+"-fetch-retries-twice.md")
		r.commit("prune")
		r.write(".agents/.keep", "")
		r.git("mv", ".memory/notes", ".agents/memory")
		r.commit("upgrade")
		has(t, r.ok("show", old), "Fetch retries twice", "pruned in")
		if res := r.gm("lint"); res.code != 0 {
			t.Fatalf("want lint to accept the supersedes entry of %s, got %s", old, res)
		}
	})
	t.Run("history reads skip files not named as notes", func(t *testing.T) {
		r := newRepo(t)
		r.setDir("docs/memory")
		r.write(".agents/memory/readme-for-other-agent-tools.md", "other tool\n")
		r.commit("other tool")
		r.git("rm", "-q", ".agents/memory/readme-for-other-agent-tools.md")
		r.commit("other tool cleans up")
		refused(t, r.gm("save", "fact", "--title", "t", "--keywords", "a b c", "--fact", "f", "--evidence", "e", "--supersedes", "readme-for-other-agent-"), "readme-for-other-agent-")
	})
	t.Run("a note in a subdirectory on the default branch does not count as merged", func(t *testing.T) {
		r := newRepo(t)
		setLine3(r, "fetch.go", "func Fetch() { plain() }")
		r.commit("code")
		r.fact("Fetch is plain", "--at", "fetch.go:3")
		name := strings.TrimPrefix(r.notes()[0], r.dir+"/")
		archived := ".agents/memory/archive/" + filepath.Base(name)
		r.write(archived, r.read(name))
		os.Remove(filepath.Join(r.dir, name))
		r.commit("archived copy only")
		r.git("checkout", "-qb", "work")
		r.write(name, r.read(archived))
		r.commit("note")
		setLine3(r, "fetch.go", "func Fetch() { retry() }")
		r.commit("change")
		has(t, lintFails(t, r), "the anchored code changed after the note")
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
