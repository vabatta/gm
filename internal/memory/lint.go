package memory

import (
	"fmt"
	"maps"
	"path"
	"slices"
	"strings"

	"github.com/vabatta/gm/internal/anchor"
	"github.com/vabatta/gm/internal/note"
)

// Report is the result of Lint. Problems make gm lint exit 1. Review lists the
// anchors of live notes whose code changed after their base. Warnings lists
// what Lint could not check. Neither Review nor Warnings makes gm lint fail.
type Report struct {
	Problems []string `json:"problems"`
	Review   []string `json:"review,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

// Lint checks every note and reports these problems:
//   - a file that does not parse
//   - an id that more than one file uses
//   - a supersedes entry that names no note on disk and no note pruned in the
//     history of HEAD
//   - a live note that breaks a rule in note.Problems
//   - an anchor whose file anchor.Resolve finds gone
//   - a line-range anchor of a note not yet on the default branch whose lines
//     changed after its base
//   - a topic with more than one live note
//
// A note on a branch is about to merge, and a squash merge would hide that
// its lines changed. So Lint reports that change as a problem rather than a
// review line. Superseded notes are history, so of the per-note checks Lint
// runs only the supersedes check on them. With review, Lint also lists every
// other anchor of a live note whose code changed after its base.
func (s *Store) Lint(review bool) Report {
	ns, bad := s.Load()
	rep := Report{Problems: bad}
	report := func(format string, a ...any) { rep.Problems = append(rep.Problems, fmt.Sprintf(format, a...)) }
	files := map[string][]string{}
	for _, n := range ns {
		files[n.ID] = append(files[n.ID], n.File)
	}
	for _, id := range slices.Sorted(maps.Keys(files)) {
		if len(files[id]) > 1 {
			report("id %s is used by %d notes: %s", id, len(files[id]), strings.Join(files[id], ", "))
		}
	}
	dead := Dead(ns)
	heads := map[string][]string{}
	var live []note.Note
	var pruned map[string]bool // read from git only when a supersedes entry names no note on disk
	unseenIDs := 0             // supersedes entries a shallow clone cannot check
	for _, n := range ns {
		for _, id := range n.Supersedes {
			if files[id] == nil && pruned == nil {
				pruned = s.prunedIDs()
			}
			switch {
			case files[id] != nil || pruned[id]:
			case s.shallow():
				unseenIDs++
			default:
				report("%s: supersedes unknown note %s", n.File, id)
			}
		}
		if dead[n.ID] {
			continue
		}
		live = append(live, n)
		if n.Topic != "" {
			heads[n.Topic] = append(heads[n.Topic], n.ID)
		}
	}
	resolved := s.Anchors(live)
	merged := s.onDefault()
	onBranch := func(n note.Note) bool { return merged != nil && !merged[path.Base(n.File)] }
	unseen := 0
	for _, n := range live {
		for _, p := range n.Problems() {
			report("%s: %s", n.File, p)
		}
		for _, r := range resolved[n.File] {
			switch {
			case r.Unseen:
				unseen++
				if r.Problem != "" {
					report("%s: %s: %s", n.File, r.Spec, r.Problem)
				}
			case r.Problem != "":
				report("%s: %s: %s", n.File, r.Spec, r.Problem)
			default:
				branchRule := onBranch(n) && r.Anchor.From > 0
				if !branchRule && !review {
					continue
				}
				changed, err := anchor.Changed(s.Git, r.Base, r.Anchor)
				switch {
				case err != nil:
					rep.Warnings = append(rep.Warnings, fmt.Sprintf("%s: %s: could not check whether the code changed: %v", n.File, r.Spec, err))
				case changed && branchRule:
					report("%s: %s: the anchored code changed after the note; save a revision anchored to the code as it is now", n.File, r.Spec)
				case changed:
					rep.Review = append(rep.Review, fmt.Sprintf("%s: %s changed since its base commit; re-read the note", n.File, r.Spec))
				}
			}
		}
	}
	if merged == nil && slices.ContainsFunc(live, func(n note.Note) bool {
		return slices.ContainsFunc(resolved[n.File], func(r anchor.Resolved) bool { return r.Anchor.From > 0 })
	}) {
		rep.Warnings = append(rep.Warnings, "no default branch (origin/HEAD, main, master, origin/main or origin/master), so line-range anchors on this branch were not checked for code that changed after the note")
	}
	if unseenIDs > 0 {
		rep.Warnings = append(rep.Warnings, fmt.Sprintf("shallow clone: %d supersedes entries name notes that are not here and may be pruned in history the clone lacks; fetch the full history to check them", unseenIDs))
	}
	if unseen > 0 {
		rep.Warnings = append(rep.Warnings, fmt.Sprintf("shallow clone: %d anchor commits are not here, so only their files were checked; fetch the full history to check them", unseen))
	}
	for _, t := range slices.Sorted(maps.Keys(heads)) {
		if len(heads[t]) > 1 {
			report("topic %s has %d live notes (parallel revisions): %s; save one note that supersedes them all", t, len(heads[t]), strings.Join(heads[t], ", "))
		}
	}
	return rep
}
