// Package search ranks notes against a query with BM25, the relevance formula
// that SQLite FTS5 uses. It reads the note files on every search. A few hundred
// small files take milliseconds to read, so there is no index to build, keep
// fresh or git-ignore.
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

// Query is one search.
type Query struct {
	// Words is the query text. Find splits it into words and drops the words
	// in stop. A note matches when it holds any of the remaining words.
	Words string
	Limit int    // the most hits Find returns
	Type  string // when set, only notes of this type match
	All   bool   // when set, superseded notes match too
}

// Hit is one search result. Superseded is true when another note supersedes
// it. Score is the BM25 score, and a higher score ranks first.
type Hit struct {
	ID         string  `json:"id"`
	Type       string  `json:"type"`
	Created    string  `json:"created"`
	Title      string  `json:"title"`
	File       string  `json:"file"`
	Superseded bool    `json:"superseded,omitempty"`
	Score      float64 `json:"score"`
}

// k1 and b are the BM25 parameters. They have the SQLite FTS5 values, so the
// ranking matches the FTS5 index that gm used before.
const k1, b = 1.2, 0.75

var (
	// wordRe finds words the way the FTS5 unicode61 tokenizer does, as runs of
	// letters and digits, except that it keeps diacritics.
	wordRe = regexp.MustCompile(`[\pL\pN]+`)
	// split turns the separators in topics and paths into spaces, so that
	// decision/cache-store and internal/search/search.go give searchable words.
	split = strings.NewReplacer("/", " ", ".", " ", "_", " ", "-", " ")
	// stop holds common English words that would match almost every note.
	stop = map[string]bool{}
)

func init() {
	for w := range strings.FieldsSeq("the a an of to in on for and or is are was were why what how did do does we our it that this with from by at as be not no") {
		stop[w] = true
	}
}

// words returns the words of s in lowercase.
func words(s string) []string { return wordRe.FindAllString(strings.ToLower(s), -1) }

// Find returns at most q.Limit notes that match q, best first.
//
// The score is BM25 over three fields. A query word counts 5 times in the
// title, 3 times in the keywords, topic and anchor paths, and once in the body.
// Document frequency and average length cover all notes, superseded ones
// included, as in FTS5. Notes with equal scores keep file order, so the older
// note comes first.
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
		tf  map[string]float64 // weighted count of each query word in the note
		len int                // all words in the note, unweighted, as FTS5 counts length
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
			Superseded: dead[n.ID]}, tf: map[string]float64{}}
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
