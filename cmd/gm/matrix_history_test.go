package main

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// The base kinds of gm(1) ANCHORS that a history shape leaves for its note.
const (
	baseOwn     = iota // the anchor's own commit, in the history of HEAD
	baseAdded          // the commit that added the note
	baseShallow        // HEAD, because a shallow clone lacks the anchor's commit
)

var basePhrase = map[int]string{
	baseOwn:     "the anchor's own commit",
	baseAdded:   "the commit that added the note",
	baseShallow: "HEAD because the shallow clone lacks the anchor's commit",
}

// histScenario is the state a history shape leaves: the repository to read
// in, the note under test, which is anchored to line 3 of file, and what the
// manual says gm reads for it.
type histScenario struct {
	r      *repo
	id     string
	file   string // the anchored file; the anchor is file:3
	line   string // line 3 of file at the base
	anchor string // the anchor's commit, 12 hex characters
	added  string // the commit that added the note, for baseAdded
	// onBranch is true when the note is not on the default branch, so the
	// branch rule of LINT applies to it.
	onBranch bool
	// noDefault is true when none of origin/HEAD, origin/main, origin/master,
	// main and master exists.
	noDefault bool
	// unseen is the number of anchor commits a shallow clone lacks.
	unseen int
	// deletedIn is the commit that deleted the note file, when the shape
	// itself removes the note from the working tree.
	deletedIn string
}

// histPrep runs on the repository and branch that a shape reads, before any
// clone the shape makes, so a revision and prune done in it travel the way
// work pushed upstream does.
type histPrep func(r *repo, id string)

type histShape struct {
	name string
	base int
	// lostHistory is true when the read repository lacks the history that
	// holds notes pruned upstream (a depth 1 clone).
	lostHistory bool
	// noCommits marks the repository with no commits, where no anchor can be
	// pinned.
	noCommits bool
	build     func(t *testing.T, prep histPrep) histScenario
}

const histRetry = "func Fetch() { retry() }"

// setLine3 writes a three-line Go file whose third line is line.
func setLine3(r *repo, file, line string) { r.write(file, "package x\n\n"+line+"\n") }

// anchoredFact saves a fact anchored to line 3 of file at HEAD and returns
// its id and the anchor's commit.
func anchoredFact(r *repo, title, file string) (string, string) {
	r.t.Helper()
	return r.fact(title, "--at", file+":3"), r.head()
}

// feature starts branch feat at HEAD, commits fetch.go with line 3 changed,
// then saves a note anchored to that line and commits it. It stays on feat.
func feature(r *repo) (id, anchor string) {
	r.t.Helper()
	r.git("checkout", "-qb", "feat")
	setLine3(r, "fetch.go", histRetry)
	r.commit("retry")
	id, anchor = anchoredFact(r, "Fetch retries", "fetch.go")
	r.commit("note")
	return id, anchor
}

// moves checks out branch and commits other.go on it, a file with three lines.
func moves(r *repo, branch string) {
	r.t.Helper()
	r.git("checkout", "-q", branch)
	setLine3(r, "other.go", "func Other() {}")
	r.commit(branch + " moves")
}

func onDisk(r *repo, id string) bool {
	return slices.ContainsFunc(r.notes(), func(f string) bool { return strings.HasPrefix(filepath.Base(f), id+"-") })
}

func runPrep(prep histPrep, r *repo, id string) {
	if prep != nil {
		prep(r, id)
	}
}

