package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/vabatta/gm/internal/memory"
	"github.com/vabatta/gm/internal/search"
	"github.com/vabatta/gm/internal/search/bm25"
)

// ranker is the search adapter gm uses. Another ranker, such as an index or
// a git grep prefilter, is an adapter package that implements search.Ranker.
// Switching to it changes only this line. bench/run.sh compares it with the
// one before.
var ranker search.Ranker = bm25.Ranker{}

// find runs gm search.
func find(st *memory.Store, args []string, w io.Writer) error {
	fs := flags("search")
	q := search.Query{}
	fs.IntVar(&q.Limit, "n", 5, "")
	fs.StringVar(&q.Type, "type", "", "")
	fs.BoolVar(&q.All, "all", false, "")
	asJSON := jsonFlag(fs)
	if err := parse(fs, args); err != nil {
		return err
	}
	q.Words = strings.Join(fs.Args(), " ")
	hits, err := search.Find(st, ranker, q)
	if err != nil {
		return err
	}
	if *asJSON {
		emit(w, append([]search.Hit{}, hits...))
		return nil
	}
	for _, h := range hits {
		mark := ""
		switch {
		case h.PrunedIn != "":
			mark = "  (superseded, pruned in " + h.PrunedIn + ")"
		case h.Superseded:
			mark = "  (superseded)"
		}
		fmt.Fprintf(w, "%s  %s %s  %s%s\n    %s\n", h.ID, h.Type, day(h.Created), h.Title, mark, h.File)
	}
	return nil
}
