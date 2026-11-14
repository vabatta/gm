package memory

import (
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/vabatta/gm/internal/note"
)

// notesPath is NotesDir relative to the repository root, the form git prints.
const notesPath = ".memory/notes"

// Prune deletes the file of every note that another note supersedes and
// returns the paths it deleted. A superseded note whose file is not committed
// as it is at HEAD has no commit that would keep it, so Prune keeps it and
// returns its path in kept. Live notes, retirements included, stay.
func (s *Store) Prune() (pruned, kept []string, err error) {
	ns, _ := s.Load()
	dead := Dead(ns)
	status, err := s.Git.Run("status", "--porcelain", "--untracked-files=all", "--", notesPath)
	if err != nil {
		return nil, nil, err
	}
	dirty := map[string]bool{}
	for line := range strings.SplitSeq(status, "\n") {
		if len(line) > 3 {
			dirty[line[3:]] = true
		}
	}
	for _, n := range ns {
		switch {
		case !dead[n.ID]:
		case dirty[n.File]:
			kept = append(kept, n.File)
		default:
			if err := os.Remove(filepath.Join(s.Git.Root, filepath.FromSlash(n.File))); err != nil {
				return pruned, kept, err
			}
			pruned = append(pruned, n.File)
		}
	}
	return pruned, kept, nil
}

// prunedFile is a note file that a commit in the history of HEAD deleted.
type prunedFile struct{ commit, path string }

// prunedFiles lists the note files deleted in the history of HEAD, newest
// deletion first, with one git log call. A shallow clone lists only the
// deletions in the history it has.
func (s *Store) prunedFiles() []prunedFile {
	out, err := s.Git.Run("log", "--diff-filter=D", "--name-only", "--format=commit %H", "--", notesPath)
	if err != nil {
		return nil
	}
	var files []prunedFile
	commit := ""
	for line := range strings.SplitSeq(out, "\n") {
		if c, ok := strings.CutPrefix(line, "commit "); ok {
			commit = c
		} else if strings.HasSuffix(line, ".md") && path.Dir(line) == notesPath {
			files = append(files, prunedFile{commit, line})
		}
	}
	return files
}

// prunedIDs returns the ids of the notes deleted in the history of HEAD,
// read from the start of their file names without reading the notes.
func (s *Store) prunedIDs() map[string]bool {
	ids := map[string]bool{}
	for _, f := range s.prunedFiles() {
		if base := path.Base(f.path); len(base) > idLen {
			ids[base[:idLen]] = true
		}
	}
	return ids
}

// idLen is the length of a note id: 20060102T150405Z-abcdef.
const idLen = 23

// Pruned reads the notes deleted in the history of HEAD back from git, each
// from the parent of the commit that deleted it, with one cat-file --batch
// call. Each note has PrunedIn set to that commit, cut to 12 hex characters.
func (s *Store) Pruned() []note.Note {
	files := s.prunedFiles()
	if len(files) == 0 {
		return nil
	}
	var stdin strings.Builder
	for _, f := range files {
		stdin.WriteString(f.commit + "^:" + f.path + "\n")
	}
	out, err := s.Git.Raw(stdin.String(), "cat-file", "--batch")
	if err != nil {
		return nil
	}
	var ns []note.Note
	seen := map[string]bool{}
	for _, f := range files {
		head, rest, _ := strings.Cut(out, "\n")
		fields := strings.Fields(head)
		if len(fields) != 3 {
			out = rest // "<object> missing": the parent commit is not in this clone
			continue
		}
		size, _ := strconv.Atoi(fields[2])
		size = min(size, len(rest))
		content := rest[:size]
		out = strings.TrimPrefix(rest[size:], "\n")
		n, err := note.Parse([]byte(content), f.path)
		if err != nil || seen[n.ID] {
			continue
		}
		seen[n.ID] = true
		n.PrunedIn = f.commit[:12]
		ns = append(ns, n)
	}
	return ns
}

// All returns the notes on disk followed by the pruned notes that git history
// still holds.
func (s *Store) All() []note.Note {
	ns, _ := s.Load()
	onDisk := map[string]bool{}
	for _, n := range ns {
		onDisk[n.ID] = true
	}
	for _, n := range s.Pruned() {
		if !onDisk[n.ID] {
			ns = append(ns, n)
		}
	}
	return ns
}
