package main

import (
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"

	"github.com/vabatta/gm/internal/importer"
	"github.com/vabatta/gm/internal/importer/engram"
	"github.com/vabatta/gm/internal/memory"
)

// sources are the importer adapters gm reads, by the name gm import takes.
// A new source is an adapter package that implements importer.Source and one
// entry here.
var sources = map[string]importer.Source{
	"engram": engram.Source{},
}

// importFrom runs gm import SOURCE FILE and prints what it wrote and which
// records it skipped as secrets.
func importFrom(st *memory.Store, args []string, w io.Writer) error {
	names := slices.Sorted(maps.Keys(sources))
	if len(args) != 2 {
		return fmt.Errorf("import needs a source and the export file; sources: %s", strings.Join(names, ", "))
	}
	src, ok := sources[args[0]]
	if !ok {
		return fmt.Errorf("unknown source %q; sources: %s", args[0], strings.Join(names, ", "))
	}
	res, err := importer.Import(st, src, args[1])
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "imported %d notes with %d supersedes links; other relations dropped\n", res.Notes, res.Links)
	for _, r := range res.Refused {
		fmt.Fprintln(w, "not imported, looks like a secret:", r)
	}
	return nil
}
