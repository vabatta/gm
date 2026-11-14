// Package engram turns an Engram JSON export, made with
// engram export FILE --project NAME, into gm note files. It serves the
// one-time move of existing memories from Engram to gm.
package engram

import (
	"encoding/json"
	"os"
	"strings"
	"time"

	"github.com/vabatta/gm/internal/memory"
	"github.com/vabatta/gm/internal/note"
)

// export holds the fields of an Engram export that Import uses.
type export struct {
	Observations []struct {
		SyncID    string `json:"sync_id"`
		Type      string `json:"type"`
		Title     string `json:"title"`
		Content   string `json:"content"`
		TopicKey  string `json:"topic_key"`
		CreatedAt string `json:"created_at"`
	} `json:"observations"`
	Relations []struct {
		SourceID       string `json:"source_id"`
		TargetID       string `json:"target_id"`
		Relation       string `json:"relation"`
		JudgmentStatus string `json:"judgment_status"`
	} `json:"relations"`
}

// Result is what Import wrote. Notes counts the note files and Links the
// supersedes links kept. Refused lists the observations that were not written
// because their text looks like a secret, as sync id and kind of secret, never
// as text.
type Result struct {
	Notes, Links int
	Refused      []string
}

// types maps Engram observation types to note types. Any other type becomes a fact.
var types = map[string]string{"decision": "decision", "learning": "lesson", "bugfix": "lesson"}

// Import writes one note file for each observation in the export at file.
//
// Each note gets a new id from note.NewID at the observation's creation time.
// Engram's own ids are not reused, because ids derived from them collided. A
// supersedes relation is kept only when Engram marked it judged, and all other
// relations are dropped. A title on several lines or longer than note.MaxTitle
// is cut, and the full title becomes the first paragraph of the body. Spaces
// in a topic key become hyphens, because a topic is one word. The migrated
// header records the Engram sync id and type. It also exempts the note from the
// section templates, which Engram text does not follow.
//
// An observation whose text looks like a secret is not written, and relations
// to it are dropped, so no credential reaches the repository.
func Import(st *memory.Store, file string) (Result, error) {
	raw, err := os.ReadFile(file)
	if err != nil {
		return Result{}, err
	}
	var ex export
	if err := json.Unmarshal(raw, &ex); err != nil {
		return Result{}, err
	}
	var res Result
	created := map[string]time.Time{}
	ids := map[string]string{} // sync id -> new note id, only for observations that will be written
	for _, o := range ex.Observations {
		if kind, bad := note.Secret(o.Title + "\n" + o.Content); bad {
			res.Refused = append(res.Refused, o.SyncID+" ("+kind+")")
			continue
		}
		t, err := time.Parse(time.DateTime, o.CreatedAt)
		if err != nil {
			t = time.Now()
		}
		created[o.SyncID], ids[o.SyncID] = t, note.NewID(t)
	}
	sup := map[string][]string{}
	for _, r := range ex.Relations {
		if r.Relation == "supersedes" && r.JudgmentStatus == "judged" && ids[r.SourceID] != "" && ids[r.TargetID] != "" {
			sup[r.SourceID] = append(sup[r.SourceID], ids[r.TargetID])
			res.Links++
		}
	}
	for _, o := range ex.Observations {
		if ids[o.SyncID] == "" {
			continue
		}
		typ := types[o.Type]
		if typ == "" {
			typ = "fact"
		}
		title, body := strings.TrimSpace(o.Title), strings.TrimSpace(o.Content)
		if short := note.Shorten(strings.SplitN(title, "\n", 2)[0], note.MaxTitle); short != title {
			title, body = short, title+"\n\n"+body
		}
		n := note.Note{
			ID: ids[o.SyncID], Type: typ, Title: title, Topic: strings.Join(strings.Fields(o.TopicKey), "-"),
			Supersedes: sup[o.SyncID], Created: created[o.SyncID].UTC().Format(time.RFC3339),
			Migrated: "engram " + o.SyncID + " (" + o.Type + ")", Body: body + "\n",
		}
		if _, err := st.Write(n); err != nil {
			return res, err
		}
		res.Notes++
	}
	return res, nil
}
