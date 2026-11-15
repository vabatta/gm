// Package anchor ties a note to the code it describes. An anchor names a file,
// and optionally a line range, at one commit. A commit never changes, so gm can
// always read the code that the note was written against. git can also tell
// whether that commit is still in the branch history and whether the code
// changed after it.
package anchor

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Anchor is a place in the code at one commit, written commit:path[:from[-to]].
// Path is relative to the repository root. From and To are the first and last
// line, counted from 1. Both are 0 when the anchor covers the whole file.
//
// A path that ends in ":", ":N" or ":N-M" would read back as a line range. For
// such a path String adds one more ":" at the end, and Parse removes it.
type Anchor struct {
	Commit, Path string
	From, To     int
}

var (
	commitRe = regexp.MustCompile(`^[0-9a-f]{7,40}$`)
	rangeRe  = regexp.MustCompile(`:[0-9]+(-[0-9]+)?$`)
)

// String returns the anchor in the form Parse reads.
func (a Anchor) String() string {
	s := a.Commit + ":" + a.Path
	switch {
	case a.From > 0 && a.To != a.From:
		s += fmt.Sprintf(":%d-%d", a.From, a.To)
	case a.From > 0:
		s += ":" + strconv.Itoa(a.From)
	case strings.HasSuffix(a.Path, ":") || rangeRe.MatchString(a.Path):
		s += ":"
	}
	return s
}

// Parse reads an anchor in the form String writes. The commit ends at the first
// ":", so the path may contain ":". A final ":N" or ":N-M" is the line range.
func Parse(s string) (Anchor, error) {
	commit, rest, ok := strings.Cut(s, ":")
	if !ok || rest == "" || !commitRe.MatchString(commit) {
		return Anchor{}, errors.New("anchor must be commit:path[:lines]")
	}
	a := Anchor{Commit: commit, Path: rest}
	if strings.HasSuffix(rest, ":") {
		a.Path = rest[:len(rest)-1]
		return a, nil
	}
	if loc := rangeRe.FindStringIndex(rest); loc != nil {
		from, to, err := Lines(rest[loc[0]+1:])
		if err != nil {
			return Anchor{}, err
		}
		a.Path, a.From, a.To = rest[:loc[0]], from, to
	}
	return a, nil
}

// Lines reads a line range written N or N-M, with 1 <= N <= M, and returns the
// first and last line. It returns an error for any other form.
func Lines(s string) (int, int, error) {
	a, b, isRange := strings.Cut(s, "-")
	from, err := strconv.Atoi(a)
	to := from
	if err == nil && isRange {
		to, err = strconv.Atoi(b)
	}
	if err != nil || from < 1 || to < from {
		return 0, 0, errors.New("lines must be N or N-M with 1 <= N <= M")
	}
	return from, to, nil
}
