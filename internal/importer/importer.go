// Package importer writes memories exported from other tools as gm notes.
//
// It is the port of a ports-and-adapters design. A Source is the port: an
// adapter reads one tool's export format into Records, and nothing else. The
// rules that make a Record a note are written once, in Import, so every
// source gets the same ids, links and title rules. Adapters live
// in subpackages, and the command wires them in; this package imports none.
package importer

import (
	"strings"
	"time"

	"github.com/vabatta/gm/internal/memory"
	"github.com/vabatta/gm/internal/note"
)

// Record is one memory from another tool, already mapped to a gm note type.
type Record struct {
	ID         string    // the memory's id in the source, kept in the migrated header
	Kind       string    // the memory's type in the source, kept in the migrated header
	Type       string    // the gm note type: decision, lesson or fact
	Title      string    // may be long or span lines; Import shortens it
	Body       string    // free text, kept as it is
	Topic      string    // may hold spaces; Import turns them into hyphens
	Created    time.Time // zero when the source has no time; Import then uses now
	Supersedes []string  // ids in the source of the memories this one replaces
}

// Source is the port each adapter implements.
type Source interface {
	// Name is the source as the command line and the migrated header name it.
	Name() string
	// Read parses an export file into records.
	Read(file string) ([]Record, error)
}

// Result is what Import wrote. Notes counts the note files and Links the
// supersedes links kept.
type Result struct{ Notes, Links int }

// Import reads file with src and writes one note per record. Each note gets a
// new id from note.NewID at the record's creation time, because ids derived
// from a source's own ids collided. A supersedes link is kept when both
// records are written. A title on several lines or longer than note.MaxTitle
// is cut, and the full title becomes the first paragraph of the body. The
// migrated header records the source, the record's id and its kind, and
// exempts the note from the section templates, which other tools' text does
// not follow.
func Import(st *memory.Store, src Source, file string) (Result, error) {
	recs, err := src.Read(file)
	if err != nil {
		return Result{}, err
	}
	var res Result
	ids := map[string]string{} // source id -> new note id
	created := map[string]time.Time{}
	for _, r := range recs {
		t := r.Created
		if t.IsZero() {
			t = time.Now()
		}
		created[r.ID], ids[r.ID] = t, note.NewID(t)
	}
	for _, r := range recs {
		var sup []string
		for _, old := range r.Supersedes {
			if ids[old] != "" {
				sup = append(sup, ids[old])
			}
		}
		res.Links += len(sup)
		title, body := strings.TrimSpace(r.Title), strings.TrimSpace(r.Body)
		if short := note.Shorten(strings.SplitN(title, "\n", 2)[0], note.MaxTitle); short != title {
			title, body = short, title+"\n\n"+body
		}
		n := note.Note{
			ID: ids[r.ID], Type: r.Type, Title: title, Topic: strings.Join(strings.Fields(r.Topic), "-"),
			Supersedes: sup, Created: created[r.ID].UTC().Format(time.RFC3339),
			Migrated: src.Name() + " " + r.ID + " (" + r.Kind + ")", Body: body + "\n",
		}
		if _, err := st.Write(n); err != nil {
			return res, err
		}
		res.Notes++
	}
	return res, nil
}
