package memory

import (
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/vabatta/gm/internal/note"
)

// Prune deletes the file of every note that another note supersedes and
// returns the paths it deleted. A superseded note whose file is not committed
// as it is at HEAD has no commit that would keep it, so Prune keeps it and
// returns its path in kept. git.Repo.AtHEAD decides that by content, so an
// ignored file, a change hidden from git status, and a note inside a
// submodule all count as not committed. Prune leaves live notes, retirements
// included, in place.
func (s *Store) Prune() (pruned, kept []string, err error) {
	ns, _ := s.Load()
	dead := Dead(ns)
	var files []string
	for _, n := range ns {
		if dead[n.ID] {
			files = append(files, n.File)
		}
	}
	at := s.Git.AtHEAD(files)
	for _, n := range ns {
		switch {
		case !dead[n.ID]:
		case !at[n.File]:
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

// prunedFile is a note file that a commit in the history of HEAD deleted. It
// holds the commit, the file's path, and the id of the blob the file held.
type prunedFile struct{ commit, path, blob string }

// prunedFiles lists the note files deleted in the history of HEAD, newest
// deletion first, with one git log --raw call. The raw diff names the blob
// of each deleted file, so Pruned can read the note by blob id without walking
// the tree of the commit before. --no-renames keeps git from pairing a deleted
// note with a similar note added in the same commit, which would hide the
// deletion. --diff-merges=separate shows a merge commit's diff against each
// parent, so a note deleted in a merge counts too; Pruned keeps one copy. In a shallow clone it lists only the deletions in the history the
// clone has.
func (s *Store) prunedFiles() []prunedFile {
	out, err := s.Git.Run(append([]string{"log", "--no-renames", "--diff-merges=separate", "--diff-filter=D", "--raw", "--no-abbrev", "--format=commit %H", "--"}, s.historyPaths()...)...)
	if err != nil {
		return nil
	}
	var files []prunedFile
	commit := ""
	for line := range strings.SplitSeq(out, "\n") {
		if c, ok := strings.CutPrefix(line, "commit "); ok {
			commit = c
			continue
		}
		// :100644 000000 <old blob> <new blob> D<TAB><path>
		meta, file, ok := strings.Cut(line, "\t")
		if f := strings.Fields(meta); ok && len(f) == 5 && strings.HasSuffix(file, ".md") && slices.Contains(s.historyPaths(), path.Dir(file)) {
			files = append(files, prunedFile{commit, file, f[2]})
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

// Pruned reads the notes deleted in the history of HEAD back from git, by the
// blob each deleted file held, with one cat-file --batch call. It sets each
// note's PrunedIn to the commit that deleted it, cut to 12 hex characters.
func (s *Store) Pruned() []note.Note {
	files := s.prunedFiles()
	if len(files) == 0 {
		return nil
	}
	var stdin strings.Builder
	for _, f := range files {
		stdin.WriteString(f.blob + "\n")
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
			out = rest // "<blob> missing": a partial clone without the blob
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