// localShapes are the shapes made in one repository, read where they end.
func localShapes() []histShape {
	return []histShape{
		{name: "merge commit (--no-ff)", base: baseOwn, build: func(t *testing.T, prep histPrep) histScenario {
			r := newRepo(t)
			id, a := feature(r)
			moves(r, "main")
			r.git("merge", "-q", "--no-ff", "--no-edit", "feat")
			runPrep(prep, r, id)
			return histScenario{r: r, id: id, file: "fetch.go", line: histRetry, anchor: a}
		}},
		{name: "fast-forward", base: baseOwn, build: func(t *testing.T, prep histPrep) histScenario {
			r := newRepo(t)
			id, a := feature(r)
			r.git("checkout", "-q", "main")
			r.git("merge", "-q", "--ff-only", "feat")
			runPrep(prep, r, id)
			return histScenario{r: r, id: id, file: "fetch.go", line: histRetry, anchor: a}
		}},
		{name: "squash merge, branch deleted", base: baseAdded, build: func(t *testing.T, prep histPrep) histScenario {
			r := newRepo(t)
			id, a := feature(r)
			moves(r, "main")
			r.git("merge", "-q", "--squash", "feat")
			r.commit("squash feat")
			added := r.head()
			r.git("branch", "-qD", "feat")
			runPrep(prep, r, id)
			return histScenario{r: r, id: id, file: "fetch.go", line: histRetry, anchor: a, added: added}
		}},
		{name: "squash merge, then notes moved to another gm.dir", base: baseAdded, build: func(t *testing.T, prep histPrep) histScenario {
			r := newRepo(t)
			id, a := feature(r)
			moves(r, "main")
			r.git("merge", "-q", "--squash", "feat")
			r.commit("squash feat")
			added := r.head()
			r.git("branch", "-qD", "feat")
			runPrep(prep, r, id)
			move(r, ".agents/memory", "docs/memory")
			return histScenario{r: r, id: id, file: "fetch.go", line: histRetry, anchor: a, added: added}
		}},
		{name: "rebase then fast-forward, branch deleted", base: baseAdded, build: func(t *testing.T, prep histPrep) histScenario {
			r := newRepo(t)
			id, a := feature(r)
			moves(r, "main")
			r.git("checkout", "-q", "feat")
			r.git("rebase", "-q", "main")
			added := r.head()
			r.git("checkout", "-q", "main")
			r.git("merge", "-q", "--ff-only", "feat")
			r.git("branch", "-qD", "feat")
			runPrep(prep, r, id)
			return histScenario{r: r, id: id, file: "fetch.go", line: histRetry, anchor: a, added: added}
		}},
		{name: "cherry-pick of the code and note commits", base: baseAdded, build: func(t *testing.T, prep histPrep) histScenario {
			r := newRepo(t)
			id, a := feature(r)
			moves(r, "main")
			r.git("cherry-pick", a, "feat")
			added := r.head()
			runPrep(prep, r, id)
			return histScenario{r: r, id: id, file: "fetch.go", line: histRetry, anchor: a, added: added}
		}},
		{name: "note committed together with the code", base: baseAdded, build: func(t *testing.T, prep histPrep) histScenario {
			r := newRepo(t)
			r.git("checkout", "-qb", "feat")
			setLine3(r, "fetch.go", histRetry)
			r.commit("retry")
			id, a := anchoredFact(r, "Fetch retries", "fetch.go")
			r.git("add", "-A")
			r.git("commit", "-q", "--amend", "--no-edit")
			added := r.head()
			runPrep(prep, r, id)
			return histScenario{r: r, id: id, file: "fetch.go", line: histRetry, anchor: a, added: added, onBranch: true}
		}},
		{name: "note committed in a later commit than the code", base: baseOwn, build: func(t *testing.T, prep histPrep) histScenario {
			r := newRepo(t)
			r.git("checkout", "-qb", "feat")
			setLine3(r, "fetch.go", histRetry)
			r.commit("retry")
			id, a := anchoredFact(r, "Fetch retries", "fetch.go")
			r.write("docs.txt", "docs\n")
			r.git("add", "docs.txt")
			r.git("commit", "-qm", "docs")
			r.commit("note")
			runPrep(prep, r, id)
			return histScenario{r: r, id: id, file: "fetch.go", line: histRetry, anchor: a, onBranch: true}
		}},
		{name: "code amended after the note was saved, before the note commit", base: baseAdded, build: func(t *testing.T, prep histPrep) histScenario {
			r := newRepo(t)
			r.git("checkout", "-qb", "feat")
			setLine3(r, "fetch.go", histRetry)
			r.commit("retry")
			id, a := anchoredFact(r, "Fetch retries", "fetch.go")
			setLine3(r, "fetch.go", "func Fetch() { retry(3) }")
			r.git("add", "fetch.go")
			r.git("commit", "-q", "--amend", "--no-edit")
			r.commit("note")
			added := r.head()
			runPrep(prep, r, id)
			return histScenario{r: r, id: id, file: "fetch.go", line: "func Fetch() { retry(3) }", anchor: a, added: added, onBranch: true}
		}},
		{name: "several notes in one commit, squash merged", base: baseAdded, build: func(t *testing.T, prep histPrep) histScenario {
			r := newRepo(t)
			r.git("checkout", "-qb", "feat")
			setLine3(r, "fetch.go", histRetry)
			r.commit("retry")
			id, a := anchoredFact(r, "Fetch retries", "fetch.go")
			r.fact("Fetch is in package x", "--at", "fetch.go:1")
			r.fact("Fetch has its own file", "--at", "fetch.go")
			r.commit("notes")
			moves(r, "main")
			r.git("merge", "-q", "--squash", "feat")
			r.commit("squash feat")
			added := r.head()
			r.git("branch", "-qD", "feat")
			runPrep(prep, r, id)
			return histScenario{r: r, id: id, file: "fetch.go", line: histRetry, anchor: a, added: added}
		}},
		{name: "note saved directly on main", base: baseOwn, build: func(t *testing.T, prep histPrep) histScenario {
			r := newRepo(t)
			setLine3(r, "fetch.go", histRetry)
			r.commit("retry")
			id, a := anchoredFact(r, "Fetch retries", "fetch.go")
			r.commit("note")
			runPrep(prep, r, id)
			return histScenario{r: r, id: id, file: "fetch.go", line: histRetry, anchor: a}
		}},
		{name: "branch from a branch, both merged", base: baseOwn, build: func(t *testing.T, prep histPrep) histScenario {
			r := newRepo(t)
			feature(r)
			r.git("checkout", "-qb", "feat2")
			line := "func Fetch() { retry(); log() }"
			setLine3(r, "fetch.go", line)
			r.commit("log")
			id, a := anchoredFact(r, "Fetch logs", "fetch.go")
			r.commit("note 2")
			moves(r, "main")
			r.git("merge", "-q", "--no-ff", "--no-edit", "feat")
			r.git("merge", "-q", "--no-ff", "--no-edit", "feat2")
			runPrep(prep, r, id)
			return histScenario{r: r, id: id, file: "fetch.go", line: line, anchor: a}
		}},
		{name: "octopus merge", base: baseOwn, build: func(t *testing.T, prep histPrep) histScenario {
			r := newRepo(t)
			id, a := feature(r)
			for _, b := range []string{"b", "c"} {
				r.git("checkout", "-qb", b, "main")
				r.write(b+".go", "package x\n")
				r.commit(b)
				r.fact("Branch "+b+" file", "--at", b+".go")
				r.commit("note " + b)
			}
			r.git("checkout", "-q", "main")
			r.git("merge", "-q", "--no-edit", "feat", "b", "c")
			if n := len(strings.Fields(r.git("rev-list", "--parents", "-1", "HEAD"))); n != 4 {
				t.Fatalf("want an octopus merge with 3 parents, got %d fields", n)
			}
			runPrep(prep, r, id)
			return histScenario{r: r, id: id, file: "fetch.go", line: histRetry, anchor: a}
		}},
		{name: "note saved while resolving a merge (merge --no-commit)", base: baseOwn, build: func(t *testing.T, prep histPrep) histScenario {
			r := newRepo(t)
			r.git("checkout", "-qb", "feat")
			setLine3(r, "fetch.go", histRetry)
			r.commit("retry")
			moves(r, "main")
			r.git("merge", "-q", "--no-ff", "--no-commit", "feat")
			// The merge changed fetch.go against HEAD, so save refuses it.
			refused(t, r.gm("save", "fact", "--title", "t", "--keywords", "a b c", "--fact", "f", "--evidence", "e", "--at", "fetch.go:3"), "uncommitted changes")
			id, a := anchoredFact(r, "Other is empty", "other.go")
			r.commit("merge feat with a note")
			runPrep(prep, r, id)
			return histScenario{r: r, id: id, file: "other.go", line: "func Other() {}", anchor: a}
		}},
		{name: "note pruned inside a merge commit", base: baseOwn, build: func(t *testing.T, prep histPrep) histScenario {
			r := newRepo(t)
			id, a := feature(r)
			r.git("checkout", "-q", "main")
			r.git("merge", "-q", "--no-ff", "--no-edit", "feat")
			r.git("checkout", "-qb", "feat2")
			r.write("x.go", "package x\n")
			r.commit("feat2")
			r.git("checkout", "-q", "main")
			r.write("y.go", "package x\n")
			r.commit("main")
			r.git("merge", "-q", "--no-ff", "--no-commit", "feat2")
			r.fact("Fetch retries, revised in the merge", "--supersedes", id)
			has(t, r.ok("prune"), "pruned .agents/memory/"+id+"-")
			r.commit("merge feat2, revise and prune")
			deleted := r.head()
			runPrep(prep, r, id)
			return histScenario{r: r, id: id, file: "fetch.go", line: histRetry, anchor: a, deletedIn: deleted}
		}},
		{name: "revert of the note commit", base: baseOwn, build: func(t *testing.T, prep histPrep) histScenario {
			r := newRepo(t)
			id, a := feature(r)
			moves(r, "main")
			r.git("merge", "-q", "--no-ff", "--no-edit", "feat")
			r.git("revert", "--no-edit", "feat")
			deleted := r.head()
			runPrep(prep, r, id)
			return histScenario{r: r, id: id, file: "fetch.go", line: histRetry, anchor: a, deletedIn: deleted}
		}},
	}
}

