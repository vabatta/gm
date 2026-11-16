package memory

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/vabatta/gm/internal/git"
)

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
	if !ok || linked(r.Root, p) {
		return nil, fmt.Errorf("gm.dir in .gmconfig must name a directory inside the worktree, relative to its root, with no symbolic link, no .git component and no quote, backslash or control character, not %q", d)
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

// linked reports whether some part of the notes directory p under root is a
// symbolic link. git tracks the files under a link by the target's path, not
// by p, so history reads would never match them, and a link may lead out of
// the worktree or to its root. The parts that do not exist yet are not links.
func linked(root, p string) bool {
	at := root
	for part := range strings.SplitSeq(p, "/") {
		at = filepath.Join(at, part)
		info, err := os.Lstat(at)
		if err != nil {
			return false
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return true
		}
	}
	return false
}

// NotesDir returns the directory that holds one markdown file per note.
func (s *Store) NotesDir() string { return filepath.Join(s.Git.Root, filepath.FromSlash(s.notesPath)) }
