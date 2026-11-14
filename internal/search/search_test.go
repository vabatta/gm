package search_test

import (
	"testing"

	"github.com/vabatta/gm/internal/gittest"
	"github.com/vabatta/gm/internal/note"
	"github.com/vabatta/gm/internal/search"
)

func TestSearchRanksTitleMatchesFirstAndHidesSupersededNotesUnlessAll(t *testing.T) {
	st := gittest.New(t)
	save := func(title, keywords, fact string, sup ...string) string {
		s, err := st.Save(note.Note{Type: "fact", Title: title, Keywords: keywords, Supersedes: sup,
			Body: note.Body("fact", map[string]string{"Fact": fact, "Evidence": "measured"})}, nil)
		if err != nil {
			t.Fatal(err)
		}
		return s.Note.ID
	}
	body := save("Workers share one queue", "jobs backlog pool", "the proxy budget is checked per job")
	title := save("Proxy budget ignores bytes", "cost flat rate", "billed flat")
	if hits, err := search.Find(st, search.Query{Words: "proxy budget", Limit: 5}); err != nil || len(hits) != 2 || hits[0].ID != title {
		t.Fatalf("want the title match first, got %+v %v", hits, err)
	}
	save("Proxy budget counts requests", "cost flat rate", "per request", title)
	hits, _ := search.Find(st, search.Query{Words: "proxy budget", Limit: 5})
	for _, h := range hits {
		if h.ID == title {
			t.Fatalf("want the superseded note hidden, got %+v", hits)
		}
	}
	hits, _ = search.Find(st, search.Query{Words: "proxy budget", Limit: 5, All: true})
	if len(hits) != 3 || hits[len(hits)-1].ID != body {
		t.Fatalf("want all three with --all, body match last, got %+v", hits)
	}
}