// fresh wraps a local shape: the same history, read in a fresh clone over
// file://, on the branch the shape ends on. The clone holds only what its
// branches reach.
func fresh(s histShape) histShape {
	inner := s.build
	s.name = "fresh file:// clone after " + s.name
	s.build = func(t *testing.T, prep histPrep) histScenario {
		sc := inner(t, prep)
		branch := sc.r.git("rev-parse", "--abbrev-ref", "HEAD")
		// The source stays on main, as a hosted repository does, so the
		// clone's origin/HEAD names origin/main.
		sc.r.git("checkout", "-q", "main")
		c := &repo{t, t.TempDir()}
		c.git("clone", "-q", "-b", branch, "file://"+sc.r.dir, ".")
		sc.r = c
		return sc
	}
	return s
}

// directOnMain is the source repository of the shallow clones: code, then a
// note anchored to it, both on main.
func directOnMain(t *testing.T, prep histPrep) (*repo, string, string) {
	r := newRepo(t)
	setLine3(r, "fetch.go", histRetry)
	r.commit("retry")
	id, a := anchoredFact(r, "Fetch retries", "fetch.go")
	r.commit("note")
	runPrep(prep, r, id)
	return r, id, a
}

func otherShapes() []histShape {
	return []histShape{
		{name: "shallow clone depth 1", base: baseShallow, lostHistory: true, build: func(t *testing.T, prep histPrep) histScenario {
			src, id, a := directOnMain(t, prep)
			c := &repo{t, t.TempDir()}
			c.git("clone", "-q", "--depth", "1", "file://"+src.dir, ".")
			return histScenario{r: c, id: id, file: "fetch.go", line: histRetry, anchor: a, unseen: 1}
		}},
		{name: "shallow clone depth 2", base: baseOwn, build: func(t *testing.T, prep histPrep) histScenario {
			src, id, a := directOnMain(t, prep)
			c := &repo{t, t.TempDir()}
			c.git("clone", "-q", "--depth", "2", "file://"+src.dir, ".")
			return histScenario{r: c, id: id, file: "fetch.go", line: histRetry, anchor: a}
		}},
		{name: "partial clone --filter=blob:none after a squash merge", base: baseAdded, build: func(t *testing.T, prep histPrep) histScenario {
			r := newRepo(t)
			r.git("config", "uploadpack.allowFilter", "true")
			id, a := feature(r)
			moves(r, "main")
			r.git("merge", "-q", "--squash", "feat")
			r.commit("squash feat")
			added := r.head()
			r.git("branch", "-qD", "feat")
			runPrep(prep, r, id)
			c := &repo{t, t.TempDir()}
			c.git("clone", "-q", "--filter=blob:none", "file://"+r.dir, ".")
			if c.git("config", "remote.origin.promisor") != "true" {
				t.Fatal("want a partial clone")
			}
			return histScenario{r: c, id: id, file: "fetch.go", line: histRetry, anchor: a, added: added}
		}},
		{name: "single-branch clone of an unmerged branch", base: baseOwn, build: func(t *testing.T, prep histPrep) histScenario {
			r := newRepo(t)
			id, a := feature(r)
			runPrep(prep, r, id)
			r.git("checkout", "-q", "main")
			c := &repo{t, t.TempDir()}
			c.git("clone", "-q", "--single-branch", "-b", "feat", "file://"+r.dir, ".")
			return histScenario{r: c, id: id, file: "fetch.go", line: histRetry, anchor: a, noDefault: true}
		}},
		{name: "detached HEAD on an unmerged branch", base: baseOwn, build: func(t *testing.T, prep histPrep) histScenario {
			r := newRepo(t)
			id, a := feature(r)
			r.git("checkout", "-q", "main")
			r.git("checkout", "-q", "--detach", "feat")
			runPrep(prep, r, id)
			return histScenario{r: r, id: id, file: "fetch.go", line: histRetry, anchor: a, onBranch: true}
		}},
		{name: "linked worktree on an unmerged branch", base: baseOwn, build: func(t *testing.T, prep histPrep) histScenario {
			r := newRepo(t)
			id, a := feature(r)
			r.git("checkout", "-q", "main")
			w := &repo{t, filepath.Join(t.TempDir(), "wt")}
			r.git("worktree", "add", "-q", w.dir, "feat")
			runPrep(prep, w, id)
			return histScenario{r: w, id: id, file: "fetch.go", line: histRetry, anchor: a, onBranch: true}
		}},
		{name: "repository with no commits", noCommits: true},
		{name: "local main behind origin/main", base: baseAdded, build: func(t *testing.T, prep histPrep) histScenario {
			r := newRepo(t)
			first := r.head()
			id, a := feature(r)
			moves(r, "main")
			r.git("merge", "-q", "--squash", "feat")
			r.commit("squash feat")
			added := r.head()
			r.git("branch", "-qD", "feat")
			runPrep(prep, r, id)
			c := &repo{t, t.TempDir()}
			c.git("clone", "-q", "file://"+r.dir, ".")
			c.git("remote", "set-head", "origin", "-d")
			c.git("checkout", "-qb", "work", "origin/main")
			c.git("branch", "-qf", "main", first)
			return histScenario{r: c, id: id, file: "fetch.go", line: histRetry, anchor: a, added: added}
		}},
		{name: "no default branch at all", base: baseOwn, build: func(t *testing.T, prep histPrep) histScenario {
			r := newRepo(t)
			id, a := feature(r)
			r.git("checkout", "-q", "main")
			c := &repo{t, t.TempDir()}
			c.git("clone", "-q", "file://"+r.dir, ".")
			c.git("checkout", "-q", "origin/feat")
			c.git("branch", "-qD", "main")
			c.git("update-ref", "-d", "refs/remotes/origin/main")
			c.git("remote", "set-head", "origin", "-d")
			runPrep(prep, c, id)
			return histScenario{r: c, id: id, file: "fetch.go", line: histRetry, anchor: a, noDefault: true}
		}},
		{name: "origin/HEAD pointing at a branch named trunk, beside a stale main", base: baseOwn, build: func(t *testing.T, prep histPrep) histScenario {
			r := newRepo(t)
			r.git("branch", "-m", "main", "trunk")
			r.git("branch", "main")
			id, a := feature(r)
			moves(r, "trunk")
			r.git("merge", "-q", "--no-ff", "--no-edit", "feat")
			runPrep(prep, r, id)
			c := &repo{t, t.TempDir()}
			c.git("clone", "-q", "file://"+r.dir, ".")
			if got := c.git("symbolic-ref", "--short", "refs/remotes/origin/HEAD"); got != "origin/trunk" {
				t.Fatalf("want origin/HEAD at origin/trunk, got %s", got)
			}
			return histScenario{r: c, id: id, file: "fetch.go", line: histRetry, anchor: a}
		}},
		{name: "only master, note on an unmerged branch", base: baseOwn, build: func(t *testing.T, prep histPrep) histScenario {
			r := newRepo(t)
			r.git("branch", "-m", "main", "master")
			id, a := feature(r)
			runPrep(prep, r, id)
			return histScenario{r: r, id: id, file: "fetch.go", line: histRetry, anchor: a, onBranch: true}
		}},
		{name: "init -b trunk with no remote", base: baseOwn, build: func(t *testing.T, prep histPrep) histScenario {
			r := &repo{t, t.TempDir()}
			r.git("init", "-q", "-b", "trunk")
			r.write("fetch.go", "package x\n\nfunc Fetch() {}\n")
			r.commit("code")
			setLine3(r, "fetch.go", histRetry)
			r.commit("retry")
			id, a := anchoredFact(r, "Fetch retries", "fetch.go")
			r.commit("note")
			runPrep(prep, r, id)
			return histScenario{r: r, id: id, file: "fetch.go", line: histRetry, anchor: a, noDefault: true}
		}},
	}
}

