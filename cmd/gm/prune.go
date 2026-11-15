package main

import (
	"fmt"
	"io"

	"github.com/vabatta/gm/internal/memory"
)

// prune runs gm prune. It prints one line per deleted or kept file, then how
// pruned notes stay readable.
func prune(st *memory.Store, args []string, w io.Writer) error {
	if err := parse(flags("prune"), args); err != nil {
		return err
	}
	pruned, kept, err := st.Prune()
	for _, p := range pruned {
		fmt.Fprintln(w, "pruned", p)
	}
	for _, k := range kept {
		fmt.Fprintln(w, "kept", k+": not committed as it is at HEAD; commit it, then prune")
	}
	if err == nil && len(pruned) > 0 {
		fmt.Fprintln(w, "gm: commit the deletions; gm show and --all read pruned notes from git history")
	}
	return err
}
