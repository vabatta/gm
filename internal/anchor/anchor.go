// Package anchor ties a note to the code it is about: a path, optionally a
// line range, at one commit. The commit never changes, so the anchored code
// can be read at any time, and git can tell whether it still exists and
// whether it changed since.
package anchor

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/vabatta/gm/internal/git"
)

// Anchor is a fixed place in the code, written commit:path[:from[-to]].
// A path that itself ends in ":" or in something shaped like a line range is
// written with one extra trailing ":" so it reads back unchanged.
type Anchor struct {
	Commit, Path string
	From, To     int // 0 when the anchor is the whole file
}

var (
	commitRe = regexp.MustCompile(`^[0-9a-f]{7,40}$`)
	rangeRe  = regexp.MustCompile(`:[0-9]+(-[0-9]+)?$`)
)

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

// Parse reads commit:path[:from[-to]].
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

// Lines reads N or N-M.
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

// Pin turns path[:from[-to]], relative to the repository root, into an anchor at HEAD.
// It refuses a file with uncommitted changes, so the anchor shows the code the note describes.
// A spec that names an existing file is taken whole, so paths holding ":" need no escaping.
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

// Code returns the anchored lines at the anchor's commit, numbered from 1; a whole-file anchor returns every line.
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

// Check asks git, in a few batched calls, whether each anchor still points at
// code: its commit must be in the history of HEAD and its file must exist at
// HEAD. It returns a problem for each broken anchor, keyed by the anchor as written.
func Check(r *git.Repo, specs []string) map[string]string {
	problems := map[string]string{}
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
		return problems
	}
	out, err := r.Input(strings.Join(objects, "\n")+"\n", "cat-file", "--batch-check")
	lines := strings.Split(out, "\n")
	if err != nil || len(lines) != len(objects) {
		for i := range as {
			problems[written[i]] = "git could not check anchors: " + fmt.Sprint(err)
		}
		return problems
	}
	full := map[string]string{} // object asked -> full id, absent when missing
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
	unreached := map[string]bool{} // commits the anchors name that HEAD does not contain
	out, err = r.Input(stdin.String(), "rev-list", "--stdin")
	for _, id := range strings.Fields(out) {
		unreached[id] = true
	}
	renames := map[string]map[string]string{}
	for i, a := range as {
		s := written[i]
		id := full[a.Commit+"^{commit}"]
		switch {
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
	return problems
}

// renamed maps old paths to new ones for files git sees as renamed between commit and HEAD.
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

// Changed counts the commits after the anchor's that touched its lines, or its file when it has no lines.
func Changed(r *git.Repo, a Anchor) int {
	var out string
	if a.From > 0 {
		out, _ = r.Run("log", "--format=%h", "--no-patch", fmt.Sprintf("-L%d,%d:%s", a.From, a.To, a.Path), a.Commit+"..HEAD")
	} else {
		out, _ = r.Run("log", "--format=%h", a.Commit+"..HEAD", "--", a.Path)
	}
	return len(strings.Fields(out))
}