func histShapes() []histShape {
	local := localShapes()
	all := append([]histShape{}, local...)
	for _, s := range local {
		all = append(all, fresh(s))
	}
	return append(all, otherShapes()...)
}

// spec is the anchor as the note writes it.
func (s histScenario) spec() string { return s.anchor + ":" + s.file + ":3" }

// live reports whether the note is still a live note on disk.
func (s histScenario) live() bool { return s.deletedIn == "" }

// checked reports whether lint can compare the anchored lines with HEAD: the
// base is a commit, not HEAD in a shallow clone.
func (s histScenario) checked(sh histShape) bool { return sh.base != baseShallow }

// branchRule reports whether the branch rule of LINT applies to the note.
func (s histScenario) branchRule(sh histShape) bool {
	return s.live() && s.checked(sh) && s.onBranch && !s.noDefault
}

// changeLine3 edits line 3 of the anchored file and commits it.
func changeLine3(r *repo, file string) {
	r.t.Helper()
	lines := strings.Split(r.read(file), "\n")
	lines[2] += " // changed"
	r.write(file, strings.Join(lines, "\n"))
	r.commit("change the anchored line")
}

// lintWarnings checks that res printed only warnings, and the ones the
// manual names for s.
func lintWarnings(t *testing.T, s histScenario, res result) {
	t.Helper()
	if s.noDefault && s.live() {
		has(t, res.stdout, "warning: no default branch")
	} else {
		lacks(t, res.stdout, "warning: no default branch")
	}
	if s.unseen > 0 && s.live() {
		has(t, res.stdout, "warning: shallow clone: 1 anchor commits")
	} else {
		lacks(t, res.stdout, "warning: shallow clone")
	}
}

