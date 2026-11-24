package anchor

import (
	"errors"
	"fmt"
	"strings"

	"github.com/vabatta/gm/internal/git"
)

// Pin turns spec, written path[:from[-to]] relative to the repository root,
// into an anchor at HEAD, with the commit id cut to 12 hex characters.
//
// When spec names a file at HEAD, Pin takes it whole, so a path that contains
// ":" needs no escaping. The anchor holds the path as git spells it at HEAD. Otherwise a final ":N" or ":N-M" is the line range.
// Pin refuses a file that is not committed at HEAD or that has uncommitted
// changes, because the anchor must show the code that the note describes. It
// also refuses an empty path, a path with a line break, which the one-line
// header cannot hold, a directory or submodule, and a range past the end of
// the file.
func Pin(r *git.Repo, spec string) (Anchor, error) {
	head, err := r.Run("rev-parse", "--short=12", "HEAD")
	if err != nil {
		return Anchor{}, errors.New("anchors need a commit: " + err.Error())
	}
	a := Anchor{Commit: head, Path: strings.TrimPrefix(spec, "./")}
	if strings.Trim(a.Path, "/.") == "" {
		return Anchor{}, fmt.Errorf("at %q: an anchor needs a path from the repository root", spec)
	}
	if strings.ContainsAny(a.Path, "\r\n") {
		return Anchor{}, fmt.Errorf("at %q: a path with a line break cannot be an anchor", spec)
	}
	typ, err := r.Run("cat-file", "-t", "HEAD:"+a.Path)
	if err != nil {
		if loc := rangeRe.FindStringIndex(a.Path); loc != nil {
			if a.From, a.To, err = Lines(a.Path[loc[0]+1:]); err != nil {
				return Anchor{}, fmt.Errorf("at %s: %v", spec, err)
			}
			a.Path = a.Path[:loc[0]]
		}
		if typ, err = r.Run("cat-file", "-t", "HEAD:"+a.Path); err != nil {
			// A submodule is a tree entry whose commit lives in the
			// submodule's own object store, so cat-file cannot read it, yet
			// rev-parse resolves the entry from the tree alone.
			if _, err := r.Run("rev-parse", "-q", "--verify", "HEAD:"+a.Path); err != nil {
				return Anchor{}, fmt.Errorf("at %s: the file is not committed at HEAD", spec)
			}
			typ = "commit"
		}
	}
	if typ != "blob" {
		return Anchor{}, fmt.Errorf("at %s: the path names a directory, not a file", spec)
	}
	// git may spell the path in another Unicode form than spec: on macOS,
	// core.precomposeUnicode turns an NFD name into the NFC name the tree
	// holds. Later lookups, such as cat-file --batch-check on stdin, are not
	// precomposed, so the anchor keeps the tree's spelling.
	if name, err := r.Run("ls-tree", "-z", "--name-only", "HEAD", "--", a.Path); err == nil {
		if name, ok := strings.CutSuffix(name, "\x00"); ok && name != "" && !strings.Contains(name, "\x00") {
			a.Path = name
		}
	}
	if !r.AtHEAD([]string{a.Path})[a.Path] {
		return Anchor{}, fmt.Errorf("at %s: the file has uncommitted changes; commit the code first, then save the note", spec)
	}
	if a.From > 0 {
		lines, _ := Code(r, a.Commit, a)
		if a.To > len(lines) {
			return Anchor{}, fmt.Errorf("at %s: the file has %d lines", spec, len(lines))
		}
	}
	return a, nil
}

// Code returns the anchored file's lines at commit base, read with git cat-file.
// For a line range it returns lines 1 to To, so index i holds line i+1 and the
// caller drops the lines before From. For a whole file it returns every line.
func Code(r *git.Repo, base string, a Anchor) ([]string, error) {
	// Raw keeps blank lines at the end of the file, which Run would trim. Only
	// the newline that ends the last line is not a line of its own.
	content, err := r.Raw("", "cat-file", "-p", base+":"+a.Path)
	if err != nil {
		return nil, err
	}
	var lines []string
	if content != "" {
		lines = strings.Split(strings.TrimSuffix(content, "\n"), "\n")
	}
	if a.From > 0 && a.To <= len(lines) {
		lines = lines[:a.To]
	}
	return lines, nil
}
