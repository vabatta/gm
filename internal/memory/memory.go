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
	"slices"
	"strings"
	"unicode"

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
	// history caches historyPaths.
	history []string
}

// defaultNotes is the notes directory when .gmconfig names none.
const defaultNotes = ".agents/memory"

// Open returns the Store of the worktree that contains dir, or git.ErrNoRepo.
// The notes directory is gm.dir in the committed file .gmconfig, read with git
// config so its syntax is git's, and .agents/memory when that is not set.
// gm.dir in the git configuration of a clone does not count, because each
// clone has its own and the notes must be in one place for all of them. Open
// runs git config only when .gmconfig exists.
func Open(dir string) (*Store, error) {
	r, err := git.Open(dir)
	if err != nil {
		return nil, err
	}
	d := defaultNotes
	if _, err := os.Lstat(filepath.Join(r.Root, ".gmconfig")); err == nil {
		if d, err = r.Run("config", "--file", ".gmconfig", "--default", defaultNotes, "gm.dir"); err != nil {
			return nil, err
		}
	}
	p, ok := notesPath(d)
	if !ok || escapes(r.Root, p) {
		return nil, fmt.Errorf("gm.dir in .gmconfig must name a directory inside the worktree, relative to its root, with no .git component and no quote, backslash or control character, not %q", d)
	}
	return &Store{Git: r, notesPath: p}, nil
}

// notesPath cleans the gm.dir value d to the slash form git prints and reports
// whether it is a usable notes directory: local, not the root, with no .git
// component, which git never commits, and with no character that git quotes
// in the paths it prints.
func notesPath(d string) (string, bool) {
	if strings.ContainsFunc(d, func(c rune) bool { return c == '"' || c == '\\' || unicode.IsControl(c) }) {
		return "", false
	}
	c := filepath.Clean(d)
	if !filepath.IsLocal(c) || c == "." {
		return "", false
	}
	for part := range strings.SplitSeq(filepath.ToSlash(c), "/") {
		if strings.EqualFold(part, ".git") {
			return "", false
		}
	}
	return filepath.ToSlash(c), true
}

// escapes reports whether the notes directory p leads out of the worktree at
// root through a symbolic link, judged by the deepest part of it that exists.
func escapes(root, p string) bool {
	at := filepath.Join(root, filepath.FromSlash(p))
	for {
		if _, err := os.Lstat(at); err == nil || at == root {
			break
		}
		at = filepath.Dir(at)
	}
	realRoot, err1 := filepath.EvalSymlinks(root)
	real, err2 := filepath.EvalSymlinks(at)
	if err1 != nil || err2 != nil {
		return true
	}
	rel, err := filepath.Rel(realRoot, real)
	return err != nil || !filepath.IsLocal(rel) && rel != "."
}

// historyPaths returns the notes directories whose notes git history may
// hold: .agents/memory, the current one, and every gm.dir that .gmconfig set
// in the history of HEAD. So after a move, history reads still find the notes
// of the old directory. It reads the history of .gmconfig once per Store.
func (s *Store) historyPaths() []string {
	if s.history != nil {
		return s.history
	}
	s.history = []string{defaultNotes}
	add := func(p string) {
		if !slices.Contains(s.history, p) {
			s.history = append(s.history, p)
		}
	}
	add(s.notesPath)
	commits, _ := s.Git.Run("log", "--format=%H", "--", ".gmconfig")
	for c := range strings.SplitSeq(commits, "\n") {
		if c == "" {
			continue
		}
		if d, err := s.Git.Run("config", "--blob", c+":.gmconfig", "gm.dir"); err == nil {
			if p, ok := notesPath(d); ok {
				add(p)
			}
		}
	}
	return s.history
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