const problemChanged = "the anchored code changed after the note"

type histCell struct {
	name string
	run  func(t *testing.T, sh histShape)
}

// prunedScenario builds the shape, then saves a revision that supersedes the
// note, commits it, prunes and commits the deletion, on the branch the shape
// reads and before any clone. It returns the scenario and the commit that
// pruned the note.
func prunedScenario(t *testing.T, sh histShape) (histScenario, string) {
	prunedIn := ""
	s := sh.build(t, func(r *repo, id string) {
		r.fact("Fetch retries, revised", "--supersedes", id)
		r.commit("revise")
		if !onDisk(r, id) {
			return
		}
		has(t, r.ok("prune"), "pruned ", "/"+id+"-")
		r.commit("prune")
		prunedIn = r.head()
	})
	if prunedIn == "" {
		prunedIn = s.deletedIn
	}
	return s, prunedIn
}

func histCells(sh histShape) []histCell {
	if sh.noCommits {
		return noCommitCells()
	}
	return []histCell{
		{"show reads at " + basePhrase[sh.base] + " and its heading says so", func(t *testing.T, sh histShape) {
			s := sh.build(t, nil)
			out := s.r.ok("show", s.id)
			heading := "# " + s.spec()
			switch sh.base {
			case baseAdded:
				heading += " (read at " + s.added + ", the commit that added the note)"
			case baseShallow:
				heading += " (read at HEAD, because this shallow clone lacks commit " + s.anchor + ")"
			}
			has(t, out, "id: "+s.id+"\n", "\n"+heading+"\n    3  "+s.line+"\n")
			if !s.live() && !strings.HasPrefix(out, "# pruned in "+s.deletedIn+"\n") {
				t.Fatalf("want a first line naming the commit that deleted the note, %s, got:\n%s", s.deletedIn, out)
			}
		}},
		{"show --json read_at", func(t *testing.T, sh histShape) {
			s := sh.build(t, nil)
			out := s.r.ok("show", s.id, "--json")
			has(t, out, `"anchor": "`+s.spec()+`"`, `"`+s.line+`"`)
			switch sh.base {
			case baseOwn:
				lacks(t, out, `"read_at"`)
			case baseAdded:
				has(t, out, `"read_at": "`+s.added+`"`)
			case baseShallow:
				has(t, out, `"read_at": "HEAD"`)
			}
		}},
		{"lint before any change exits 0 with only the warnings the manual names", func(t *testing.T, sh histShape) {
			s := sh.build(t, nil)
			res := s.r.gm("lint")
			if res.code != 0 {
				t.Fatalf("want exit 0, got %s", res)
			}
			for _, l := range strings.Split(strings.TrimSpace(res.stdout), "\n") {
				if l != "" && !strings.HasPrefix(l, "warning: ") {
					t.Fatalf("want only warning lines, got %s", res)
				}
			}
			lintWarnings(t, s, res)
		}},
		{"lint after the anchored lines change", func(t *testing.T, sh histShape) {
			s := sh.build(t, nil)
			changeLine3(s.r, s.file)
			res := s.r.gm("lint")
			if s.branchRule(sh) {
				if res.code != 1 {
					t.Fatalf("want exit 1 for the branch rule, got %s", res)
				}
				has(t, res.stdout, s.spec()+": "+problemChanged)
			} else {
				if res.code != 0 {
					t.Fatalf("want exit 0, got %s", res)
				}
				lacks(t, res.stdout, problemChanged)
			}
			lintWarnings(t, s, res)
		}},
		{"lint --review after the anchored lines change", func(t *testing.T, sh histShape) {
			s := sh.build(t, nil)
			changeLine3(s.r, s.file)
			res := s.r.gm("lint", "--review")
			review := "review: "
			line := s.spec() + " changed since its base commit"
			switch {
			case s.branchRule(sh):
				if res.code != 1 {
					t.Fatalf("want exit 1 for the branch rule, got %s", res)
				}
				has(t, res.stdout, s.spec()+": "+problemChanged)
				lacks(t, res.stdout, line)
			case s.live() && s.checked(sh):
				if res.code != 0 {
					t.Fatalf("want exit 0, got %s", res)
				}
				has(t, res.stdout, review, line)
			default:
				if res.code != 0 {
					t.Fatalf("want exit 0, got %s", res)
				}
				lacks(t, res.stdout, line, problemChanged)
			}
			lintWarnings(t, s, res)
		}},
		{"after a revision and prune, show OLD reads it from history with # pruned in", func(t *testing.T, sh histShape) {
			s, prunedIn := prunedScenario(t, sh)
			if sh.lostHistory {
				refused(t, s.r.gm("show", s.id), "no note")
				return
			}
			out := s.r.ok("show", s.id)
			if !strings.HasPrefix(out, "# pruned in "+prunedIn+"\n") {
				t.Fatalf("want a first line # pruned in %s, got:\n%s", prunedIn, out)
			}
			has(t, out, "id: "+s.id+"\n")
		}},
		{"after a revision and prune, search --all and list --all include it", func(t *testing.T, sh histShape) {
			s, prunedIn := prunedScenario(t, sh)
			lacks(t, s.r.ok("search", "fetch", "retries", "-n", "50"), s.id)
			lacks(t, s.r.ok("list"), s.id)
			search, list := s.r.ok("search", "fetch", "retries", "-n", "50", "--all"), s.r.ok("list", "--all")
			if sh.lostHistory {
				lacks(t, search, s.id)
				lacks(t, list, s.id)
				return
			}
			has(t, search, s.id, "pruned in "+prunedIn)
			has(t, list, s.id)
		}},
		{"after a revision and prune, lint accepts the supersedes entry, or warns where the clone lacks its history", func(t *testing.T, sh histShape) {
			s, _ := prunedScenario(t, sh)
			res := s.r.gm("lint")
			lacks(t, res.stdout, "supersedes unknown note")
			if res.code != 0 {
				t.Fatalf("want exit 0, got %s", res)
			}
			if sh.lostHistory {
				has(t, res.stdout, "warning: shallow clone")
			}
		}},
		{"after a revision and prune, save --supersedes accepts the pruned id, or refuses where the clone lacks its history", func(t *testing.T, sh histShape) {
			s, _ := prunedScenario(t, sh)
			if sh.lostHistory {
				refused(t, s.r.gm("save", "fact", "--title", "Fetch retries, revised again", "--keywords", "fetch retry backoff", "--fact", "f", "--evidence", "e", "--supersedes", s.id), "shallow clone", "fetch the full history")
				return
			}
			s.r.fact("Fetch retries, revised again", "--supersedes", s.id)
		}},
		{"after a revision and prune, retire accepts the pruned id, or refuses where the clone lacks its history", func(t *testing.T, sh histShape) {
			s, _ := prunedScenario(t, sh)
			if sh.lostHistory {
				refused(t, s.r.gm("retire", s.id, "--why", "Fetch no longer retries"), "shallow clone", "fetch the full history")
				return
			}
			s.r.ok("retire", s.id, "--why", "Fetch no longer retries")
		}},
	}
}

