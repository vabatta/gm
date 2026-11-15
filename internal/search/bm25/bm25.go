// Package bm25 is the search adapter that ranks notes with BM25, the
// relevance formula that SQLite FTS5 uses. It computes the scores over the
// note files on every search, so there is no index to build, keep fresh or
// git-ignore.
package bm25

import (
	"math"
	"strings"

	"github.com/vabatta/gm/internal/anchor"
	"github.com/vabatta/gm/internal/note"
	"github.com/vabatta/gm/internal/search"
)

// Ranker ranks with BM25 over three fields. A query word counts 5 times in
// the title, 3 times in the keywords, topic and anchor paths, and once in the
// body. Rank computes document frequency and average length over every note
// it is given, as FTS5 does.
type Ranker struct{}

// k1 and b are the BM25 parameters. They have the SQLite FTS5 values, so the
// ranking matches FTS5.
const k1, b = 1.2, 0.75

// split turns the separators in topics and paths into spaces, so that
// decision/cache-store and internal/search/search.go give searchable words.
var split = strings.NewReplacer("/", " ", ".", " ", "_", " ", "-", " ")

// Rank implements search.Ranker.
func (Ranker) Rank(ns []note.Note, terms []string) []float64 {
	wanted := map[string]bool{}
	for _, t := range terms {
		wanted[t] = true
	}
	tfs := make([]map[string]float64, len(ns)) // weighted count of each query word per note
	lens := make([]int, len(ns))               // all words per note, unweighted, as FTS5 counts length
	df := map[string]int{}
	total := 0
	for i, n := range ns {
		kw := []string{n.Keywords, split.Replace(n.Topic)}
		for _, s := range n.At {
			if a, err := anchor.Parse(s); err == nil {
				kw = append(kw, split.Replace(a.Path))
			}
		}
		tf := map[string]float64{}
		for _, field := range []struct {
			text   string
			weight float64
		}{{n.Title, 5}, {strings.Join(kw, " "), 3}, {n.Body, 1}} {
			for _, w := range search.Words(field.text) {
				lens[i]++
				if wanted[w] {
					tf[w] += field.weight
				}
			}
		}
		for w := range tf {
			df[w]++
		}
		total += lens[i]
		tfs[i] = tf
	}
	avg := float64(total) / float64(max(len(ns), 1))
	scores := make([]float64, len(ns))
	for i, tf := range tfs {
		// Sum in the order of terms, not of the map. Float addition depends
		// on order, and near-tied notes would swap places between runs.
		for _, w := range terms {
			f := tf[w]
			if f == 0 {
				continue
			}
			idf := max(math.Log((float64(len(ns)-df[w])+0.5)/(float64(df[w])+0.5)), 1e-6)
			scores[i] += idf * f * (k1 + 1) / (f + k1*(1-b+b*float64(lens[i])/avg))
		}
	}
	return scores
}
