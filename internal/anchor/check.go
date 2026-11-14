package anchor

import (
	"fmt"
	"strings"

	"github.com/vabatta/gm/internal/git"
)

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
