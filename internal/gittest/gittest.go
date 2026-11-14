// Package gittest makes throwaway git repositories for tests. gm's behaviour
// depends on what git reports, so tests run against real git, not a fake.
package gittest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vabatta/gm/internal/git"
	"github.com/vabatta/gm/internal/memory"
	"github.com/vabatta/gm/internal/note"
)

// New makes a repository in a temporary directory with one committed file,
// fetch.go, and returns its Store. It turns commit signing off, because a
// global git config that signs commits would make test commits fail.
func New(t *testing.T) *memory.Store {
	t.Helper()
	dir := t.TempDir()
	st := &memory.Store{Git: &git.Repo{Root: dir}}
	for _, args := range [][]string{{"init", "-q", "-b", "main"}, {"config", "user.email", "t@t"}, {"config", "user.name", "t"}, {"config", "commit.gpgsign", "false"}} {
		Git(t, st, args...)
	}
	Write(t, st, "fetch.go", "package x\n\nfunc Fetch() {}\n")
	Commit(t, st, "code")
	return st
}

// Git runs git in the repository and fails the test on error.
func Git(t *testing.T, st *memory.Store, args ...string) string {
	t.Helper()
	out, err := st.Git.Run(args...)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// Commit stages every change and commits it.
func Commit(t *testing.T, st *memory.Store, msg string) {
	t.Helper()
	Git(t, st, "add", "-A")
	Git(t, st, "commit", "-qm", msg)
}

// Write writes a file relative to the repository root.
func Write(t *testing.T, st *memory.Store, name, s string) {
	t.Helper()
	path := filepath.Join(st.Git.Root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Fact saves a fact note that passes every check, anchored at each spec in at.
func Fact(st *memory.Store, title string, at ...string) (memory.Saved, error) {
	return st.Save(note.Note{Type: "fact", Title: title, Keywords: "fetch retry backoff",
		Body: note.Body("fact", map[string]string{"Fact": "f", "Evidence": "e"})}, at)
}
