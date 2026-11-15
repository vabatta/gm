package main

// A matrix of git settings against the flows of gm(1). The other specs run git
// with no user configuration at all. This file runs the same user-visible
// flows under one git setting at a time, then under a hostile profile of all
// of them together, set in the global file that GIT_CONFIG_GLOBAL names and
// again as repository-level config. The expected outcome of every flow comes
// from the manual and does not depend on the setting.

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// gitSetting is one row of the matrix: git config entries, environment
// variables, or a repository format that a user may have.
type gitSetting struct {
	name string
	// kv holds key=value entries. $HOOKS, $KEY, $SIGNERS and $FALSE name
	// fixtures: a directory of failing hooks, an SSH signing key, its allowed
	// signers file and the false program.
	kv  []string
	env []string
	// sha256 makes repositories with --object-format=sha256.
	sha256 bool
	// gitDir sets GIT_DIR and GIT_WORK_TREE for gm.
	gitDir bool
	// trunk leaves the branch name to init.defaultBranch, which is trunk.
	trunk bool
	// ignoreRevs writes .git-blame-ignore-revs, listing the commits that
	// change anchored code.
	ignoreRevs bool
	// globalOnly marks settings that git reads only from the global or
	// system file, or only when a repository is made.
	globalOnly bool
	// hostile puts the setting in the combined profile.
	hostile bool
}

// fixtures are the files that settings point at.
type fixtures struct{ hooks, noHooks, key, signers, falseBin string }