// emptyRepo is a repository with no commits that holds fetch.go uncommitted.
func emptyRepo(t *testing.T) *repo {
	r := &repo{t, t.TempDir()}
	r.git("init", "-q", "-b", "main")
	r.write("fetch.go", "package x\n\nfunc Fetch() {}\n")
	return r
}

// noCommitCells are the cells for a repository with no commits. No anchor can
// be pinned there, so each cell asserts what the manual says instead.
func noCommitCells() []histCell {
	revised := func(t *testing.T) (*repo, string) {
		r := emptyRepo(t)
		old := r.fact("Fetch retries")
		r.fact("Fetch retries, revised", "--supersedes", old)
		// The superseded note is not committed, so prune keeps it.
		out := r.ok("prune")
		has(t, out, "kept .agents/memory/"+old+"-")
		lacks(t, out, "pruned ")
		return r, old
	}
	skip := func(t *testing.T, _ histShape) {
		t.Skip("a repository with no commits has no commit to pin an anchor to, so no anchored lines exist to change")
	}
	return []histCell{
		{"show: save refuses the anchor, since the file is not committed at HEAD, and writes nothing", func(t *testing.T, _ histShape) {
			r := emptyRepo(t)
			refused(t, r.gm("save", "fact", "--title", "t", "--keywords", "a b c", "--fact", "f", "--evidence", "e", "--at", "fetch.go:3"), "note refused")
			if len(r.notes()) != 0 {
				t.Fatal("want no note written")
			}
		}},
		{"show --json of a note without anchors has no read_at", func(t *testing.T, _ histShape) {
			r := emptyRepo(t)
			out := r.ok("show", r.fact("Fetch retries"), "--json")
			lacks(t, out, `"read_at"`)
		}},
		{"lint of a note without anchors exits 0 and prints nothing", func(t *testing.T, _ histShape) {
			r := emptyRepo(t)
			r.fact("Fetch retries")
			if out := r.ok("lint"); out != "" {
				t.Fatalf("want no output, got %q", out)
			}
		}},
		{"lint after the anchored lines change", skip},
		{"lint --review after the anchored lines change", skip},
		{"after a revision, prune keeps the uncommitted note and show reads it from disk", func(t *testing.T, _ histShape) {
			r, old := revised(t)
			out := r.ok("show", old)
			has(t, out, "id: "+old+"\n")
			lacks(t, out, "# pruned in")
		}},
		{"after a revision, prune keeps the note and search --all and list --all include it", func(t *testing.T, _ histShape) {
			r, old := revised(t)
			has(t, r.ok("search", "fetch", "retries", "--all"), old, "(superseded)")
			has(t, r.ok("list", "--all"), old)
		}},
		{"after a revision, prune keeps the note and lint accepts the supersedes entry", func(t *testing.T, _ histShape) {
			r, _ := revised(t)
			r.ok("lint")
		}},
		{"after a revision, prune keeps the note and save --supersedes accepts its id", func(t *testing.T, _ histShape) {
			r, old := revised(t)
			r.fact("Fetch retries, revised again", "--supersedes", old)
		}},
		{"after a revision, prune keeps the note and retire accepts its id", func(t *testing.T, _ histShape) {
			r, old := revised(t)
			r.ok("retire", old, "--why", "Fetch no longer retries")
		}},
	}
}

// TestMatrixHistory runs every read of gm(1) against every history shape a
// note with a line-range anchor can go through: show and its base, show
// --json read_at, lint and its branch rule and warnings, lint --review, and
// pruning with the reads of a pruned note.
func TestMatrixHistory(t *testing.T) {
	for _, sh := range histShapes() {
		t.Run("history "+`"`+sh.name+`"`, func(t *testing.T) {
			t.Parallel()
			for _, c := range histCells(sh) {
				t.Run(c.name, func(t *testing.T) {
					t.Parallel()
					c.run(t, sh)
				})
			}
		})
	}
}
