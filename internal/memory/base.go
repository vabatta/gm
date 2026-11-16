package memory

import (
	"path"
	"strings"

	"github.com/vabatta/gm/internal/anchor"
	"github.com/vabatta/gm/internal/note"
)

// added maps each note file name to the newest commit in the history of HEAD
// that added it, with one git log call over historyPaths. A note that is not
// committed has no entry. The key is the file name, which starts with the note
// id, so a note moved to another notes directory keeps its entry, and a commit
// that adds a name and deletes it elsewhere only moved the note, so it does
// not count. --no-renames keeps git from reporting a note added next to a
// similar deleted note as a rename, which would hide the addition.
func (s *Store) added() map[string]string {
	out, _ := s.log("--diff-filter=AD", "--name-status")
	m := map[string]string{}
	commit := ""
	adds, dels := map[string]bool{}, map[string]bool{}
	flush := func() {
		for name := range adds {
			if !dels[name] && m[name] == "" {
				m[name] = commit
			}
		}
		clear(adds)
		clear(dels)
	}
	for line := range strings.SplitSeq(out, "\n") {
		if c, ok := strings.CutPrefix(line, "commit "); ok {
			flush()
			commit = c
		} else if status, file, ok := strings.Cut(line, "\t"); ok && s.noteFile(file) {
			if status == "A" {
				adds[path.Base(file)] = true
			} else {
				dels[path.Base(file)] = true
			}
		}
	}
	flush()
	return m
}

// Anchors resolves the anchors of every note in ns with anchor.Resolve, keyed
// by note file. The commit that added a note is the fallback base of its
// anchors. Anchors looks it up only when some anchor's own commit is not in
// the history of HEAD, because that takes a walk of the history.
func (s *Store) Anchors(ns []note.Note) map[string][]anchor.Resolved {
	res := s.resolve(ns, nil)
	for _, rs := range res {
		for _, r := range rs {
			if r.Fallback {
				return s.resolve(ns, s.added())
			}
		}
	}
	return res
}

// resolve runs anchor.Resolve once for the anchors of every note in ns, with
// added giving the commit that added each note file, and groups the results
// by note file.
func (s *Store) resolve(ns []note.Note, added map[string]string) map[string][]anchor.Resolved {
	var targets []anchor.Target
	for _, n := range ns {
		for _, spec := range n.At {
			targets = append(targets, anchor.Target{Spec: spec, Added: added[path.Base(n.File)]})
		}
	}
	res := anchor.Resolve(s.Git, targets)
	m := map[string][]anchor.Resolved{}
	i := 0
	for _, n := range ns {
		m[n.File] = res[i : i+len(n.At)]
		i += len(n.At)
	}
	return m
}

// defaultBranch returns the branch that work merges into: the first that
// exists of origin/HEAD, origin/main, origin/master, main and master. A remote
// branch comes first, because a local branch may be behind it and miss notes
// that are already merged. It is empty when none exists.
func (s *Store) defaultBranch() string {
	if ref, err := s.Git.Run("symbolic-ref", "-q", "--short", "refs/remotes/origin/HEAD"); err == nil && ref != "" {
		return ref
	}
	for _, ref := range []string{"refs/remotes/origin/main", "refs/remotes/origin/master", "refs/heads/main", "refs/heads/master"} {
		if _, err := s.Git.Run("rev-parse", "--verify", "-q", ref); err == nil {
			return strings.TrimPrefix(strings.TrimPrefix(ref, "refs/heads/"), "refs/remotes/")
		}
	}
	return ""
}

// onDefault returns the names of the note files that the default branch
// holds directly in any of historyPaths, with one git ls-tree call, or nil when there
// is no default branch. A note missing from it is work on a branch that is not
// merged yet. Names start with the note id, so a branch that only moves notes
// to another notes directory does not count them as new.
func (s *Store) onDefault() map[string]bool {
	def := s.defaultBranch()
	if def == "" {
		return nil
	}
	out, _ := s.Git.Run(append([]string{"-c", "core.quotePath=false", "ls-tree", "-r", "--name-only", def, "--"}, s.historyPaths()...)...)
	m := map[string]bool{}
	for name := range strings.SplitSeq(out, "\n") {
		if s.noteFile(name) {
			m[path.Base(name)] = true
		}
	}
	return m
}

// shallow reports whether the repository is a shallow clone, which lacks the
// older history that holds pruned notes and old anchor commits.
func (s *Store) shallow() bool {
	out, _ := s.Git.Run("rev-parse", "--is-shallow-repository")
	return out == "true"
}

// fetchHint is what gm adds to an error about an id it cannot find in a
// shallow clone, where the id may name a note pruned in the missing history.
const fetchHint = "; this shallow clone lacks the history that may hold it, so fetch the full history"
