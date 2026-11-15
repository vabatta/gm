// Package memory reads and writes the notes of one repository, one file per
// note in <root>/<dir>, where dir is gm.dir in <root>/.gmconfig or
// .agents/memory.
//
// gm never edits a note file. A revision is a new note that names the old one
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
type Store struct {
	Git *git.Repo
	// notesPath is NotesDir relative to the worktree root, in the slash form
	// git prints.
	notesPath string
}

// defaultNotes is the notes directory when .gmconfig names none. legacyNotes
// is where earlier releases kept notes.
const (
	defaultNotes = ".agents/memory"
	legacyNotes  = ".memory/notes"
)

// Open returns the Store of the worktree that contains dir, or git.ErrNoRepo.
// The notes directory is gm.dir in the committed file .gmconfig, read with git
// config so its syntax is git's. gm.dir in the git configuration of a clone
// does not count, because each clone has its own and the notes must be in one
// place for all of them. Without gm.dir it is .agents/memory, or legacyNotes
// when only that exists, so a repository from an earlier release keeps
// working.
func Open(dir string) (*Store, error) {
	r, err := git.Open(dir)
	if err != nil {
		return nil, err
	}
	d, err := r.Run("config", "--file", ".gmconfig", "--default", "", "gm.dir")
	switch {
	case err != nil:
		return nil, err
	case d == "" && !isDir(r.Root, defaultNotes) && isDir(r.Root, legacyNotes):
		return &Store{Git: r, notesPath: legacyNotes}, nil
	case d == "" && !hasKey(r):
		d = defaultNotes
	}
	d = filepath.Clean(d)
	if !filepath.IsLocal(d) || d == "." {
		return nil, fmt.Errorf("gm.dir in .gmconfig must be a directory inside the worktree, relative to its root, not %q", d)
	}
	return &Store{Git: r, notesPath: filepath.ToSlash(d)}, nil
}

// hasKey reports whether .gmconfig sets gm.dir at all, so an empty value is
// refused rather than taken for the default.
func hasKey(r *git.Repo) bool {
	_, err := r.Run("config", "--file", ".gmconfig", "gm.dir")
	return err == nil
}

// isDir reports whether rel, relative to root, is a directory.
func isDir(root, rel string) bool {
	fi, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel)))
	return err == nil && fi.IsDir()
}

// historyPaths are the notes directories whose notes git history may hold:
// legacyNotes and the current one. A move of the notes from legacyNotes thus
// keeps notes pruned before the move readable.
func (s *Store) historyPaths() []string {
	if s.notesPath == legacyNotes {
		return []string{legacyNotes}
	}
	return []string{legacyNotes, s.notesPath}
}

// NotesDir returns the directory that holds one markdown file per note.
func (s *Store) NotesDir() string { return filepath.Join(s.Git.Root, filepath.FromSlash(s.notesPath)) }

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

// Live returns the notes that hold now, which are those that no note supersedes.
// Retirements are live, because their reason often says what replaced the
// notes they retire.
func Live(ns []note.Note) []note.Note {
	dead := Dead(ns)
	var out []note.Note
	for _, n := range ns {
		if !dead[n.ID] {
			out = append(out, n)
		}
	}
	return out
}

// Get returns the note with the given id, from disk or, when it was pruned,
// from git history.
func (s *Store) Get(id string) (note.Note, error) {
	ns, _ := s.Load()
	for _, n := range ns {
		if n.ID == id {
			return n, nil
		}
	}
	for _, n := range s.Pruned() {
		if n.ID == id {
			return n, nil
		}
	}
	return note.Note{}, fmt.Errorf("no note %s", id)
}

// Write writes n to a new file in NotesDir without checking it and returns n
// with File set. It creates the file with O_EXCL, so it never overwrites a
// note. importer.Import calls it directly for migrated notes.
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
