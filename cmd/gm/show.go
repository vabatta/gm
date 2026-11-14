package main

import (
	"errors"
	"fmt"
	"io"

	"github.com/vabatta/gm/internal/anchor"
	"github.com/vabatta/gm/internal/memory"
	"github.com/vabatta/gm/internal/note"
)

// code is the code of one anchor as gm show prints it. From is the number of
// the first line in Lines. When git cannot read the code, for example because
// the commit is gone, Error is set instead.
type code struct {
	Anchor string   `json:"anchor"`
	From   int      `json:"from,omitempty"`
	Lines  []string `json:"lines,omitempty"`
	Error  string   `json:"error,omitempty"`
}

// show runs gm show. It prints the note file, then the code of each anchor,
// read at the anchor's commit. For a line range it prints those lines. For a
// whole file it prints the first 40 lines.
func show(st *memory.Store, args []string, w io.Writer) error {
	fs := flags("show")
	asJSON := jsonFlag(fs)
	if err := parse(fs, args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("show needs one note id")
	}
	n, err := st.Get(fs.Arg(0))
	if err != nil {
		return err
	}
	var codes []code
	for _, s := range n.At {
		c := code{Anchor: s, From: 1}
		a, err := anchor.Parse(s)
		if err == nil {
			c.Lines, err = anchor.Code(st.Git, a)
		}
		switch {
		case err != nil:
			c.Error = err.Error()
		case a.From > 0:
			c.From, c.Lines = a.From, c.Lines[a.From-1:]
		case len(c.Lines) > 40:
			c.Lines = c.Lines[:40]
		}
		codes = append(codes, c)
	}
	if *asJSON {
		emit(w, struct {
			note.Note
			Code []code `json:"code"`
		}{n, codes})
		return nil
	}
	if n.PrunedIn != "" {
		fmt.Fprintf(w, "# pruned in %s\n", n.PrunedIn)
	}
	fmt.Fprint(w, n.Render())
	for _, c := range codes {
		if c.Error != "" {
			fmt.Fprintf(w, "\n# %s: %s\n", c.Anchor, c.Error)
			continue
		}
		fmt.Fprintf(w, "\n# %s\n", c.Anchor)
		for i, line := range c.Lines {
			fmt.Fprintf(w, "%5d  %s\n", c.From+i, line)
		}
	}
	return nil
}
