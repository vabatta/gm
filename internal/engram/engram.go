// Package engram migrates an Engram JSON export (`engram export FILE --project X`) into note files.
package engram

import (
	"encoding/json"
	"os"
	"strings"
	"time"

	"github.com/vabatta/gm/internal/memory"
	"github.com/vabatta/gm/internal/note"
)

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

// Result counts what Import wrote. Refused lists observations not written because they hold a secret.
type Result struct {
	Notes, Links int
	Refused      []string
}

// types maps Engram observation types to note types; any other type becomes a fact.
var types = map[string]string{"decision": "decision", "learning": "lesson", "bugfix": "lesson"}

// Import writes one note per observation with a new random id. It keeps the
// judged supersedes relations and drops the others. A title over the limit is
// shortened, and the full title opens the body. Each note records its origin
// in a migrated header, which also exempts it from the section template.
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
	ids := map[string]string{} // only for observations that will be written
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
