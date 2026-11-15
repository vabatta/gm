package main

import (
	"encoding/json"
	"errors"
	"flag"
	"io"
	"math"
	"strconv"
	"strings"
)

// flags returns a flag set that returns parse errors instead of printing
// usage and exiting, so run reports every error the same way.
func flags(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	return fs
}

// countFlag adds the -n flag to fs with default def. It refuses anything but
// a count of 0 or more, because a negative count would reach a slice bound,
// and it caps a count too large for an int, which asks for every result.
func countFlag(fs *flag.FlagSet, def int) *int {
	n := def
	fs.Func("n", "", func(v string) error {
		u, err := strconv.ParseUint(v, 10, 64)
		if err != nil && !errors.Is(err, strconv.ErrRange) {
			return errors.New("-n needs a count of 0 or more")
		}
		n = int(min(u, math.MaxInt))
		return nil
	})
	return &n
}

// jsonFlag adds the --json flag to fs.
func jsonFlag(fs *flag.FlagSet) *bool { return fs.Bool("json", false, "print JSON") }

// parse parses args with fs and accepts flags and positional words in any
// order, so gm search words -n 3 works. The flag package stops at the first
// positional word, so parse moves each flag, with the value of a non-boolean
// flag, to the front. Everything after -- stays positional.
func parse(fs *flag.FlagSet, args []string) error {
	var fl, rest []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			rest = append(rest, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(a, "-") || a == "-" {
			rest = append(rest, a)
			continue
		}
		fl = append(fl, a)
		name := strings.TrimLeft(a, "-")
		if f := fs.Lookup(name); f != nil && !isBool(f) && i+1 < len(args) {
			fl = append(fl, args[i+1])
			i++
		}
	}
	return fs.Parse(append(append(fl, "--"), rest...))
}

// isBool reports whether f takes no value, the way the flag package decides it.
func isBool(f *flag.Flag) bool {
	b, ok := f.Value.(interface{ IsBoolFlag() bool })
	return ok && b.IsBoolFlag()
}

// emit writes v to w as indented JSON.
func emit(w io.Writer, v any) {
	e := json.NewEncoder(w)
	e.SetIndent("", "  ")
	_ = e.Encode(v)
}
