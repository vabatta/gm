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
// the commit is gone, show sets Error instead.
type code struct {
	Anchor string   `json:"anchor"`
	ReadAt string   `json:"read_at,omitempty"` // the base, when it is not the anchor's own commit
	From   int      `json:"from,omitempty"`
	Lines  []string `json:"lines,omitempty"`
	Error  string   `json:"error,omitempty"`
	why    string   // why the base is ReadAt, for the heading
}

// show runs gm show. It prints the note file, then the code of each anchor,
// read at the base commit that Store.Anchors picks. When the base is not the
// anchor's own commit, the heading names the base and why. For a line range it
// prints those lines, or the reason when the file at the base ends before them.
// For a whole file it prints the first 40 lines.
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
	for _, r := range st.Anchors([]note.Note{n})[n.File] {
		c, a := code{Anchor: r.Spec, From: 1}, r.Anchor
		switch {
		case r.Unseen:
			c.ReadAt, c.why = "HEAD", "because this shallow clone lacks commit "+a.Commit
		case r.Fallback && r.Base == "HEAD":
			c.ReadAt, c.why = "HEAD", "because the note is not committed yet"
		case r.Fallback:
			c.ReadAt, c.why = r.Base[:12], "the commit that added the note"
		}
		var err error
		if r.Problem != "" && r.Base == "" {
			err = errors.New(r.Problem)
		} else {
			c.Lines, err = anchor.Code(st.Git, r.Base, a)
		}
		switch {
		case err != nil:
			c.Error = err.Error()
		case a.From > len(c.Lines):
			c.Error, c.Lines = fmt.Sprintf("the file has %d lines at %s, fewer than the anchor names", len(c.Lines), r.Base[:min(12, len(r.Base))]), nil
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
		if c.ReadAt != "" {
			fmt.Fprintf(w, "\n# %s (read at %s, %s)\n", c.Anchor, c.ReadAt, c.why)
		} else {
			fmt.Fprintf(w, "\n# %s\n", c.Anchor)
		}
		for i, line := range c.Lines {
			fmt.Fprintf(w, "%5d  %s\n", c.From+i, line)
		}
	}
	return nil
}
