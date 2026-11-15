package memory

import (
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/vabatta/gm/internal/anchor"
	"github.com/vabatta/gm/internal/note"
)

// Saved is the result of Save and Retire. Others lists the live notes of the
// same topic that the new note does not supersede, so gm save can warn about a
// parallel revision.
type Saved struct {
	Note   note.Note
	Others []string
}

// Save pins each spec in at to HEAD with anchor.Pin, gives the note a new id
// and creation time, checks it, and writes it to a new file. On any problem it
// writes nothing and returns one error that lists every problem.
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

// Retire writes a retirement that supersedes the notes named by ids and
// records why in its Why section. Its title lists the retired titles, cut to
// note.MaxTitle. Retire returns an error when ids is empty or an id names no
// note.
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

// create gives n its id and creation time, checks it, writes it, and finds the
// other live notes of its topic. problems holds problems found earlier, such
// as anchors that Pin refused. A supersedes id may name a note on disk or one
// pruned into git history.
func (s *Store) create(n note.Note, problems []string) (Saved, error) {
	now := time.Now()
	n.ID, n.Created = note.NewID(now), now.UTC().Format(time.RFC3339)
	ns, _ := s.Load()
	ids := map[string]bool{}
	for _, x := range ns {
		ids[x.ID] = true
	}
	var pruned map[string]bool // read from git only when an id names no note on disk
	for _, id := range n.Supersedes {
		if !ids[id] && pruned == nil {
			pruned = s.prunedIDs()
		}
		if !ids[id] && !pruned[id] {
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
