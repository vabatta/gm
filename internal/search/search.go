// Package search finds the notes that match a query.
//
// It is the port of a ports-and-adapters design, like package importer. A
// Ranker is the port: an adapter scores notes against the query words, and
// nothing else. The rules every ranker shares are written once, in Find:
// which words of the query count, which notes are searched, and how scores
// become hits. Adapters live in subpackages, such as search/bm25, and the
// command picks one; this package imports none.
package search

import (
	"cmp"
	"errors"
	"regexp"
	"slices"
	"strings"

	"github.com/vabatta/gm/internal/memory"
	"github.com/vabatta/gm/internal/note"
)

// Ranker is the port each search adapter implements.
type Ranker interface {
	// Rank scores each note in ns against terms, the lowercase query words
	// without stop words. The result has one score per note, in the order of
	// ns; 0 means the note does not match.
	Rank(ns []note.Note, terms []string) []float64
}

// Query is one search.
type Query struct {
	// Words is the query text. Find splits it into words and drops the words
	// in stop. A note matches when it holds any of the remaining words.
	Words string
	Limit int    // the most hits Find returns
	Type  string // when set, only notes of this type match
	All   bool   // when set, superseded notes match too, pruned ones included
}

// Hit is one search result. Superseded is true when another note supersedes
// it, and PrunedIn names the commit that pruned it, for a note read from git
// history. Score is the ranker's score, and a higher score ranks first.
type Hit struct {
	ID         string  `json:"id"`
	Type       string  `json:"type"`
	Created    string  `json:"created"`
	Title      string  `json:"title"`
	File       string  `json:"file"`
	Superseded bool    `json:"superseded,omitempty"`
	PrunedIn   string  `json:"pruned_in,omitempty"`
	Score      float64 `json:"score"`
}

var (
	// wordRe finds words the way the FTS5 unicode61 tokenizer does, as runs of
	// letters and digits, except that it keeps diacritics.
	wordRe = regexp.MustCompile(`[\pL\pN]+`)
	// stop holds common English words that would match almost every note.
	stop = map[string]bool{}
)

func init() {
	for w := range strings.FieldsSeq("the a an of to in on for and or is are was were why what how did do does we our it that this with from by at as be not no") {
		stop[w] = true
	}
}

// Words returns the words of s in lowercase. Rankers split note text with it,
// so a note's words and the query's words always compare alike.
func Words(s string) []string { return wordRe.FindAllString(strings.ToLower(s), -1) }

// Find returns at most q.Limit notes that match q, best first, as r ranks
// them. r scores every note on disk, superseded ones included, so corpus
// statistics do not depend on --all; --all adds the pruned notes from git
// history. Notes with equal scores keep file order, so the older note comes
// first.
func Find(st *memory.Store, r Ranker, q Query) ([]Hit, error) {
	var terms []string
	for _, w := range Words(q.Words) {
		if !stop[w] && !slices.Contains(terms, w) {
			terms = append(terms, w)
		}
	}
	if len(terms) == 0 {
		return nil, errors.New("empty query")
	}
	ns, _ := st.Load()
	if q.All {
		ns = st.All()
	}
	dead := memory.Dead(ns)
	scores := r.Rank(ns, terms)
	var hits []Hit
	for i, n := range ns {
		if scores[i] == 0 || (dead[n.ID] && !q.All) || (q.Type != "" && n.Type != q.Type) {
			continue
		}
		hits = append(hits, Hit{ID: n.ID, Type: n.Type, Created: n.Created, Title: n.Title, File: n.File,
			Superseded: dead[n.ID], PrunedIn: n.PrunedIn, Score: scores[i]})
	}
	slices.SortStableFunc(hits, func(x, y Hit) int { return cmp.Compare(y.Score, x.Score) })
	return hits[:min(len(hits), q.Limit)], nil
}
