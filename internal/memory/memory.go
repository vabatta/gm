// Package memory keeps the notes of one repository in <root>/.memory/notes.
//
// A note is never edited. A revision is a new note that names the old one in
// supersedes, and a retirement supersedes notes with no successor. When
// parallel branches revise one topic, both notes stay live after the merge and
// Lint fails until one note supersedes both, so a merge never loses a note.
package memory

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/vabatta/gm/internal/anchor"
	"github.com/vabatta/gm/internal/git"
	"github.com/vabatta/gm/internal/note"
)

// Store is the memory of the worktree it was opened in.
type Store struct{ Git *git.Repo }

// Open returns the store of the worktree that contains dir.
func Open(dir string) (*Store, error) {
	r, err := git.Open(dir)
	if err != nil {
		return nil, err
	}
	return &Store{Git: r}, nil
}

// Dir is the .memory directory.
func (s *Store) Dir() string { return filepath.Join(s.Git.Root, ".memory") }

// NotesDir holds one file per note.
func (s *Store) NotesDir() string { return filepath.Join(s.Dir(), "notes") }

// Files lists the note files sorted by name, so by id.
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

// Load reads every note; bad lists the files that are not notes, with the reason.
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

// Dead reports the ids that some note supersedes.
func Dead(ns []note.Note) map[string]bool {
	dead := map[string]bool{}
	for _, n := range ns {
		for _, s := range n.Supersedes {
			dead[s] = true
		}
	}
	return dead
}

// Live drops superseded notes and retirements.
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

// Saved is a note just written. Others lists the live notes of its topic that it does not supersede.
type Saved struct {
	Note   note.Note
	Others []string
}

// Save pins each of at to HEAD, checks the note, and writes it, refusing it with every problem at once.
func (s *Store) Save(n note.Note, at []string) (Saved, error) {
	var problems []string
	for _, spec := range at {
		a, err := anchor.Pin(s.Git, spec)
		if err != nil {
			problems = append(problems, err.Error())
			continue
		}
		n.At = append(n.At, a.String())
	}
	return s.create(n, problems)
}

// Retire supersedes the notes ids with a retirement that says why they no longer hold.
func (s *Store) Retire(ids []string, why string) (Saved, error) {
	if len(ids) == 0 {
		return Saved{}, errors.New("retire needs the ids of the notes to retire")
	}
	var titles []string
	for _, id := range ids {
		n, err := s.Get(id)
		if err != nil {
			return Saved{}, err
		}
		titles = append(titles, n.Title)
	}
	return s.create(note.Note{
		Type:       "retirement",
		Title:      note.Shorten("Retired: "+strings.Join(titles, "; "), note.MaxTitle),
		Keywords:   "retired retirement obsolete " + strings.Join(titles, " "),
		Supersedes: ids,
		Body:       note.Body("retirement", map[string]string{"Why": why}),
	}, nil)
}

func (s *Store) create(n note.Note, problems []string) (Saved, error) {
	now := time.Now()
	n.ID, n.Created = note.NewID(now), now.UTC().Format(time.RFC3339)
	ns, _ := s.Load()
	ids := map[string]bool{}
	for _, x := range ns {
		ids[x.ID] = true
	}
	for _, id := range n.Supersedes {
		if !ids[id] {
			problems = append(problems, "supersedes unknown note "+id)
		}
	}
	problems = append(problems, n.Problems()...)
	if len(problems) > 0 {
		return Saved{}, errors.New("note refused:\n  " + strings.Join(problems, "\n  "))
	}
	n, err := s.Write(n)
	if err != nil {
		return Saved{}, err
	}
	saved := Saved{Note: n}
	if n.Topic != "" {
		for _, o := range Live(ns) {
			if o.Topic == n.Topic && !slices.Contains(n.Supersedes, o.ID) {
				saved.Others = append(saved.Others, o.ID)
			}
		}
	}
	return saved, nil
}

// Write stores n in a new file without checking it, and never overwrites a file.
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

// Report is the outcome of Lint. Problems fail it. Review lists anchors to re-read and Warnings
// what lint could not check; neither fails it.
type Report struct {
	Problems []string `json:"problems"`
	Review   []string `json:"review,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

// Lint checks every note: parse errors, duplicate ids, broken supersedes, the
// rules of live notes, anchors that no longer point at code, and topics with
// more than one live note. With review it also lists live anchors whose code changed since.
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
				if k := anchor.Changed(s.Git, a); k > 0 {
					rep.Review = append(rep.Review, fmt.Sprintf("%s: %s changed in %d commits since; re-read the note", n.File, spec, k))
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
