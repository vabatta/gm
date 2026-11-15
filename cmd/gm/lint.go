package main

import (
	"fmt"
	"io"

	"github.com/vabatta/gm/internal/memory"
)

// lintFailed is the error lint returns when it finds problems, so run exits 1
// instead of 2. lint has already printed the problems.
type lintFailed int

// Error returns the number of problems as the error text.
func (n lintFailed) Error() string { return fmt.Sprintf("%d problems", int(n)) }

// lint runs gm lint. It prints one line per problem, then the review: and
// warning: lines, or all of them as one JSON object with --json.
func lint(st *memory.Store, args []string, w io.Writer) error {
	fs := flags("lint")
	review, asJSON := fs.Bool("review", false, ""), jsonFlag(fs)
	if err := parse(fs, args); err != nil {
		return err
	}
	rep := st.Lint(*review)
	if *asJSON {
		if rep.Problems == nil {
			rep.Problems = []string{}
		}
		emit(w, rep)
	} else {
		for _, p := range rep.Problems {
			fmt.Fprintln(w, p)
		}
		for _, r := range rep.Review {
			fmt.Fprintln(w, "review:", r)
		}
		for _, r := range rep.Warnings {
			fmt.Fprintln(w, "warning:", r)
		}
	}
	if len(rep.Problems) > 0 {
		return lintFailed(len(rep.Problems))
	}
	return nil
}
