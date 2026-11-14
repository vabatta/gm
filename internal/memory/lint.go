package memory

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/vabatta/gm/internal/anchor"
	"github.com/vabatta/gm/internal/note"
)

// Report is the result of Lint. Problems make gm lint exit 1. Review lists the
// anchors of live notes whose code changed since, and Warnings lists what Lint
// could not check. Neither Review nor Warnings makes gm lint fail.
type Report struct {
	Problems []string `json:"problems"`
	Review   []string `json:"review,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

// Lint checks every note. A problem is a file that does not parse, an id used
// by more than one file, a supersedes entry that names no note, a live note
// that breaks a rule in note.Problems, an anchor that anchor.Check finds
// broken, or a topic with more than one live note. Superseded notes and
// retirements are history, so of the per-note checks only the supersedes check
// applies to them. With
// review, Lint also runs anchor.Changed on each sound anchor of a live note.
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
	var specs []string
	for _, n := range ns {
		for _, id := range n.Supersedes {
			if files[id] == nil {
				report("%s: supersedes unknown note %s", n.File, id)
			}
		}
		if dead[n.ID] || n.Type == "retirement" {
			continue
		}
		live = append(live, n)
		specs = append(specs, n.At...)
		if n.Topic != "" {
			heads[n.Topic] = append(heads[n.Topic], n.ID)
		}
	}
	broken, unseen := anchor.Check(s.Git, specs)
	if unseen > 0 {
		rep.Warnings = append(rep.Warnings, fmt.Sprintf("shallow clone: %d anchor commits are not here, so only their files were checked; fetch the full history to check them", unseen))
	}
	for _, n := range live {
		for _, p := range n.Problems() {
			report("%s: %s", n.File, p)
		}
		for _, spec := range n.At {
			if p := broken[spec]; p != "" {
				report("%s: %s: %s", n.File, spec, p)
			} else if a, err := anchor.Parse(spec); review && err == nil {
				if anchor.Changed(s.Git, a) {
					rep.Review = append(rep.Review, fmt.Sprintf("%s: %s changed since its commit; re-read the note", n.File, spec))
				}
			}
		}
	}
	for _, t := range slices.Sorted(maps.Keys(heads)) {
		if len(heads[t]) > 1 {
			report("topic %s has %d live notes (parallel revisions): %s; save one note that supersedes them all", t, len(heads[t]), strings.Join(heads[t], ", "))
		}
	}
	return rep
}
