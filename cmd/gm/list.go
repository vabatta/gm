package main

import (
	"cmp"
	"errors"
	"fmt"
	"io"
	"slices"

	"github.com/vabatta/gm/internal/memory"
	"github.com/vabatta/gm/internal/note"
)

// list runs gm list. It shows live notes unless --all is set. --by type groups
// them by type, newest first in each group. --by time shows them newest first.
// --json leaves out the bodies, because gm show prints a body.
func list(st *memory.Store, args []string, w io.Writer) error {
	fs := flags("list")
	by := fs.String("by", "type", "")
	limit := countFlag(fs, 0)
	all := fs.Bool("all", false, "")
	asJSON := jsonFlag(fs)
	if err := parse(fs, args); err != nil {
		return err
	}
	if *by != "type" && *by != "time" {
		return errors.New("--by must be type or time")
	}
	ns, _ := st.Load()
	if *all {
		ns = st.All()
	} else {
		ns = memory.Live(ns)
	}
	slices.SortStableFunc(ns, func(a, b note.Note) int {
		byType := 0
		if *by == "type" {
			byType = cmp.Compare(a.Type, b.Type)
		}
		return cmp.Or(byType, cmp.Compare(b.Created, a.Created))
	})
	if *limit > 0 && len(ns) > *limit {
		ns = ns[:*limit]
	}
	if *asJSON {
		for i := range ns {
			ns[i].Body = ""
		}
		emit(w, append([]note.Note{}, ns...))
		return nil
	}
	cur := ""
	for _, n := range ns {
		if *by == "type" && n.Type != cur {
			cur = n.Type
			fmt.Fprintf(w, "\n%s\n", cur)
		}
		fmt.Fprintf(w, "- %s  %s  (%s)\n", day(n.Created), n.Title, n.ID)
	}
	return nil
}

// day returns the date part of an RFC 3339 time.
func day(created string) string { return created[:min(10, len(created))] }
