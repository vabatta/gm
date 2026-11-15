package main

// Behaviour specs for gm. Each *_test.go file holds one section of gm(1) or
// gm-note(5) as one test, with one subtest per sentence. This file holds the
// harness they share. Every spec runs the real gm command in its own process
// against a throwaway git repository. It checks only what a user sees, which
// is the exit status, stdout, stderr and the files in the repository.

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestMain lets the test binary act as gm. With GM_SPEC_RUN_MAIN=1 it runs
// main instead of the tests, so each spec runs gm as its own process.
func TestMain(m *testing.M) {
	if os.Getenv("GM_SPEC_RUN_MAIN") == "1" {
		main()
	}
	os.Exit(m.Run())
}

// env keeps the user's git configuration out of the specs.
var env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1",
	"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")

type repo struct {
	t   *testing.T
	dir string
}

// newRepo makes a repository whose first commit holds fetch.go.
func newRepo(t *testing.T) *repo {
	t.Helper()
	r := &repo{t, t.TempDir()}
	r.git("init", "-q", "-b", "main")
	r.write("fetch.go", "package x\n\nfunc Fetch() {}\n")
	r.commit("code")
	return r
}

func (r *repo) git(args ...string) string {
	r.t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir, cmd.Env = r.dir, env
	out, err := cmd.CombinedOutput()
	if err != nil {
		r.t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func (r *repo) write(name, s string) {
	r.t.Helper()
	path := filepath.Join(r.dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		r.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(s), 0o644); err != nil {
		r.t.Fatal(err)
	}
}

func (r *repo) read(name string) string {
	r.t.Helper()
	b, err := os.ReadFile(filepath.Join(r.dir, name))
	if err != nil {
		r.t.Fatal(err)
	}
	return string(b)
}

func (r *repo) commit(msg string) {
	r.t.Helper()
	r.git("add", "-A")
	r.git("commit", "-qm", msg)
}

func (r *repo) head() string { r.t.Helper(); return r.git("rev-parse", "--short=12", "HEAD") }

// notes lists the note files in .memory/notes and in the notes folder that
// .gmconfig names.
func (r *repo) notes() []string {
	files, _ := filepath.Glob(filepath.Join(r.dir, ".memory", "notes", "*.md"))
	if dir := r.git("config", "--file", ".gmconfig", "--default", "", "gm.dir"); dir != "" {
		more, _ := filepath.Glob(filepath.Join(r.dir, dir, "notes", "*.md"))
		files = append(files, more...)
	}
	return files
}

// note writes a note file by hand, for states that gm save never produces.
func (r *repo) note(id, header, body string) {
	r.t.Helper()
	r.write(".memory/notes/"+id+"-hand.md", "---\nid: "+id+"\n"+header+"---\n"+body)
}

type result struct {
	stdout, stderr string
	code           int
}

func (res result) String() string {
	return fmt.Sprintf("exit %d\nstdout:\n%s\nstderr:\n%s", res.code, res.stdout, res.stderr)
}

func gmIn(t *testing.T, dir string, args ...string) result {
	t.Helper()
	cmd := exec.Command(os.Args[0], args...)
	cmd.Dir, cmd.Env = dir, append(env, "GM_SPEC_RUN_MAIN=1")
	var out, errb strings.Builder
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	res := result{out.String(), errb.String(), cmd.ProcessState.ExitCode()}
	if err != nil && res.code < 0 {
		t.Fatal(err)
	}
	return res
}

func (r *repo) gm(args ...string) result { r.t.Helper(); return gmIn(r.t, r.dir, args...) }

// ok runs gm, fails the spec unless gm exits 0, and returns stdout.
func (r *repo) ok(args ...string) string {
	r.t.Helper()
	res := r.gm(args...)
	if res.code != 0 {
		r.t.Fatalf("gm %s: want exit 0, got %s", strings.Join(args, " "), res)
	}
	return res.stdout
}

// fact saves a sound fact note and returns its id.
func (r *repo) fact(title string, extra ...string) string {
	r.t.Helper()
	out := r.ok(append([]string{"save", "fact", "--title", title, "--keywords", "fetch retry backoff", "--fact", "f", "--evidence", "e"}, extra...)...)
	return strings.Fields(out)[0]
}

// decision saves a sound decision note on topic decision/cache and returns its id.
func (r *repo) decision(title string, supersedes ...string) string {
	r.t.Helper()
	args := []string{"save", "decision", "--title", title, "--keywords", "cache store choice", "--topic", "decision/cache",
		"--decision", title, "--why", "w", "--rejected", "r"}
	for _, id := range supersedes {
		args = append(args, "--supersedes", id)
	}
	return strings.Fields(r.ok(args...))[0]
}

func has(t *testing.T, got string, want ...string) {
	t.Helper()
	for _, w := range want {
		if !strings.Contains(got, w) {
			t.Fatalf("want %q in:\n%s", w, got)
		}
	}
}

func lacks(t *testing.T, got string, unwanted ...string) {
	t.Helper()
	for _, u := range unwanted {
		if strings.Contains(got, u) {
			t.Fatalf("want no %q in:\n%s", u, got)
		}
	}
}

func refused(t *testing.T, res result, want ...string) {
	t.Helper()
	if res.code != 2 {
		t.Fatalf("want exit 2, got %s", res)
	}
	has(t, res.stderr, want...)
}

// ids returns the ids that gm search or gm list printed with --json, in order.
func ids(t *testing.T, out string) []string {
	t.Helper()
	var xs []struct{ ID string }
	if err := json.Unmarshal([]byte(out), &xs); err != nil {
		t.Fatalf("want JSON, got %v:\n%s", err, out)
	}
	var s []string
	for _, x := range xs {
		s = append(s, x.ID)
	}
	return s
}

func equal(t *testing.T, got, want []string) {
	t.Helper()
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("want %v, got %v", want, got)
	}
}

// merged makes a repository whose main branch holds a branch that committed
// code and then a note anchored to it. strategy, "squash" or "rebase", picks
// how the branch merges while main moves on. merged returns a clone over
// file:// and the note's id. The clone holds only what main reaches, as a
// clone from GitHub does once the branch is gone.
func merged(t *testing.T, strategy string) (*repo, string) {
	t.Helper()
	r := newRepo(t)
	r.git("checkout", "-qb", "feat")
	r.write("fetch.go", "package x\n\nfunc Fetch() { retry() }\n")
	r.commit("retry")
	id := r.fact("Fetch retries", "--at", "fetch.go:3")
	r.commit("note")
	r.git("checkout", "-q", "main")
	r.write("other.go", "package x\n")
	r.commit("main moves")
	if strategy == "squash" {
		r.git("merge", "-q", "--squash", "feat")
		r.commit("squash feat")
	} else {
		r.git("checkout", "-q", "feat")
		r.git("rebase", "-q", "main")
		r.git("checkout", "-q", "main")
		r.git("merge", "-q", "--ff-only", "feat")
	}
	r.git("branch", "-qD", "feat")
	clone := &repo{t, t.TempDir()}
	clone.git("clone", "-q", "file://"+r.dir, ".")
	return clone, id
}
