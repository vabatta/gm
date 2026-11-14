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

	"github.com/vabatta/gm/internal/git"
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
	commitRe = regexp.MustCompile(`^[0-9a-f]{7,40}$`)   // an abbreviated or full commit id
	rangeRe  = regexp.MustCompile(`:[0-9]+(-[0-9]+)?$`) // a final :N or :N-M
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
// first and last line.
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

// Pin turns spec, written path[:from[-to]] relative to the repository root,
// into an anchor at HEAD, with the commit id cut to 12 hex characters.
//
// When spec names a file at HEAD, Pin takes it whole, so a path that contains
// ":" needs no escaping. Otherwise a final ":N" or ":N-M" is the line range.
// Pin refuses a file that is not committed at HEAD or that has uncommitted
// changes, because the anchor must show the code that the note describes. It
// also refuses an empty path and a range past the end of the file.
func Pin(r *git.Repo, spec string) (Anchor, error) {
	head, err := r.Run("rev-parse", "--short=12", "HEAD")
	if err != nil {
		return Anchor{}, errors.New("anchors need a commit: " + err.Error())
	}
	a := Anchor{Commit: head, Path: strings.TrimPrefix(spec, "./")}
	if strings.Trim(a.Path, "/.") == "" {
		return Anchor{}, fmt.Errorf("at %q: an anchor needs a path from the repository root", spec)
	}
	if _, err := r.Run("cat-file", "-e", "HEAD:"+a.Path); err != nil {
		if loc := rangeRe.FindStringIndex(a.Path); loc != nil {
			if a.From, a.To, err = Lines(a.Path[loc[0]+1:]); err != nil {
				return Anchor{}, fmt.Errorf("at %s: %v", spec, err)
			}
			a.Path = a.Path[:loc[0]]
		}
		if _, err := r.Run("cat-file", "-e", "HEAD:"+a.Path); err != nil {
			return Anchor{}, fmt.Errorf("at %s: the file is not committed at HEAD", spec)
		}
	}
	if dirty, _ := r.Run("status", "--porcelain", "--", a.Path); dirty != "" {
		return Anchor{}, fmt.Errorf("at %s: the file has uncommitted changes; commit the code first, then save the note", spec)
	}
	if a.From > 0 {
		lines, _ := Code(r, a)
		if a.To > len(lines) {
			return Anchor{}, fmt.Errorf("at %s: the file has %d lines", spec, len(lines))
		}
	}
	return a, nil
}

// Code returns the file's lines at the anchor's commit, read with git cat-file.
// For a line range it returns lines 1 to To, so index i holds line i+1 and the
// caller drops the lines before From. For a whole file it returns every line.
func Code(r *git.Repo, a Anchor) ([]string, error) {
	content, err := r.Run("cat-file", "-p", a.Commit+":"+a.Path)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(content, "\n")
	if a.From > 0 && a.To <= len(lines) {
		lines = lines[:a.To]
	}
	return lines, nil
}

