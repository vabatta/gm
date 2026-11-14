// Package search ranks notes against a query with BM25, computed in memory
// from the note files on every search. With no index there is nothing to
// keep fresh, ignore or rebuild.
package search

import (
	"cmp"
	"errors"
	"math"
	"regexp"
	"slices"
	"strings"

	"github.com/vabatta/gm/internal/anchor"
	"github.com/vabatta/gm/internal/memory"
)

// Query is a search: any of its words may match. Notes rank by BM25 with
// term counts weighted 5 in the title, 3 in the keywords and 1 in the body.
// Topic and anchor paths count as keywords.
type Query struct {
	Words string
	Limit int
	Type  string // only notes of this type when set
	All   bool   // include superseded and retired notes
}

// Hit is one search result.
type Hit struct {
	ID         string  `json:"id"`
	Type       string  `json:"type"`
	Created    string  `json:"created"`
	Title      string  `json:"title"`
	File       string  `json:"file"`
	Superseded bool    `json:"superseded,omitempty"`
	Score      float64 `json:"score"`
}

// BM25 parameters, the ones SQLite FTS5 uses.
const k1, b = 1.2, 0.75

var (
	wordRe = regexp.MustCompile(`[\pL\pN]+`)
	split  = strings.NewReplacer("/", " ", ".", " ", "_", " ", "-", " ")
	stop   = map[string]bool{}
)

func init() {
	for w := range strings.FieldsSeq("the a an of to in on for and or is are was were why what how did do does we our it that this with from by at as be not no") {
		stop[w] = true
	}
}

func words(s string) []string { return wordRe.FindAllString(strings.ToLower(s), -1) }

// Find returns the best matches for q, best first.
func Find(st *memory.Store, q Query) ([]Hit, error) {
	terms := map[string]bool{}
	for _, w := range words(q.Words) {
		if !stop[w] {
			terms[w] = true
		}
	}
	if len(terms) == 0 {
		return nil, errors.New("empty query")
	}
	ns, _ := st.Load()
	dead := memory.Dead(ns)
	type doc struct {
		hit Hit
		tf  map[string]float64 // weighted count of each query term
		len int
	}
	docs := make([]doc, len(ns))
	df := map[string]int{}
	total := 0
	for i, n := range ns {
		kw := []string{n.Keywords, split.Replace(n.Topic)}
		for _, s := range n.At {
			if a, err := anchor.Parse(s); err == nil {
				kw = append(kw, split.Replace(a.Path))
			}
		}
		d := doc{hit: Hit{ID: n.ID, Type: n.Type, Created: n.Created, Title: n.Title, File: n.File,
			Superseded: dead[n.ID] || n.Type == "retirement"}, tf: map[string]float64{}}
		for _, col := range []struct {
			text   string
			weight float64
		}{{n.Title, 5}, {strings.Join(kw, " "), 3}, {n.Body, 1}} {
			for _, w := range words(col.text) {
				d.len++
				if terms[w] {
					d.tf[w] += col.weight
				}
			}
		}
		for w := range d.tf {
			df[w]++
		}
		total += d.len
		docs[i] = d
	}
	avg := float64(total) / float64(max(len(docs), 1))
	var hits []Hit
	for _, d := range docs {
		if len(d.tf) == 0 || (d.hit.Superseded && !q.All) || (q.Type != "" && d.hit.Type != q.Type) {
			continue
		}
		for w, tf := range d.tf {
			idf := max(math.Log((float64(len(docs)-df[w])+0.5)/(float64(df[w])+0.5)), 1e-6)
			d.hit.Score += idf * tf * (k1 + 1) / (tf + k1*(1-b+b*float64(d.len)/avg))
		}
		hits = append(hits, d.hit)
	}
	slices.SortStableFunc(hits, func(x, y Hit) int { return cmp.Compare(y.Score, x.Score) })
	return hits[:min(len(hits), q.Limit)], nil
}
