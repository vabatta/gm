package memory

import (
	"path"
	"slices"
	"strings"

	"github.com/vabatta/gm/internal/note"
)

// legacyNotes is where earlier releases kept notes. gm no longer reads notes
// on disk there, but history reads cover it, so a repository that moved its
// notes from there keeps its pruned notes and anchor bases.
const legacyNotes = ".memory/notes"

// historyPaths returns the notes directories whose notes git history may
// hold: .agents/memory, .memory/notes, the current one, and every gm.dir that
// .gmconfig set in the history of HEAD. So after a move, history reads still
// find the notes of the old directory. It lists the versions of .gmconfig
// with one git log call and runs git config once for each distinct version,
// once per Store.
func (s *Store) historyPaths() []string {
	if s.history != nil {
		return s.history
	}
	s.history = []string{defaultNotes, legacyNotes}
	add := func(p string) {
		if !slices.Contains(s.history, p) {
			s.history = append(s.history, p)
		}
	}
	add(s.notesPath)
	out, _ := s.Git.Run("log", "--format=", "--raw", "--no-abbrev", "--no-renames", "--", ".gmconfig")
	seen := map[string]bool{}
	for line := range strings.SplitSeq(out, "\n") {
		// :100644 100644 <old blob> <new blob> M<TAB>.gmconfig
		f := strings.Fields(line)
		if len(f) < 4 || strings.Trim(f[3], "0") == "" || seen[f[3]] {
			continue
		}
		seen[f[3]] = true
		if d, err := s.Git.Run("config", "--blob", f[3], "gm.dir"); err == nil {
			if p, ok := notesPath(d); ok {
				add(p)
			}
		}
	}
	return s.history
}

// noteFile reports whether file, a path that git prints, is a note file of
// git history: named as note.FileName names one, directly in one of
// historyPaths. Other markdown files there, such as those of other tools in
// .agents/memory or those in a subdirectory, are not notes.
func (s *Store) noteFile(file string) bool {
	return slices.Contains(s.historyPaths(), path.Dir(file)) && note.IsFileName(path.Base(file))
}

// log runs git log over historyPaths with --no-renames, one "commit <hash>"
// line per commit, and paths printed unquoted, so a notes directory with
// characters beyond ASCII matches the paths git prints.
func (s *Store) log(args ...string) (string, error) {
	args = append([]string{"-c", "core.quotePath=false", "log", "--no-renames", "--format=commit %H"}, args...)
	return s.Git.Run(append(append(args, "--"), s.historyPaths()...)...)
}