// Check asks git whether each anchor in specs still points at code, and returns
// a problem for each broken anchor, keyed by the anchor as written in the note.
// An anchor is broken when its commit is missing, when its commit is not in the
// history of HEAD (a squash or a rebase replaced it), or when its file is gone
// at HEAD. When git sees the file as renamed since the commit, the problem
// names the new path.
//
// The number of git processes does not grow with the number of anchors. One
// cat-file --batch-check looks up every commit and every HEAD:path. One
// rev-list --stdin lists the anchor commits that HEAD does not contain. One
// rev-parse tells whether the clone is shallow. Rename detection runs only for
// missing files, once per commit.
//
// A shallow clone does not have the old commits, so there a missing commit is
// not a problem. unseen counts those anchors, and Check still checks their
// files at HEAD.
func Check(r *git.Repo, specs []string) (problems map[string]string, unseen int) {
	problems = map[string]string{}
	var as []Anchor
	var written, objects []string
	seen := map[string]bool{}
	ask := func(obj string) {
		if !seen[obj] {
			seen[obj] = true
			objects = append(objects, obj)
		}
	}
	for _, s := range specs {
		a, err := Parse(s)
		if err != nil {
			problems[s] = err.Error()
			continue
		}
		as, written = append(as, a), append(written, s)
		ask(a.Commit + "^{commit}")
		ask("HEAD:" + a.Path)
	}
	if len(as) == 0 {
		return problems, 0
	}
	out, err := r.Input(strings.Join(objects, "\n")+"\n", "cat-file", "--batch-check")
	lines := strings.Split(out, "\n")
	if err != nil || len(lines) != len(objects) {
		for i := range as {
			problems[written[i]] = "git could not check anchors: " + fmt.Sprint(err)
		}
		return problems, 0
	}
	full := map[string]string{} // object asked for -> its full id; no entry when git has no such object
	for i, obj := range objects {
		if f := strings.Fields(lines[i]); len(f) == 3 {
			full[obj] = f[0]
		}
	}
	var stdin strings.Builder
	for _, a := range as {
		if id := full[a.Commit+"^{commit}"]; id != "" {
			stdin.WriteString(id + "\n")
		}
	}
	stdin.WriteString("^HEAD\n")
	unreached := map[string]bool{} // anchor commits that HEAD does not contain
	out, err = r.Input(stdin.String(), "rev-list", "--stdin")
	for _, id := range strings.Fields(out) {
		unreached[id] = true
	}
	shallow, _ := r.Run("rev-parse", "--is-shallow-repository")
	renames := map[string]map[string]string{}
	for i, a := range as {
		s := written[i]
		id := full[a.Commit+"^{commit}"]
		switch {
		case id == "" && shallow == "true":
			unseen++
			if full["HEAD:"+a.Path] == "" {
				problems[s] = "the file no longer exists at HEAD"
			}
		case id == "":
			problems[s] = "commit not found (a squash, a rebase or a shallow clone dropped it)"
		case err != nil || unreached[id]:
			problems[s] = "commit is not in the history of HEAD"
		case full["HEAD:"+a.Path] != "":
		default:
			if renames[a.Commit] == nil {
				renames[a.Commit] = renamed(r, a.Commit)
			}
			if to := renames[a.Commit][a.Path]; to != "" {
				problems[s] = "the file moved to " + to + "; save a revision anchored to the new path"
			} else {
				problems[s] = "the file no longer exists at HEAD"
			}
		}
	}
	return problems, unseen
}

// renamed maps old paths to new paths for the files that git diff -M finds
// renamed between commit and HEAD. Copies are skipped, because after a copy the
// original file still exists.
func renamed(r *git.Repo, commit string) map[string]string {
	out, _ := r.Run("diff", "-M", "--name-status", "-z", commit, "HEAD")
	m := map[string]string{}
	f := strings.Split(out, "\x00")
	for i := 0; i < len(f); i++ {
		if strings.HasPrefix(f[i], "R") || strings.HasPrefix(f[i], "C") {
			if i+2 < len(f) && f[i][0] == 'R' {
				m[f[i+1]] = f[i+2]
			}
			i += 2
		} else {
			i++
		}
	}
	return m
}

// Changed counts the commits after the anchor's commit that touched the
// anchored code: git log -L for a line range, git log -- path for a whole file.
// git log -L reads the line numbers in the file at HEAD, not at the anchor's
// commit. After lines are added or removed above the range, it follows other
// lines.
func Changed(r *git.Repo, a Anchor) int {
	var out string
	if a.From > 0 {
		out, _ = r.Run("log", "--format=%h", "--no-patch", fmt.Sprintf("-L%d,%d:%s", a.From, a.To, a.Path), a.Commit+"..HEAD")
	} else {
		out, _ = r.Run("log", "--format=%h", a.Commit+"..HEAD", "--", a.Path)
	}
	return len(strings.Fields(out))
}
