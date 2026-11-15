// Package engram is the importer adapter for Engram, read from a JSON export
// made with engram export FILE --project NAME.
package engram

import (
	"encoding/json"
	"os"
	"time"

	"github.com/vabatta/gm/internal/importer"
)

// Source reads Engram exports.
type Source struct{}

// Name is "engram".
func (Source) Name() string { return "engram" }

// export holds the fields of an Engram export that Read uses.
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

// types maps Engram observation types to note types. Any other type becomes a fact.
var types = map[string]string{"decision": "decision", "learning": "lesson", "bugfix": "lesson"}

// Read returns one record per observation. Only the supersedes relations that
// Engram marked judged count, because the others were never confirmed.
func (Source) Read(file string) ([]importer.Record, error) {
	raw, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	var ex export
	if err := json.Unmarshal(raw, &ex); err != nil {
		return nil, err
	}
	sup := map[string][]string{}
	for _, r := range ex.Relations {
		if r.Relation == "supersedes" && r.JudgmentStatus == "judged" {
			sup[r.SourceID] = append(sup[r.SourceID], r.TargetID)
		}
	}
	recs := make([]importer.Record, 0, len(ex.Observations))
	for _, o := range ex.Observations {
		typ := types[o.Type]
		if typ == "" {
			typ = "fact"
		}
		t, _ := time.Parse(time.DateTime, o.CreatedAt)
		recs = append(recs, importer.Record{ID: o.SyncID, Kind: o.Type, Type: typ, Title: o.Title, Body: o.Content,
			Topic: o.TopicKey, Created: t, Supersedes: sup[o.SyncID]})
	}
	return recs, nil
}
