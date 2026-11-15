package anchor

import (
	"fmt"
	"strings"

	"github.com/vabatta/gm/internal/git"
)

// Target is an anchor as a note writes it, with Added, the commit that added
// the note. Added is empty for a note that is not committed yet.
type Target struct{ Spec, Added string }

// Resolved is an anchor with the commit it is read at.
type Resolved struct {
	Spec   string
	Anchor Anchor
	// Base is the anchor's own commit while it is in the history of HEAD.
	// Otherwise it is the commit that added the note, which a squash or rebase
	// merge leaves in its place, or else HEAD.
	Base string
	// Fallback is true when Base is not the anchor's own commit.
	Fallback bool
	// Unseen is true in a shallow clone that lacks the anchor's commit. Resolve
	// then checks only the file, because the clone cannot tell what changed.
	Unseen bool
	// Problem says why the anchor no longer points at code, or is empty.
	Problem string
}

// Resolve picks the base of each target and checks that its file still exists
// at HEAD, naming the new path when git sees the file as renamed since the
// base. The number of git processes does not grow with the number of
// anchors. One cat-file --batch-check looks up every anchor commit and every
// HEAD:path. One rev-list --stdin lists the anchor commits that HEAD does not
// contain. One rev-parse tells whether the clone is shallow. Resolve runs
// rename detection only for missing files, once per base.
func Resolve(r *git.Repo, targets []Target) []Resolved {
	res := make([]Resolved, len(targets))
	var objects []string
	seen := map[string]bool{}
	ask := func(obj string) {
		if !seen[obj] {
			seen[obj] = true
			objects = append(objects, obj)
		}
	}
	for i, t := range targets {
		a, err := Parse(t.Spec)
		res[i] = Resolved{Spec: t.Spec, Anchor: a}
		if err != nil {
			res[i].Problem = err.Error()
			continue
		}
		ask(a.Commit + "^{commit}")
		ask("HEAD:" + a.Path)
	}
	if len(objects) == 0 {
		return res
	}
	out, err := r.Input(strings.Join(objects, "\n")+"\n", "cat-file", "--batch-check")
	lines := strings.Split(out, "\n")
	if err != nil || len(lines) != len(objects) {
		for i := range res {
			res[i].Problem = "git could not check anchors: " + fmt.Sprint(err)
		}
		return res
	}
	full := map[string]string{} // object asked for -> its full id; no entry when git has no such object
	for i, obj := range objects {
		// A hit is "<id> <type> <size>". A miss is "<object> missing", and an
		// object name with a space in its path also splits into three fields,
		// so the type is what tells them apart.
		if f := strings.Fields(lines[i]); len(f) == 3 && objectTypes[f[1]] {
			full[obj] = f[0]
		}
	}
	var stdin strings.Builder
	for _, obj := range objects {
		if id := full[obj]; id != "" && strings.HasSuffix(obj, "^{commit}") {
			stdin.WriteString(id + "\n")
		}
	}
	stdin.WriteString("^HEAD\n")
	unreached := map[string]bool{}
	out, err = r.Input(stdin.String(), "rev-list", "--stdin")
	for _, id := range strings.Fields(out) {
		unreached[id] = true
	}
	shallow, _ := r.Run("rev-parse", "--is-shallow-repository")
	renames := map[string]map[string]string{}
	for i, t := range targets {
		x := &res[i]
		if x.Problem != "" {
			continue
		}
		id := full[x.Anchor.Commit+"^{commit}"]
		switch {
		case id != "" && err == nil && !unreached[id]:
			x.Base = id
		case id == "" && shallow == "true":
			x.Base, x.Unseen = "HEAD", true
		case t.Added != "":
			x.Base, x.Fallback = t.Added, true
		default:
			x.Base, x.Fallback = "HEAD", true
		}
		if full["HEAD:"+x.Anchor.Path] != "" {
			continue
		}
		if renames[x.Base] == nil && !x.Unseen {
			renames[x.Base] = renamed(r, x.Base)
		}
		if to := renames[x.Base][x.Anchor.Path]; to != "" {
			x.Problem = "the file moved to " + to + "; save a revision anchored to the new path"
		} else {
			x.Problem = "the file no longer exists at HEAD"
		}
	}
	return res
}

// objectTypes are the object types git cat-file --batch-check prints for a hit.
var objectTypes = map[string]bool{"commit": true, "tree": true, "blob": true, "tag": true}

// renamed maps old paths to new paths for the files that git diff -M finds
// renamed between commit and HEAD. It skips copies, because after a copy the
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
