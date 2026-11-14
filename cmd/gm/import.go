package main

import (
	"errors"
	"fmt"
	"io"

	"github.com/vabatta/gm/internal/engram"
	"github.com/vabatta/gm/internal/memory"
)

// importEngram runs gm import-engram FILE and prints what it wrote and which
// observations it skipped as secrets.
func importEngram(st *memory.Store, args []string, w io.Writer) error {
	if len(args) != 1 {
		return errors.New("import-engram needs the export file")
	}
	res, err := engram.Import(st, args[0])
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "imported %d notes with %d supersedes links; other relations dropped\n", res.Notes, res.Links)
	for _, r := range res.Refused {
		fmt.Fprintln(w, "not imported, looks like a secret:", r)
	}
	return nil
}