func newFixtures(t *testing.T) *fixtures {
	t.Helper()
	d := t.TempDir()
	fx := &fixtures{hooks: filepath.Join(d, "hooks"), noHooks: filepath.Join(d, "nohooks")}
	for _, dir := range []string{fx.hooks, fx.noHooks} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, h := range []string{"pre-commit", "commit-msg", "prepare-commit-msg", "post-commit", "pre-merge-commit", "post-merge",
		"post-checkout", "post-rewrite", "pre-rebase", "reference-transaction", "post-index-change", "pre-auto-gc", "pre-push"} {
		if err := os.WriteFile(filepath.Join(fx.hooks, h), []byte("#!/bin/sh\necho \"hook $0 failed\" >&2\nexit 1\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	key := filepath.Join(d, "key")
	if err := exec.Command("ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-C", "t@t", "-f", key).Run(); err == nil {
		pub, _ := os.ReadFile(key + ".pub")
		fx.key, fx.signers = key, filepath.Join(d, "allowed_signers")
		if err := os.WriteFile(fx.signers, []byte("t@t "+string(pub)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	fx.falseBin, _ = exec.LookPath("false")
	return fx
}

// mcase is a setting in one mode: "global", "repo" or "env".
type mcase struct {
	gitSetting
	mode    string
	cfgFile string      // the file GIT_CONFIG_GLOBAL names
	kv      [][2]string // the setting's entries with fixtures filled in
	fx      *fixtures
}

func (c *mcase) expand(t *testing.T) {
	t.Helper()
	for _, e := range c.gitSetting.kv {
		k, v, _ := strings.Cut(e, "=")
		fill := map[string]string{"$HOOKS": c.fx.hooks, "$KEY": c.fx.key, "$SIGNERS": c.fx.signers, "$FALSE": c.fx.falseBin}
		if f, ok := fill[v]; ok {
			if f == "" {
				t.Logf("skip %s: this machine lacks the fixture %s", k, v)
				continue
			}
			v = f
		}
		if k == "commit.gpgSign" && c.fx.key == "" {
			continue
		}
		c.kv = append(c.kv, [2]string{k, v})
	}
	c.cfgFile = "/dev/null"
	if c.mode == "global" {
		c.cfgFile = filepath.Join(t.TempDir(), "gitconfig")
		if err := os.WriteFile(c.cfgFile, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		for _, kv := range c.kv {
			cmd := exec.Command("git", "config", "--file", c.cfgFile, kv[0], kv[1])
			cmd.Env = env
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git config %s: %v\n%s", kv[0], err, out)
			}
		}
	}
}

// setupEnv is the environment of every process in the case: the harness env,
// the setting's variables and its global file.
func (c *mcase) setupEnv() []string {
	e := append(slices.Clone(env), c.env...)
	return append(e, "GIT_CONFIG_GLOBAL="+c.cfgFile)
}

// mrepo is a throwaway repository of one case. Its git calls see the case's
// configuration, as the user's own git would, except hooks, which the setup
// turns off so that failing hooks reach only gm.
type mrepo struct {
	t   *testing.T
	c   *mcase
	dir string
}

func (r *mrepo) run(args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-c", "core.hooksPath=" + r.c.fx.noHooks}, args...)...)
	cmd.Dir, cmd.Env = r.dir, r.c.setupEnv()
	var out, errb strings.Builder
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	if err != nil {
		return out.String() + errb.String(), err
	}
	return strings.TrimSpace(out.String()), nil
}

func (r *mrepo) git(args ...string) string {
	r.t.Helper()
	out, err := r.run(args...)
	if err != nil {
		r.t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return out
}

func (r *mrepo) write(name, s string) {
	r.t.Helper()
	path := filepath.Join(r.dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		r.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(s), 0o644); err != nil {
		r.t.Fatal(err)
	}
}

func (r *mrepo) commit(msg string) {
	r.t.Helper()
	r.git("add", "-A")
	r.git("commit", "-qm", msg)
}

// full is the full id of HEAD, and head its first 12 hex characters, the
// form gm writes in anchors and pruned in lines.
func (r *mrepo) full() string { r.t.Helper(); return r.git("rev-parse", "HEAD") }
func (r *mrepo) head() string { r.t.Helper(); return r.full()[:12] }

// branch is the name of the first branch.
func (r *mrepo) branch() string {
	if r.c.trunk {
		return "trunk"
	}
	return "main"
}

// ignore lists HEAD in .git-blame-ignore-revs when the case asks for the file.
func (r *mrepo) ignore() {
	r.t.Helper()
	if !r.c.ignoreRevs {
		return
	}
	path := filepath.Join(r.dir, ".git-blame-ignore-revs")
	old, _ := os.ReadFile(path)
	r.write(".git-blame-ignore-revs", string(old)+r.full()+"\n")
}

func (r *mrepo) gm(args ...string) result {
	r.t.Helper()
	cmd := exec.Command(os.Args[0], args...)
	e := append(r.c.setupEnv(), "GM_SPEC_RUN_MAIN=1")
	if r.c.gitDir {
		e = append(e, "GIT_DIR="+filepath.Join(r.dir, ".git"), "GIT_WORK_TREE="+r.dir)
	}
	cmd.Dir, cmd.Env = r.dir, e
	var out, errb strings.Builder
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	res := result{out.String(), errb.String(), cmd.ProcessState.ExitCode()}
	if err != nil && res.code < 0 {
		r.t.Fatal(err)
	}
	return res
}

func (r *mrepo) ok(args ...string) string {
	r.t.Helper()
	res := r.gm(args...)
	if res.code != 0 {
		r.t.Fatalf("gm %s: want exit 0, got %s", strings.Join(args, " "), res)
	}
	return res.stdout
}

// fails runs gm and fails the spec unless gm exits 1, as lint does on problems.
func (r *mrepo) fails(args ...string) string {
	r.t.Helper()
	res := r.gm(args...)
	if res.code != 1 {
		r.t.Fatalf("gm %s: want exit 1, got %s", strings.Join(args, " "), res)
	}
	return res.stdout
}

// clean fails the spec unless lint exits 0 and prints nothing. Without a
// default branch, which init.defaultBranch=trunk leaves, the manual has lint
// print a warning about line-range anchors, and clean allows that line.
func (r *mrepo) clean() {
	r.t.Helper()
	res := r.gm("lint")
	out := res.stdout
	if r.c.trunk {
		out = strings.TrimPrefix(out, "warning: no default branch (origin/HEAD, main, master, origin/main or origin/master), so line-range anchors on this branch were not checked for code that changed after the note\n")
	}
	if res.code != 0 || out != "" {
		r.t.Fatalf("want a clean lint, got %s", res)
	}
}

func (r *mrepo) fact(title string, extra ...string) string {
	r.t.Helper()
	out := r.ok(append([]string{"save", "fact", "--title", title, "--keywords", "fetch retry backoff", "--fact", "f", "--evidence", "e"}, extra...)...)
	return strings.Fields(out)[0]
}

// newMRepo makes a repository whose first commit holds fetch.go, with the
// case's repository format and, in repo mode, its config.
func newMRepo(t *testing.T, c *mcase) *mrepo {
	t.Helper()
	r := &mrepo{t, c, t.TempDir()}
	args := []string{"init", "-q"}
	if !c.trunk {
		args = append(args, "-b", "main")
	}
	if c.sha256 {
		args = append(args, "--object-format=sha256")
	}
	r.git(args...)
	if c.mode == "repo" {
		for _, kv := range c.kv {
			r.git("config", kv[0], kv[1])
		}
	}
	r.write("fetch.go", "package x\n\nfunc Fetch() {}\n")
	r.commit("code")
	r.ignore()
	return r
}

// cloneOf clones src over file://. In repo mode the clone gets the config too,
// because git clone does not copy it.
func cloneOf(t *testing.T, src *mrepo) *mrepo {
	t.Helper()
	r := &mrepo{t, src.c, t.TempDir()}
	args := []string{"clone", "-q"}
	if src.c.mode == "repo" {
		for _, kv := range src.c.kv {
			args = append(args, "-c", kv[0]+"="+kv[1])
		}
	}
	r.git(append(args, "file://"+src.dir, ".")...)
	return r
}

func matrixSettings() []gitSetting {
	return []gitSetting{
		{name: "baseline: safe.directory unset, repository owned by this user", globalOnly: true},
		{name: "color.ui=always", kv: []string{"color.ui=always"}, hostile: true},
		{name: "color.diff=always", kv: []string{"color.diff=always"}, hostile: true},
		{name: "color.status=always", kv: []string{"color.status=always"}, hostile: true},
		{name: "color.branch=always", kv: []string{"color.branch=always"}, hostile: true},
		{name: "log.showSignature=true with signed commits", kv: []string{"log.showSignature=true", "gpg.format=ssh", "user.signingKey=$KEY",
			"commit.gpgSign=true", "gpg.ssh.allowedSignersFile=$SIGNERS"}, hostile: true},
		{name: "log.decorate=full", kv: []string{"log.decorate=full"}, hostile: true},
		{name: "log.abbrevCommit=true", kv: []string{"log.abbrevCommit=true"}, hostile: true},
		{name: "log.follow=true", kv: []string{"log.follow=true"}, hostile: true},
		{name: "log.date=relative", kv: []string{"log.date=relative"}, hostile: true},
		{name: "format.pretty=fuller", kv: []string{"format.pretty=fuller"}, hostile: true},
		{name: "core.abbrev=7", kv: []string{"core.abbrev=7"}},
		{name: "core.abbrev=40", kv: []string{"core.abbrev=40"}},
		{name: "core.abbrev=no", kv: []string{"core.abbrev=no"}, hostile: true},
		{name: "diff.renames=copies", kv: []string{"diff.renames=copies"}, hostile: true},
		{name: "diff.renames=false", kv: []string{"diff.renames=false"}},
		{name: "diff.noprefix=true", kv: []string{"diff.noprefix=true"}, hostile: true},
		{name: "diff.mnemonicPrefix=true", kv: []string{"diff.mnemonicPrefix=true"}, hostile: true},
		{name: "diff.relative=true", kv: []string{"diff.relative=true"}, hostile: true},
		{name: "diff.external=false", kv: []string{"diff.external=$FALSE"}, hostile: true},
		{name: "diff.algorithm=patience", kv: []string{"diff.algorithm=patience"}, hostile: true},
		{name: "diff.indentHeuristic=false", kv: []string{"diff.indentHeuristic=false"}, hostile: true},
		{name: "core.quotePath=true", kv: []string{"core.quotePath=true"}},
		{name: "core.quotePath=false", kv: []string{"core.quotePath=false"}, hostile: true},
		{name: "core.autocrlf=true", kv: []string{"core.autocrlf=true"}, hostile: true},
		{name: "core.autocrlf=input", kv: []string{"core.autocrlf=input"}},
		// Not hostile: with core.autocrlf=true, git add refuses the LF files the flows write.
		{name: "core.safecrlf=true", kv: []string{"core.safecrlf=true"}},
		{name: "core.eol=crlf", kv: []string{"core.eol=crlf"}, hostile: true},
		{name: "core.fileMode=false", kv: []string{"core.fileMode=false"}, hostile: true},
		{name: "core.ignoreCase=true", kv: []string{"core.ignoreCase=true"}, hostile: true},
		{name: "core.precomposeUnicode=true", kv: []string{"core.precomposeUnicode=true"}, hostile: true},
		{name: "core.pager=less with PAGER and GIT_PAGER", kv: []string{"core.pager=less"}, env: []string{"PAGER=less", "GIT_PAGER=less"}, hostile: true},
		{name: "pager.log=true", kv: []string{"pager.log=true"}, hostile: true},
		{name: "status.showUntrackedFiles=no", kv: []string{"status.showUntrackedFiles=no"}, hostile: true},
		{name: "status.relativePaths=false", kv: []string{"status.relativePaths=false"}, hostile: true},
		{name: "status.branch=true", kv: []string{"status.branch=true"}, hostile: true},
		{name: "status.short=true", kv: []string{"status.short=true"}, hostile: true},
		{name: "status.renames=copies", kv: []string{"status.renames=copies"}, hostile: true},
		{name: "blame.coloring=highlightRecent", kv: []string{"blame.coloring=highlightRecent"}, hostile: true},
		{name: "blame.markIgnoredLines=true", kv: []string{"blame.markIgnoredLines=true"}, hostile: true},
		{name: "blame.ignoreRevsFile, file present", kv: []string{"blame.ignoreRevsFile=.git-blame-ignore-revs"}, ignoreRevs: true, hostile: true},
		{name: "blame.ignoreRevsFile, file absent", kv: []string{"blame.ignoreRevsFile=.git-blame-ignore-revs"}},
		{name: "merge.renames=false", kv: []string{"merge.renames=false"}, hostile: true},
		{name: "init.defaultBranch=trunk", kv: []string{"init.defaultBranch=trunk"}, trunk: true, globalOnly: true},
		{name: "core.hooksPath to failing hooks", kv: []string{"core.hooksPath=$HOOKS"}, hostile: true},
		{name: "i18n.logOutputEncoding=latin1", kv: []string{"i18n.logOutputEncoding=latin1"}, hostile: true},
		{name: "core.commentChar=;", kv: []string{"core.commentChar=;"}, hostile: true},
		{name: "advice.* off", kv: []string{"advice.statusHints=false", "advice.detachedHead=false", "advice.resolveConflict=false",
			"advice.implicitIdentity=false", "advice.commitBeforeMerge=false", "advice.addIgnoredFile=false", "advice.skippedCherryPicks=false",
			"advice.waitingForEditor=false", "advice.mergeConflict=false", "advice.forceDeleteBranch=false"}, hostile: true},
		// de_DE.UTF-8 is not in this machine's locale archive, so LC_ALL picks
		// a UTF-8 locale that is, and LANGUAGE picks git's German messages.
		{name: "German locale (LANG=de_DE.UTF-8, LANGUAGE=de)", env: []string{"LANG=de_DE.UTF-8", "LC_ALL=en_US.UTF-8", "LANGUAGE=de"}, hostile: true},
		{name: "GIT_DIR and GIT_WORK_TREE set", gitDir: true, hostile: true},
		{name: "SHA-256 repository", sha256: true},
	}
}

// hostileSetting merges every setting marked hostile into one.
func hostileSetting(all []gitSetting) gitSetting {
	h := gitSetting{name: "hostile profile"}
	for _, s := range all {
		if !s.hostile {
			continue
		}
		h.kv = append(h.kv, s.kv...)
		h.env = append(h.env, s.env...)
		h.gitDir = h.gitDir || s.gitDir
		h.ignoreRevs = h.ignoreRevs || s.ignoreRevs
	}
	return h
}

func TestMatrixGitConfig(t *testing.T) {
	fx := newFixtures(t)
	settings := matrixSettings()
	settings = append(settings, hostileSetting(settings))
	for _, s := range settings {
		modes := []string{"global", "repo"}
		switch {
		case len(s.kv) == 0:
			modes = []string{"env"}
		case s.globalOnly:
			modes = []string{"global"}
		}
		for _, mode := range modes {
			t.Run(s.name+"/"+mode, func(t *testing.T) {
				t.Parallel()
				c := &mcase{gitSetting: s, mode: mode, fx: fx}
				c.expand(t)
				matrixFlows(t, c)
			})
		}
	}
}

// matrixFlows runs every flow of the matrix under one case.
func matrixFlows(t *testing.T, c *mcase) {
	t.Run("show prints the code of a whole-file and a line-range anchor under their headings", func(t *testing.T) {
		r := newMRepo(t, c)
		id := r.fact("Fetch retries with backoff", "--at", "fetch.go", "--at", "fetch.go:3")
		r.commit("note")
		h := r.git("rev-parse", "HEAD~1")[:12]
		has(t, r.ok("show", id), "title: \"Fetch retries with backoff\"",
			"\n# "+h+":fetch.go\n    1  package x\n    2  \n    3  func Fetch() {}\n",
			"\n# "+h+":fetch.go:3\n    3  func Fetch() {}\n")
	})
	t.Run("show --json holds the code of each anchor", func(t *testing.T) {
		r := newMRepo(t, c)
		id := r.fact("Fetch retries with backoff", "--at", "fetch.go", "--at", "fetch.go:3")
		r.commit("note")
		out := r.ok("show", id, "--json")
		var got struct {
			ID   string
			Code []struct {
				Anchor, ReadAt, Error string
				From                  int
				Lines                 []string
			}
		}
		if err := json.Unmarshal([]byte(out), &got); err != nil {
			t.Fatalf("want JSON, got %v:\n%s", err, out)
		}
		if got.ID != id || len(got.Code) != 2 || got.Code[0].Error != "" || got.Code[1].Error != "" || got.Code[0].ReadAt != "" ||
			len(got.Code[0].Lines) != 3 || got.Code[1].From != 3 || strings.Join(got.Code[1].Lines, "|") != "func Fetch() {}" {
			t.Fatalf("want the note and the code of both anchors, got:\n%s", out)
		}
	})
	t.Run("search finds the note, and --json parses", func(t *testing.T) {
		r := newMRepo(t, c)
		id := r.fact("Fetch retries with backoff", "--at", "fetch.go", "--at", "fetch.go:3")
		r.commit("note")
		has(t, r.ok("search", "fetch", "retry"), id, "Fetch retries with backoff", ".agents/memory/"+id+"-")
		equal(t, ids(t, r.ok("search", "fetch", "retry", "--json")), []string{id})
	})
	t.Run("list and list --json print the live note", func(t *testing.T) {
		r := newMRepo(t, c)
		id := r.fact("Fetch retries with backoff", "--at", "fetch.go", "--at", "fetch.go:3")
		r.commit("note")
		has(t, r.ok("list"), "\nfact\n", id, "Fetch retries with backoff")
		equal(t, ids(t, r.ok("list", "--json")), []string{id})
	})
	t.Run("lint is clean for a sound anchored note", func(t *testing.T) {
		r := newMRepo(t, c)
		r.fact("Fetch retries with backoff", "--at", "fetch.go", "--at", "fetch.go:3")
		r.clean()
		r.commit("note")
		r.clean()
	})
	t.Run("lint fails on a branch whose anchored lines changed, and --review lists the whole-file anchor", func(t *testing.T) {
		r := newMRepo(t, c)
		r.git("checkout", "-qb", "feat")
		r.fact("Fetch is empty", "--at", "fetch.go", "--at", "fetch.go:3")
		r.commit("note")
		r.write("fetch.go", "package x\n\nfunc Fetch() { retry() }\n")
		r.commit("change after the note")
		r.ignore()
		if c.trunk {
			// No origin and no main or master: the manual says lint skips
			// the branch rule and warns.
			res := r.gm("lint", "--review")
			if res.code != 0 {
				t.Fatalf("want exit 0 without a default branch, got %s", res)
			}
			has(t, res.stdout, "warning: no default branch", "review: ", ":fetch.go:3 changed", ":fetch.go changed")
			return
		}
		out := r.fails("lint")
		has(t, out, ":fetch.go:3: the anchored code changed after the note")
		lacks(t, out, ":fetch.go: ", "review: ")
		has(t, r.fails("lint", "--review"), ":fetch.go:3: the anchored code changed after the note", "review: ", ":fetch.go changed since its base commit")
		var rep struct{ Problems, Review []string }
		js := r.fails("lint", "--review", "--json")
		if err := json.Unmarshal([]byte(js), &rep); err != nil || len(rep.Problems) != 1 || len(rep.Review) != 1 {
			t.Fatalf("want one problem and one review line as JSON, got %v:\n%s", err, js)
		}
	})
	t.Run("lint reports the anchors of a file that git rm removed", func(t *testing.T) {
		r := newMRepo(t, c)
		r.fact("Fetch lives here", "--at", "fetch.go", "--at", "fetch.go:3")
		r.commit("note")
		r.git("rm", "-q", "fetch.go")
		r.commit("remove")
		has(t, r.fails("lint"), ":fetch.go: the file no longer exists at HEAD", ":fetch.go:3: the file no longer exists at HEAD")
	})
	t.Run("lint names the new path of a file that git mv moved", func(t *testing.T) {
		r := newMRepo(t, c)
		r.fact("Fetch lives here", "--at", "fetch.go", "--at", "fetch.go:3")
		r.commit("note")
		r.git("mv", "fetch.go", "client.go")
		r.commit("move")
		out := r.fails("lint")
		if strings.Count(out, "the file moved to client.go") != 2 {
			t.Fatalf("want both anchors to name client.go, got:\n%s", out)
		}
	})
	t.Run("after a revision and prune, show, search --all and lint read the pruned note", func(t *testing.T) {
		r := newMRepo(t, c)
		old := r.fact("Fetch retries twice", "--at", "fetch.go:3")
		r.commit("old")
		r.fact("Fetch retries five times", "--supersedes", old, "--at", "fetch.go:3")
		r.commit("revision")
		has(t, r.ok("prune"), "pruned .agents/memory/"+old+"-")
		r.commit("prune")
		h := r.head()
		has(t, r.ok("show", old), "# pruned in "+h+"\n", "title: \"Fetch retries twice\"", "    3  func Fetch() {}\n")
		has(t, r.ok("show", old, "--json"), `"pruned_in": "`+h+`"`)
		has(t, r.ok("search", "fetch", "retries", "--all"), old, "(superseded, pruned in "+h+")")
		has(t, r.ok("search", "fetch", "retries", "--all", "--json"), `"pruned_in": "`+h+`"`)
		has(t, r.ok("list", "--all"), old)
		r.clean()
	})
	t.Run("after a squash merge, a fresh clone reads the anchor at the commit that added the note", func(t *testing.T) {
		r := newMRepo(t, c)
		r.git("checkout", "-qb", "feat")
		r.write("fetch.go", "package x\n\nfunc Fetch() { retry() }\n")
		r.commit("retry")
		id := r.fact("Fetch retries", "--at", "fetch.go:3")
		r.commit("note")
		r.git("checkout", "-q", r.branch())
		r.write("other.go", "package x\n")
		r.commit("main moves")
		r.git("merge", "-q", "--squash", "feat")
		r.commit("squash feat")
		added := r.head()
		r.git("branch", "-qD", "feat")
		clone := cloneOf(t, r)
		has(t, clone.ok("show", id), ":fetch.go:3 (read at "+added+", the commit that added the note)\n    3  func Fetch() { retry() }\n")
		clone.clean()
	})
	t.Run("import engram writes the observations and keeps a judged supersedes relation", func(t *testing.T) {
		r := newMRepo(t, c)
		obs := func(id, title string) string {
			b, _ := json.Marshal(map[string]string{"sync_id": id, "type": "decision", "title": title, "content": "c", "topic_key": "topic/" + id, "created_at": "2026-11-13 10:00:00"})
			return string(b)
		}
		file := filepath.Join(t.TempDir(), "export.json")
		export := `{"observations": [` + obs("obs-1", "Old") + "," + obs("obs-2", "New") + `], "relations": [` +
			`{"source_id": "obs-2", "target_id": "obs-1", "relation": "supersedes", "judgment_status": "judged"}]}`
		if err := os.WriteFile(file, []byte(export), 0o644); err != nil {
			t.Fatal(err)
		}
		has(t, r.ok("import", "engram", file), "imported 2 notes", "1 supersedes links")
		out := r.ok("list")
		has(t, out, "New")
		lacks(t, out, "Old")
		r.clean()
		r.commit("import")
		r.clean()
	})
}
