// Package memory reads and writes the notes of one repository, one file per
// note in <root>/.memory/notes.
//
// A note file is never edited. A revision is a new note that names the old one
// in its supersedes list. A retirement is a note that supersedes notes with no
// successor. Each save adds a new file, so branches that save in parallel merge
// without conflicts. When two branches revise one topic, both notes stay live
// after the merge, and Lint fails until one note supersedes both. A merge
// therefore never drops a note silently.
package memory

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/vabatta/gm/internal/git"
	"github.com/vabatta/gm/internal/note"
)

// Store is the memory of one git worktree. It reads the notes of that
// worktree's checkout, so the notes follow the branch.
type Store struct{ Git *git.Repo }

// Open returns the Store of the worktree that contains dir, or git.ErrNoRepo.
func Open(dir string) (*Store, error) {
	r, err := git.Open(dir)
	if err != nil {
		return nil, err
	}
	return &Store{Git: r}, nil
}

// Dir returns the .memory directory at the worktree root.
func (s *Store) Dir() string { return filepath.Join(s.Git.Root, ".memory") }

// NotesDir returns the directory that holds one markdown file per note.
func (s *Store) NotesDir() string { return filepath.Join(s.Dir(), "notes") }

// Files returns the paths of the .md files in NotesDir, sorted by name. A
// name starts with the note id, so the order is by creation time. A missing
// directory gives no files.
func (s *Store) Files() []string {
	entries, _ := os.ReadDir(s.NotesDir())
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			files = append(files, filepath.Join(s.NotesDir(), e.Name()))
		}
	}
	return files
}

// Load reads and parses every note file. bad lists each file that does not
// parse, with the reason, so Lint reports the file instead of skipping it.
func (s *Store) Load() (ns []note.Note, bad []string) {
	for _, f := range s.Files() {
		rel, _ := filepath.Rel(s.Git.Root, f)
		b, err := os.ReadFile(f)
		var n note.Note
		if err == nil {
			n, err = note.Parse(b, filepath.ToSlash(rel))
		}
		if err != nil {
			bad = append(bad, filepath.ToSlash(rel)+": "+err.Error())
			continue
		}
		ns = append(ns, n)
	}
	return ns, bad
}

// Dead returns the set of ids that some note supersedes.
func Dead(ns []note.Note) map[string]bool {
	dead := map[string]bool{}
	for _, n := range ns {
		for _, s := range n.Supersedes {
			dead[s] = true
		}
	}
	return dead
}

// Live returns the notes that hold now: those that no note supersedes, minus
// retirements.
func Live(ns []note.Note) []note.Note {
	dead := Dead(ns)
	var out []note.Note
	for _, n := range ns {
		if !dead[n.ID] && n.Type != "retirement" {
			out = append(out, n)
		}
	}
	return out
}

// Get returns the note with the given id.
func (s *Store) Get(id string) (note.Note, error) {
	ns, _ := s.Load()
	for _, n := range ns {
		if n.ID == id {
			return n, nil
		}
	}
	return note.Note{}, fmt.Errorf("no note %s", id)
}

// Write writes n to a new file in NotesDir without checking it and returns n
// with File set. It creates the file with O_EXCL, so it never overwrites a
// note. engram.Import calls it directly for migrated notes.
func (s *Store) Write(n note.Note) (note.Note, error) {
	if err := os.MkdirAll(s.NotesDir(), 0o755); err != nil {
		return n, err
	}
	path := filepath.Join(s.NotesDir(), n.FileName())
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return n, err
	}
	_, err = f.WriteString(n.Render())
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	rel, _ := filepath.Rel(s.Git.Root, path)
	n.File = filepath.ToSlash(rel)
	return n, err
}
