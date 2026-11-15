package main

// A behaviour matrix for gm: every git state of an anchored file or a
// superseded note file against every command that reads it, then every edge
// of every input against the commands that take it. The expected outcome of
// each cell comes from a sentence of gm(1) or gm-note(5). Where the manual
// says nothing, the cell is marked "manual silent" and checks only that gm
// exits 0, 1 or 2 without a panic.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// want is the outcome of one cell. exit -1 means the manual is silent, so
// any of 0, 1 or 2 passes. has and lacks are matched against stdout and
// stderr together, after {path} and {id} expansion. quiet asks for no stdout.
type want struct {
	exit  int
	has   []string
	lacks []string
	quiet bool
}

const silent = -1

func expect(t *testing.T, res result, w want, rep *strings.Replacer) {
	t.Helper()
	if strings.Contains(res.stderr, "panic:") || strings.Contains(res.stderr, "goroutine ") {
		t.Fatalf("gm crashed: %s", res)
	}
	if w.exit == silent {
		if res.code < 0 || res.code > 2 {
			t.Fatalf("manual silent: want exit 0, 1 or 2, got %s", res)
		}
		return
	}
	if res.code != w.exit {
		t.Fatalf("want exit %d, got %s", w.exit, res)
	}
	out := res.stdout + res.stderr
	for _, s := range w.has {
		if s = rep.Replace(s); !strings.Contains(out, s) {
			t.Fatalf("want %q in %s", s, res)
		}
	}
	for _, s := range w.lacks {
		if s = rep.Replace(s); strings.Contains(out, s) {
			t.Fatalf("want no %q in %s", s, res)
		}
	}
	if w.quiet && res.stdout != "" {
		t.Fatalf("want no stdout, got %s", res)
	}
}

// fileOf returns the path, from the repository root, of the note file whose
// name starts with id.
func fileOf(r *repo, id string) string {
	r.t.Helper()
	for _, f := range r.notes() {
		if strings.HasPrefix(filepath.Base(f), id) {
			return strings.TrimPrefix(f, r.dir+"/")
		}
	}
	r.t.Fatalf("no file for note %s", id)
	return ""
}

func exists(r *repo, p string) bool {
	_, err := os.Stat(filepath.Join(r.dir, p))
	return err == nil
}

var (
	accept     = want{exit: 0}
	dirty      = want{exit: 2, has: []string{"uncommitted changes"}}
	notAtHead  = want{exit: 2, has: []string{"not committed at HEAD"}}
	cleanLint  = want{exit: 0, quiet: true}
	codeShown  = want{exit: 0, has: []string{"    3  func Fetch() {}\n"}}
	unknownSup = want{exit: 1, has: []string{"supersedes unknown note {id}"}}
)

// gitState is one row of part 1. apply puts the file at p into the state
// and returns where the file is afterwards. atNew anchors save --at to that
// new path instead of p. notePrune is "pruned", "kept" or "absent" (the file
// is not in the working tree, so prune neither deletes nor keeps it).
type gitState struct {
	name               string
	apply              func(r *repo, p string) string
	atNew              bool
	save               want
	codeLint, codeShow want
	notePrune          string
	noteLint, noteShow want
}

func TestMatrixState(t *testing.T) {
	t.Run("git state x command", matrixGitState)
	t.Run("input x command", matrixInput)
}

func matrixGitState(t *testing.T) {
	edit := func(r *repo, p string) { r.write(p, r.read(p)+"\nedited\n") }
	untrack := func(r *repo, p string) {
		r.git("rm", "-q", "--cached", p)
		r.git("commit", "-qm", "untrack "+p)
	}
	noteShown := want{exit: 0, has: []string{`title: "Fetch retries twice"`}}
	noteEdited := want{exit: 0, has: []string{`title: "Fetch retries twice"`, "edited"}}
	states := []gitState{
		{name: "committed and clean", apply: func(r *repo, p string) string { return p },
			save: accept, codeLint: cleanLint, codeShow: codeShown, notePrune: "pruned", noteLint: cleanLint, noteShow: noteShown},
		{name: "modified and unstaged", apply: func(r *repo, p string) string { edit(r, p); return p },
			save: dirty, codeLint: cleanLint, codeShow: codeShown, notePrune: "kept", noteLint: cleanLint, noteShow: noteEdited},
		{name: "staged", apply: func(r *repo, p string) string { edit(r, p); r.git("add", p); return p },
			save: dirty, codeLint: cleanLint, codeShow: codeShown, notePrune: "kept", noteLint: cleanLint, noteShow: noteEdited},
		{name: "staged then modified again", apply: func(r *repo, p string) string { edit(r, p); r.git("add", p); edit(r, p); return p },
			save: dirty, codeLint: cleanLint, codeShow: codeShown, notePrune: "kept", noteLint: cleanLint, noteShow: noteEdited},
		{name: "untracked", apply: func(r *repo, p string) string { untrack(r, p); return p },
			save: notAtHead, codeLint: want{exit: 1, has: []string{"no longer exists at HEAD"}}, codeShow: codeShown,
			notePrune: "kept", noteLint: cleanLint, noteShow: noteShown},
		{name: "git-ignored", apply: func(r *repo, p string) string {
			r.git("rm", "-q", "--cached", p)
			r.write(".gitignore", "/"+p+"\n")
			r.git("add", ".gitignore")
			r.git("commit", "-qm", "ignore "+p)
			return p
		}, save: notAtHead, codeLint: want{exit: 1, has: []string{"no longer exists at HEAD"}}, codeShow: codeShown,
			notePrune: "kept", noteLint: cleanLint, noteShow: noteShown},
		{name: "deleted and unstaged", apply: func(r *repo, p string) string {
			if err := os.Remove(filepath.Join(r.dir, p)); err != nil {
				r.t.Fatal(err)
			}
			return p
		}, save: dirty, codeLint: cleanLint, codeShow: codeShown, notePrune: "absent", noteLint: unknownSup, noteShow: want{exit: silent}},
		{name: "deleted and staged", apply: func(r *repo, p string) string { r.git("rm", "-q", p); return p },
			save: dirty, codeLint: cleanLint, codeShow: codeShown, notePrune: "absent", noteLint: unknownSup, noteShow: want{exit: silent}},
		{name: "renamed and staged, anchored at the old path", apply: func(r *repo, p string) string {
			np := strings.TrimSuffix(p, filepath.Ext(p)) + "-moved" + filepath.Ext(p)
			r.git("mv", p, np)
			return np
		}, save: dirty, codeLint: cleanLint, codeShow: codeShown, notePrune: "kept", noteLint: cleanLint, noteShow: noteShown},
		{name: "renamed and staged, anchored at the new path", atNew: true, apply: func(r *repo, p string) string {
			np := strings.TrimSuffix(p, filepath.Ext(p)) + "-moved" + filepath.Ext(p)
			r.git("mv", p, np)
			return np
		}, save: notAtHead, codeLint: cleanLint, codeShow: codeShown, notePrune: "kept", noteLint: cleanLint, noteShow: noteShown},
		{name: "mode change only", apply: func(r *repo, p string) string {
			if err := os.Chmod(filepath.Join(r.dir, p), 0o755); err != nil {
				r.t.Fatal(err)
			}
			return p
		}, save: dirty, codeLint: cleanLint, codeShow: codeShown, notePrune: "kept", noteLint: cleanLint, noteShow: noteShown},
		{name: "assume-unchanged and unmodified", apply: func(r *repo, p string) string {
			r.git("update-index", "--assume-unchanged", p)
			return p
		}, save: accept, codeLint: cleanLint, codeShow: codeShown, notePrune: "pruned", noteLint: cleanLint, noteShow: noteShown},
		{name: "assume-unchanged and modified", apply: func(r *repo, p string) string {
			r.git("update-index", "--assume-unchanged", p)
			edit(r, p)
			return p
		}, save: dirty, codeLint: cleanLint, codeShow: codeShown, notePrune: "kept", noteLint: cleanLint, noteShow: noteEdited},
		{name: "skip-worktree and unmodified", apply: func(r *repo, p string) string {
			r.git("update-index", "--skip-worktree", p)
			return p
		}, save: accept, codeLint: cleanLint, codeShow: codeShown, notePrune: "pruned", noteLint: cleanLint, noteShow: noteShown},
		{name: "skip-worktree and modified", apply: func(r *repo, p string) string {
			r.git("update-index", "--skip-worktree", p)
			edit(r, p)
			return p
		}, save: dirty, codeLint: cleanLint, codeShow: codeShown, notePrune: "kept", noteLint: cleanLint, noteShow: noteEdited},
		{name: "excluded by sparse-checkout", apply: func(r *repo, p string) string {
			r.git("sparse-checkout", "set", "--no-cone", "/*", "!/"+p)
			if exists(r, p) {
				r.t.Fatalf("sparse-checkout left %s in the working tree", p)
			}
			return p
		}, save: accept, codeLint: cleanLint, codeShow: codeShown, notePrune: "absent", noteLint: unknownSup, noteShow: want{exit: silent}},
	}

	saveAt := func(r *repo, p string) result {
		return r.gm("save", "fact", "--title", "Anchored", "--keywords", "fetch retry backoff", "--fact", "f", "--evidence", "e", "--at", p)
	}
	// saveCell runs save --at and, on a refusal, checks that nothing was written.
	saveCell := func(t *testing.T, r *repo, p string, w want, rep *strings.Replacer) {
		before := len(r.notes())
		res := saveAt(r, p)
		expect(t, res, w, rep)
		if res.code == 2 && len(r.notes()) != before {
			t.Fatalf("a refused save wrote a note")
		}
	}
	saveName := func(w want) string {
		if w.exit == 0 {
			return "save --at accepts the anchor"
		}
		return "save --at refuses: " + w.has[0]
	}
	lintName := func(w want) string {
		switch {
		case w.exit == silent:
			return "lint does not crash (manual silent)"
		case w.exit == 0:
			return "lint is clean"
		}
		return "lint fails: " + w.has[0]
	}
	showName := func(w want) string {
		if w.exit == silent {
			return "show does not crash (manual silent)"
		}
		return "show prints the note and the code at its base"
	}

	t.Run("anchored code file", func(t *testing.T) {
		// setup makes a live note anchored to fetch.go:3 that supersedes an
		// older note, both committed, and puts fetch.go into the state.
		setup := func(t *testing.T, s gitState) (r *repo, live, old, p string) {
			r = newRepo(t)
			old = r.fact("Fetch is empty", "--at", "fetch.go:3")
			live = r.fact("Fetch is still empty", "--at", "fetch.go:3", "--supersedes", old)
			r.commit("notes")
			p = s.apply(r, "fetch.go")
			if !s.atNew {
				p = "fetch.go"
			}
			return r, live, old, p
		}
		for _, s := range states {
			t.Run(s.name, func(t *testing.T) {
				t.Run(saveName(s.save), func(t *testing.T) {
					t.Parallel()
					r, _, _, p := setup(t, s)
					saveCell(t, r, p, s.save, strings.NewReplacer())
				})
				t.Run("prune deletes the superseded note, whatever the state of the code", func(t *testing.T) {
					t.Parallel()
					r, _, old, _ := setup(t, s)
					f := fileOf(r, old)
					expect(t, r.gm("prune"), want{exit: 0, has: []string{"pruned " + f}}, strings.NewReplacer())
					if exists(r, f) {
						t.Fatalf("want %s deleted", f)
					}
				})
				t.Run(lintName(s.codeLint), func(t *testing.T) {
					t.Parallel()
					r, _, _, _ := setup(t, s)
					expect(t, r.gm("lint"), s.codeLint, strings.NewReplacer())
				})
				t.Run(showName(s.codeShow), func(t *testing.T) {
					t.Parallel()
					r, live, _, _ := setup(t, s)
					expect(t, r.gm("show", live), s.codeShow, strings.NewReplacer())
				})
			})
		}
		t.Run("in a submodule", func(t *testing.T) {
			setup := func(t *testing.T) (r *repo, id, at string) {
				sub := newRepo(t)
				r = newRepo(t)
				r.git("-c", "protocol.file.allow=always", "submodule", "add", "-q", "file://"+sub.dir, "sub")
				r.git("commit", "-qm", "submodule")
				id, at = "20200101T000000Z-aaaaaa", r.head()
				r.note(id, "type: fact\ntitle: \"Sub fetch\"\nkeywords: a b c\nat: ["+at+":sub/fetch.go:3]\ncreated: 2020-01-01T00:00:00Z\n", "## Fact\nf\n\n## Evidence\ne\n")
				r.commit("note")
				return r, id, at
			}
			t.Run("save --at a file in the submodule refuses: not committed at HEAD", func(t *testing.T) {
				t.Parallel()
				r, _, _ := setup(t)
				saveCell(t, r, "sub/fetch.go", notAtHead, strings.NewReplacer())
			})
			t.Run("save --at the submodule itself refuses: the path names a directory", func(t *testing.T) {
				t.Parallel()
				r, _, _ := setup(t)
				saveCell(t, r, "sub", want{exit: 2, has: []string{"directory"}}, strings.NewReplacer())
			})
			t.Run("lint fails: the anchored file is gone at HEAD", func(t *testing.T) {
				t.Parallel()
				r, _, _ := setup(t)
				expect(t, r.gm("lint"), want{exit: 1, has: []string{"sub/fetch.go"}}, strings.NewReplacer())
			})
			t.Run("show prints the note and the reason git cannot read the code", func(t *testing.T) {
				t.Parallel()
				r, id, at := setup(t)
				expect(t, r.gm("show", id), want{exit: 0, has: []string{`title: "Sub fetch"`, "\n# " + at + ":sub/fetch.go:3: "}, lacks: []string{"    3  func Fetch"}}, strings.NewReplacer())
			})
			t.Run("prune deletes a committed superseded note", func(t *testing.T) {
				t.Parallel()
				r, _, _ := setup(t)
				old := r.fact("Fetch retries twice")
				r.fact("Fetch retries five times", "--supersedes", old)
				r.commit("notes")
				f := fileOf(r, old)
				expect(t, r.gm("prune"), want{exit: 0, has: []string{"pruned " + f}}, strings.NewReplacer())
			})
		})
	})

	t.Run("superseded note file", func(t *testing.T) {
		// setup makes a note and a revision that supersedes it, both
		// committed, and puts the old note's file into the state.
		setup := func(t *testing.T, s gitState) (r *repo, old, orig, now string) {
			r = newRepo(t)
			old = r.fact("Fetch retries twice")
			r.fact("Fetch retries five times", "--supersedes", old)
			r.commit("notes")
			orig = fileOf(r, old)
			now = s.apply(r, orig)
			return r, old, orig, now
		}
		for _, s := range states {
			t.Run(s.name, func(t *testing.T) {
				t.Run(saveName(s.save), func(t *testing.T) {
					t.Parallel()
					r, _, orig, now := setup(t, s)
					p := orig
					if s.atNew {
						p = now
					}
					saveCell(t, r, p, s.save, strings.NewReplacer())
				})
				t.Run("prune: "+s.notePrune, func(t *testing.T) {
					t.Parallel()
					r, old, orig, now := setup(t, s)
					res := r.gm("prune")
					switch s.notePrune {
					case "pruned":
						expect(t, res, want{exit: 0, has: []string{"pruned " + now}}, strings.NewReplacer())
						if exists(r, now) {
							t.Fatalf("want %s deleted", now)
						}
					case "kept":
						expect(t, res, want{exit: 0, has: []string{"kept " + now}, lacks: []string{"pruned .agents/memory/" + old}}, strings.NewReplacer())
						if !exists(r, now) {
							t.Fatalf("want %s kept on disk", now)
						}
					case "absent":
						expect(t, res, want{exit: 0, lacks: []string{"pruned " + orig, "kept " + orig}}, strings.NewReplacer())
					}
				})
				t.Run(lintName(s.noteLint), func(t *testing.T) {
					t.Parallel()
					r, old, _, _ := setup(t, s)
					expect(t, r.gm("lint"), s.noteLint, strings.NewReplacer("{id}", old))
				})
				t.Run(showName(s.noteShow), func(t *testing.T) {
					t.Parallel()
					r, old, _, _ := setup(t, s)
					expect(t, r.gm("show", old), s.noteShow, strings.NewReplacer())
				})
			})
		}
		t.Run("in a submodule that holds .agents/memory", func(t *testing.T) {
			// setup makes .agents/memory a submodule whose root holds a note
			// and a revision that supersedes it, committed in the submodule.
			setup := func(t *testing.T, modify bool) (r *repo, old, p string) {
				sub := newRepo(t)
				old = sub.fact("Fetch retries twice")
				sub.fact("Fetch retries five times", "--supersedes", old)
				sub.commit("notes")
				for _, f := range sub.notes() {
					sub.git("mv", f, filepath.Base(f))
				}
				sub.commit("move notes to the root")
				r = newRepo(t)
				r.git("-c", "protocol.file.allow=always", "submodule", "add", "-q", "file://"+sub.dir, ".agents/memory")
				r.git("commit", "-qm", "memory submodule")
				p = fileOf(r, old)
				if modify {
					edit(r, p)
				}
				return r, old, p
			}
			t.Run("modified: save --at refuses: not committed at HEAD", func(t *testing.T) {
				t.Parallel()
				r, _, p := setup(t, true)
				saveCell(t, r, p, notAtHead, strings.NewReplacer())
			})
			t.Run("modified: prune keeps the note, which no commit holds as it is", func(t *testing.T) {
				t.Parallel()
				r, _, p := setup(t, true)
				expect(t, r.gm("prune"), want{exit: 0, has: []string{"kept " + p}}, strings.NewReplacer())
				if !exists(r, p) {
					t.Fatalf("want %s kept on disk", p)
				}
			})
			t.Run("modified: lint is clean", func(t *testing.T) {
				t.Parallel()
				r, _, _ := setup(t, true)
				expect(t, r.gm("lint"), cleanLint, strings.NewReplacer())
			})
			t.Run("modified: show prints the note file", func(t *testing.T) {
				t.Parallel()
				r, old, _ := setup(t, true)
				expect(t, r.gm("show", old), noteEdited, strings.NewReplacer())
			})
			t.Run("clean: prune does not crash (manual silent)", func(t *testing.T) {
				t.Parallel()
				r, _, _ := setup(t, false)
				expect(t, r.gm("prune"), want{exit: silent}, strings.NewReplacer())
			})
		})
	})
}

// saveArgs returns the args of a sound gm save fact, with each flag in over
// replacing the default value, followed by extra.
func saveArgs(over map[string]string, extra ...string) []string {
	vals := map[string]string{"title": "Fetch retries", "keywords": "fetch retry backoff", "fact": "f", "evidence": "e"}
	for k, v := range over {
		vals[k] = v
	}
	args := []string{"save", "fact"}
	for _, k := range []string{"title", "keywords", "topic", "fact", "evidence"} {
		if v, ok := vals[k]; ok {
			args = append(args, "--"+k, v)
		}
	}
	return append(args, extra...)
}

// showJSON returns the fields that gm show --json prints for id.
func showJSON(t *testing.T, r *repo, id string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(r.ok("show", id, "--json")), &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func matrixInput(t *testing.T) {
	refuse := func(s string) want { return want{exit: 2, has: []string{s}} }
	type saveCase struct {
		name  string
		over  map[string]string
		extra func(r *repo) []string
		w     want
		post  func(t *testing.T, r *repo, id string, over map[string]string)
	}
	titleKept := func(t *testing.T, r *repo, id string, over map[string]string) {
		if got := showJSON(t, r, id)["title"]; got != over["title"] {
			t.Fatalf("want title %q read back, got %q", over["title"], got)
		}
		expect(t, r.gm("lint"), cleanLint, strings.NewReplacer())
	}
	factKept := func(t *testing.T, r *repo, id string, over map[string]string) {
		has(t, r.ok("show", id), over["fact"])
		expect(t, r.gm("lint"), cleanLint, strings.NewReplacer())
	}
	ascii, multi := func(n int) string { return strings.Repeat("t", n) }, func(n int) string { return strings.Repeat("日", n) }
	saves := map[string][]saveCase{
		"title": {
			{name: "empty is refused", over: map[string]string{"title": ""}, w: refuse("title")},
			{name: "whitespace only is 3 characters on one line, so accepted", over: map[string]string{"title": "   "}, w: accept, post: titleKept},
			{name: "1 character is accepted", over: map[string]string{"title": "x"}, w: accept, post: titleKept},
			{name: "120 ASCII characters are accepted", over: map[string]string{"title": ascii(120)}, w: accept, post: titleKept},
			{name: "121 ASCII characters are refused", over: map[string]string{"title": ascii(121)}, w: refuse("title")},
			{name: "120 multibyte characters are accepted", over: map[string]string{"title": multi(120)}, w: accept, post: titleKept},
			{name: "121 multibyte characters are refused", over: map[string]string{"title": multi(121)}, w: refuse("title")},
			{name: "quotes are kept", over: map[string]string{"title": `Fetch "retries" 'twice'`}, w: accept, post: titleKept},
			{name: "backslashes are kept", over: map[string]string{"title": `C:\fetch\retry \n`}, w: accept, post: titleKept},
			{name: "a newline is refused", over: map[string]string{"title": "Fetch\nretries"}, w: refuse("title")},
			{name: "a leading --- is kept", over: map[string]string{"title": "--- not a header"}, w: accept, post: titleKept},
		},
		"keywords": {
			{name: "0 words are refused", over: map[string]string{"keywords": ""}, w: refuse("at least 3 words")},
			{name: "2 words are refused", over: map[string]string{"keywords": "fetch retry"}, w: refuse("at least 3 words")},
			{name: "3 words are accepted", over: map[string]string{"keywords": "fetch retry backoff"}, w: accept},
			{name: "a newline is refused", over: map[string]string{"keywords": "fetch retry\nbackoff"}, w: refuse("keywords")},
		},
		"topic": {
			{name: "a space is refused", over: map[string]string{"topic": "fact/fetch retry"}, w: refuse("topic")},
			{name: "a tab is refused", over: map[string]string{"topic": "fact/fetch\tretry"}, w: refuse("topic")},
			{name: "empty is accepted and left out of the header", over: map[string]string{"topic": ""}, w: accept,
				post: func(t *testing.T, r *repo, id string, _ map[string]string) { lacks(t, r.read(fileOf(r, id)), "topic:") }},
		},
		"section": {
			{name: "empty is refused", over: map[string]string{"fact": ""}, w: refuse("section Fact is empty")},
			{name: "1 character is accepted", over: map[string]string{"fact": "x"}, w: accept, post: factKept},
			{name: "800 ASCII characters are accepted", over: map[string]string{"fact": ascii(800)}, w: accept, post: factKept},
			{name: "801 ASCII characters are refused", over: map[string]string{"fact": ascii(801)}, w: refuse("over 800")},
			{name: "800 multibyte characters are accepted", over: map[string]string{"fact": multi(800)}, w: accept, post: factKept},
			{name: "801 multibyte characters are refused", over: map[string]string{"fact": multi(801)}, w: refuse("over 800")},
			{name: "a line starting ## is refused", over: map[string]string{"fact": "Fetch retries.\n## Evidence\nsmuggled"}, w: refuse(`"## "`)},
			{name: "a line ## without a space is accepted", over: map[string]string{"fact": "Fetch retries.\n##\nmore"}, w: accept, post: factKept},
			{name: "a line starting --- is accepted", over: map[string]string{"fact": "Fetch retries.\n---\nmore"}, w: accept, post: factKept},
			{name: "a first line --- is accepted", over: map[string]string{"fact": "---\nmore"}, w: accept, post: factKept},
		},
		"supersedes": {
			{name: "an unknown id is refused", extra: func(r *repo) []string { return []string{"--supersedes", "20200101T000000Z-000000"} },
				w: refuse("supersedes unknown note 20200101T000000Z-000000")},
			{name: "a superseded id is accepted", extra: func(r *repo) []string {
				old := r.fact("Fetch retries twice")
				r.fact("Fetch retries thrice", "--supersedes", old)
				return []string{"--supersedes", old}
			}, w: accept},
			{name: "a pruned id is accepted", extra: func(r *repo) []string {
				old := r.fact("Fetch retries twice")
				r.fact("Fetch retries thrice", "--supersedes", old)
				r.commit("notes")
				r.ok("prune")
				r.commit("prune")
				return []string{"--supersedes", old}
			}, w: accept},
			{name: "a retirement's id is accepted", extra: func(r *repo) []string {
				ret := strings.Fields(r.ok("retire", r.fact("Fetch retries twice"), "--why", "gone"))[0]
				return []string{"--supersedes", ret}
			}, w: accept},
			{name: "the same id twice is accepted", extra: func(r *repo) []string {
				old := r.fact("Fetch retries twice")
				return []string{"--supersedes", old, "--supersedes", old}
			}, w: accept, post: func(t *testing.T, r *repo, _ string, _ map[string]string) {
				expect(t, r.gm("lint"), cleanLint, strings.NewReplacer())
			}},
			{name: "a malformed id is refused", extra: func(r *repo) []string { return []string{"--supersedes", "not-an-id"} },
				w: refuse("supersedes unknown note not-an-id")},
		},
	}
	for _, group := range []string{"title", "keywords", "topic", "section", "supersedes"} {
		t.Run("save "+group, func(t *testing.T) {
			for _, c := range saves[group] {
				t.Run(c.name, func(t *testing.T) {
					t.Parallel()
					r := newRepo(t)
					var extra []string
					if c.extra != nil {
						extra = c.extra(r)
					}
					before := len(r.notes())
					res := r.gm(saveArgs(c.over, extra...)...)
					expect(t, res, c.w, strings.NewReplacer())
					if res.code == 2 && len(r.notes()) != before {
						t.Fatal("a refused save wrote a note")
					}
					if c.post != nil {
						c.post(t, r, strings.Fields(res.stdout)[0], c.over)
					}
				})
			}
			if group == "supersedes" {
				t.Run("the note's own new id cannot be named before it exists", func(t *testing.T) {
					t.Skip("impossible: the id is made when save runs")
				})
			}
		})
	}

	t.Run("retire", func(t *testing.T) {
		cases := []struct {
			name string
			args func(r *repo) []string
			w    want
			post func(t *testing.T, r *repo, out string)
		}{
			{"no ids are refused", func(r *repo) []string { return []string{"--why", "gone"} }, want{exit: 2}, nil},
			{"an unknown id is refused", func(r *repo) []string { return []string{"20200101T000000Z-000000", "--why", "gone"} },
				refuse("20200101T000000Z-000000"), nil},
			{"a malformed id is refused", func(r *repo) []string { return []string{"not-an-id", "--why", "gone"} }, refuse("not-an-id"), nil},
			{"a retirement is accepted", func(r *repo) []string {
				ret := strings.Fields(r.ok("retire", r.fact("Fetch retries twice"), "--why", "gone"))[0]
				return []string{ret, "--why", "the retirement was wrong"}
			}, accept, nil},
			{"a superseded id is accepted", func(r *repo) []string {
				old := r.fact("Fetch retries twice")
				r.fact("Fetch retries thrice", "--supersedes", old)
				return []string{old, "--why", "gone"}
			}, accept, nil},
			{"the same id twice counts once", func(r *repo) []string {
				a := r.fact("Fetch retries twice")
				return []string{a, a, "--why", "gone"}
			}, accept, func(t *testing.T, r *repo, out string) {
				for _, line := range strings.Split(r.read(strings.Fields(out)[1]), "\n") {
					if strings.HasPrefix(line, "supersedes: ") && strings.Contains(line, ",") {
						t.Fatalf("want one supersedes entry, got %q", line)
					}
				}
			}},
			{"missing --why is refused", func(r *repo) []string { return []string{r.fact("Fetch retries twice")} }, refuse("--why"), nil},
			{"an empty --why is refused", func(r *repo) []string { return []string{r.fact("Fetch retries twice"), "--why", ""} }, want{exit: 2}, nil},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				t.Parallel()
				r := newRepo(t)
				args := c.args(r)
				before := len(r.notes())
				res := r.gm(append([]string{"retire"}, args...)...)
				expect(t, res, c.w, strings.NewReplacer())
				if res.code == 2 && len(r.notes()) != before {
					t.Fatal("a refused retire wrote a note")
				}
				if c.post != nil {
					c.post(t, r, res.stdout)
				}
			})
		}
	})

	t.Run("-n", func(t *testing.T) {
		six := func(t *testing.T) *repo {
			r := newRepo(t)
			for i := 1; i <= 6; i++ {
				r.fact("Fetch note " + strconv.Itoa(i))
			}
			return r
		}
		// count -1 means gm must refuse with exit 2.
		cases := []struct {
			n            string
			search, list int
		}{
			{"-1", -1, -1}, {"0", 0, 0}, {"1", 1, 1}, {"5", 5, 5}, {"1000000000", 6, 6},
			{"99999999999999999999", 6, 6}, {"abc", -1, -1}, {"", -1, -1},
		}
		for _, cmd := range []string{"search", "list"} {
			for _, c := range cases {
				n := c.search
				if cmd == "list" {
					n = c.list
				}
				name := cmd + " -n " + strconv.Quote(c.n) + " prints " + strconv.Itoa(n) + " notes"
				if n < 0 {
					name = cmd + " -n " + strconv.Quote(c.n) + " is an error"
				}
				t.Run(name, func(t *testing.T) {
					t.Parallel()
					r := six(t)
					args := []string{cmd, "-n", c.n, "--json"}
					if cmd == "search" {
						args = append(args, "fetch")
					}
					res := r.gm(args...)
					if n < 0 {
						expect(t, res, want{exit: 2}, strings.NewReplacer())
						return
					}
					expect(t, res, want{exit: 0}, strings.NewReplacer())
					if got := len(ids(t, res.stdout)); got != n {
						t.Fatalf("want %d notes, got %d:\n%s", n, got, res.stdout)
					}
				})
			}
		}
		t.Run("search without -n prints 5 notes", func(t *testing.T) {
			t.Parallel()
			if got := len(ids(t, six(t).ok("search", "fetch", "--json"))); got != 5 {
				t.Fatalf("want 5, got %d", got)
			}
		})
		t.Run("list without -n prints all notes", func(t *testing.T) {
			t.Parallel()
			if got := len(ids(t, six(t).ok("list", "--json"))); got != 6 {
				t.Fatalf("want 6, got %d", got)
			}
		})
	})

	t.Run("flags", func(t *testing.T) {
		// base makes a repository with one anchored note and returns its id.
		base := func(t *testing.T) (*repo, string) {
			r := newRepo(t)
			return r, r.fact("Fetch retries twice", "--at", "fetch.go:3")
		}
		unknown := map[string]func(id string) []string{
			"save":   func(string) []string { return saveArgs(nil, "--bogus", "x") },
			"retire": func(id string) []string { return []string{"retire", id, "--why", "w", "--bogus"} },
			"search": func(string) []string { return []string{"search", "fetch", "--bogus"} },
			"show":   func(id string) []string { return []string{"show", id, "--bogus"} },
			"lint":   func(string) []string { return []string{"lint", "--bogus"} },
			"list":   func(string) []string { return []string{"list", "--bogus"} },
			"prune":  func(string) []string { return []string{"prune", "--bogus"} },
			"import": func(string) []string { return []string{"import", "engram", "x.json", "--bogus"} },
		}
		for _, cmd := range []string{"save", "retire", "search", "show", "lint", "list", "prune", "import"} {
			t.Run(cmd+" refuses an unknown flag", func(t *testing.T) {
				t.Parallel()
				r, id := base(t)
				before := len(r.notes())
				expect(t, r.gm(unknown[cmd](id)...), want{exit: 2}, strings.NewReplacer())
				if len(r.notes()) != before {
					t.Fatal("wrote a note")
				}
			})
		}
		jsonKeys := map[string]struct {
			args func(id string) []string
			keys []string
		}{
			"search": {func(string) []string { return []string{"search", "fetch", "--json"} }, []string{"id", "type", "created", "title", "file", "score"}},
			"show":   {func(id string) []string { return []string{"show", id, "--json"} }, []string{"id", "type", "title", "created", "code"}},
			"lint":   {func(string) []string { return []string{"lint", "--json"} }, []string{"problems"}},
			"list":   {func(string) []string { return []string{"list", "--json"} }, []string{"id", "type", "title", "created"}},
		}
		for _, cmd := range []string{"search", "show", "lint", "list"} {
			t.Run(cmd+" --json prints the documented fields", func(t *testing.T) {
				t.Parallel()
				r, id := base(t)
				out := r.ok(jsonKeys[cmd].args(id)...)
				var obj map[string]any
				if strings.HasPrefix(strings.TrimSpace(out), "[") {
					var xs []map[string]any
					if err := json.Unmarshal([]byte(out), &xs); err != nil || len(xs) == 0 {
						t.Fatalf("want a non-empty JSON array, got %v:\n%s", err, out)
					}
					obj = xs[0]
				} else if err := json.Unmarshal([]byte(out), &obj); err != nil {
					t.Fatalf("want JSON, got %v:\n%s", err, out)
				}
				for _, k := range jsonKeys[cmd].keys {
					if _, ok := obj[k]; !ok {
						t.Fatalf("want key %q in %s", k, out)
					}
				}
				switch cmd {
				case "show":
					code := obj["code"].([]any)[0].(map[string]any)
					for _, k := range []string{"anchor", "from", "lines"} {
						if _, ok := code[k]; !ok {
							t.Fatalf("want code key %q in %s", k, out)
						}
					}
				case "list":
					if b, _ := obj["body"].(string); b != "" {
						t.Fatalf("want notes without bodies, got %q", b)
					}
				case "lint":
					if _, ok := obj["problems"].([]any); !ok {
						t.Fatalf("want problems to be an array, got %s", out)
					}
				}
			})
		}
		for _, cmd := range []string{"save", "retire", "prune", "import"} {
			t.Run(cmd+" --json is not an option of "+cmd+", so it is an error", func(t *testing.T) {
				t.Parallel()
				r, id := base(t)
				args := map[string][]string{
					"save": saveArgs(nil, "--json"), "retire": {"retire", id, "--why", "w", "--json"},
					"prune": {"prune", "--json"}, "import": {"import", "engram", "x.json", "--json"},
				}[cmd]
				expect(t, r.gm(args...), want{exit: 2}, strings.NewReplacer())
			})
		}
		same := map[string][2]func(id string) []string{
			"search -n": {func(string) []string { return []string{"search", "-n", "1", "fetch"} }, func(string) []string { return []string{"search", "fetch", "-n", "1"} }},
			"search --type": {func(string) []string { return []string{"search", "--type", "fact", "fetch", "retries"} },
				func(string) []string { return []string{"search", "fetch", "retries", "--type", "fact"} }},
			"show --json": {func(id string) []string { return []string{"show", "--json", id} }, func(id string) []string { return []string{"show", id, "--json"} }},
			"list --by":   {func(string) []string { return []string{"list", "--by", "time", "-n", "1"} }, func(string) []string { return []string{"list", "-n", "1", "--by", "time"} }},
		}
		for _, name := range []string{"search -n", "search --type", "show --json", "list --by"} {
			t.Run(name+" prints the same before and after the words", func(t *testing.T) {
				t.Parallel()
				r, id := base(t)
				a, b := r.ok(same[name][0](id)...), r.ok(same[name][1](id)...)
				if a != b {
					t.Fatalf("want the same output, got\n%s\nand\n%s", a, b)
				}
			})
		}
		t.Run("retire --why before and after the ids writes the same retirement", func(t *testing.T) {
			t.Parallel()
			r, id := base(t)
			id2 := r.fact("Fetch backs off")
			a := r.read(strings.Fields(r.ok("retire", "--why", "gone", id))[1])
			b := r.read(strings.Fields(r.ok("retire", id2, "--why", "gone"))[1])
			has(t, a, "supersedes: ["+id+"]", "## Why\ngone")
			has(t, b, "supersedes: ["+id2+"]", "## Why\ngone")
		})
		t.Run("save takes flags in any order", func(t *testing.T) {
			t.Parallel()
			r, _ := base(t)
			r.ok("save", "fact", "--evidence", "e", "--fact", "f", "--keywords", "a b c", "--title", "Reordered")
		})
		t.Run("save with the type after the flags does not crash (manual silent)", func(t *testing.T) {
			t.Parallel()
			r, _ := base(t)
			expect(t, r.gm("save", "--title", "T", "--keywords", "a b c", "--fact", "f", "--evidence", "e", "fact"), want{exit: silent}, strings.NewReplacer())
		})
		t.Run("save refuses an unquoted multi-word title", func(t *testing.T) {
			t.Parallel()
			r, _ := base(t)
			before := len(r.notes())
			expect(t, r.gm("save", "fact", "--title", "Fetch", "retries", "--keywords", "a b c", "--fact", "f", "--evidence", "e"), refuse("quote"), strings.NewReplacer())
			if len(r.notes()) != before {
				t.Fatal("wrote a note")
			}
		})
		t.Run("save refuses unquoted keywords", func(t *testing.T) {
			t.Parallel()
			r, _ := base(t)
			expect(t, r.gm("save", "fact", "--title", "T", "--keywords", "fetch", "retry", "backoff", "--fact", "f", "--evidence", "e"), refuse("quote"), strings.NewReplacer())
		})
		t.Run("retire refuses an unquoted multi-word --why, whose words name no note", func(t *testing.T) {
			t.Parallel()
			r, id := base(t)
			before := len(r.notes())
			expect(t, r.gm("retire", id, "--why", "no", "longer", "holds"), want{exit: 2}, strings.NewReplacer())
			if len(r.notes()) != before {
				t.Fatal("wrote a note")
			}
		})
		t.Run("show refuses a second word", func(t *testing.T) {
			t.Parallel()
			r, id := base(t)
			expect(t, r.gm("show", id, "extra"), want{exit: 2}, strings.NewReplacer())
		})
		t.Run("search takes unquoted words as the query", func(t *testing.T) {
			t.Parallel()
			r, id := base(t)
			expect(t, r.gm("search", "fetch", "retries", "twice"), want{exit: 0, has: []string{id}}, strings.NewReplacer())
		})
	})

	t.Run("note files on disk", func(t *testing.T) {
		const id = "20200101T000000Z-aaaaaa"
		sound := "## Fact\nf\n\n## Evidence\ne\n"
		head := func(title string) string {
			return "---\nid: " + id + "\ntype: fact\ntitle: \"" + title + "\"\nkeywords: proxy budget limit\ncreated: 2020-01-01T00:00:00Z\n"
		}
		file := ".agents/memory/" + id + "-hand.md"
		type files map[string]string
		absent := want{exit: 0, lacks: []string{id}}
		found := want{exit: 0, has: []string{id}}
		gone := want{exit: 2}
		cases := []struct {
			name                    string
			files                   files
			lint, search, show, lst want
		}{
			{"no header", files{file: "proxy text\n"},
				want{exit: 1, has: []string{file}}, absent, gone, absent},
			{"unterminated header", files{file: head("Proxy bad") + sound},
				want{exit: 1, has: []string{file}}, absent, gone, absent},
			{"header without id", files{file: strings.Replace(head("Proxy bad"), "id: "+id+"\n", "", 1) + "---\n" + sound},
				want{exit: 1, has: []string{file}}, want{exit: 0, lacks: []string{"Proxy bad"}}, gone, want{exit: 0, lacks: []string{"Proxy bad"}}},
			{"duplicate ids", files{file: head("Proxy one") + "---\n" + sound, ".agents/memory/" + id + "-copy.md": head("Proxy two") + "---\n" + sound},
				want{exit: 1, has: []string{"id " + id + " is used by 2 notes"}}, want{exit: silent}, want{exit: silent}, want{exit: silent}},
			{"CRLF line ends", files{file: strings.ReplaceAll(head("Proxy crlf")+"---\n"+sound, "\n", "\r\n")},
				cleanLint, found, want{exit: 0, has: []string{"Proxy crlf"}}, found},
			{"a format: 2 key", files{file: head("Proxy future") + "format: 2\n---\n" + sound},
				want{exit: 1, has: []string{file, "note format 2 needs a newer gm"}}, absent, gone, absent},
			{"unknown keys", files{file: head("Proxy keys") + "color: blue\nowner: x\n---\n" + sound},
				cleanLint, found, want{exit: 0, has: []string{"Proxy keys"}}, found},
			{"a non-.md file", files{".agents/memory/" + id + "-hand.txt": head("Proxy text") + "---\n" + sound},
				cleanLint, absent, gone, absent},
			{"an empty file", files{file: ""},
				want{exit: 1, has: []string{file}}, absent, gone, absent},
			{"a very large note (1 MB)", files{file: head("Proxy huge") + "---\n## Fact\nf\n\n## Evidence\n" + strings.Repeat("proxy budget ", 1<<20/13) + "\n"},
				want{exit: 1, has: []string{"over 800"}}, found, want{exit: 0, has: []string{"Proxy huge"}}, found},
			{"invalid UTF-8", files{file: head("Proxy \xff\xfe bad") + "---\n## Fact\n\xc3\x28 \xff\n\n## Evidence\ne\n"},
				want{exit: silent}, want{exit: silent}, want{exit: silent}, want{exit: silent}},
		}
		for _, c := range cases {
			setup := func(t *testing.T) (*repo, string) {
				r := newRepo(t)
				soundID := r.fact("Proxy sound", "--keywords", "proxy budget limit")
				for name, content := range c.files {
					r.write(name, content)
				}
				return r, soundID
			}
			withSound := func(w want, soundID string) want {
				if w.exit == 0 && len(w.has)+len(w.lacks) > 0 {
					w.has = append(append([]string{}, w.has...), soundID)
				}
				return w
			}
			name := func(cmd string, w want) string {
				switch {
				case w.exit == silent:
					return cmd + " does not crash (manual silent)"
				case cmd == "lint" && w.exit == 1:
					return "lint fails"
				case cmd == "lint":
					return "lint is clean"
				case cmd == "show" && w.exit == 2:
					return "show finds no note"
				case cmd == "show":
					return "show prints the note"
				case len(w.has) > 0:
					return cmd + " includes the note"
				}
				return cmd + " leaves the file out"
			}
			t.Run(c.name, func(t *testing.T) {
				t.Run(name("lint", c.lint), func(t *testing.T) {
					t.Parallel()
					r, _ := setup(t)
					expect(t, r.gm("lint"), c.lint, strings.NewReplacer())
				})
				t.Run(name("search", c.search), func(t *testing.T) {
					t.Parallel()
					r, soundID := setup(t)
					expect(t, r.gm("search", "proxy", "-n", "10"), withSound(c.search, soundID), strings.NewReplacer())
				})
				t.Run(name("show", c.show), func(t *testing.T) {
					t.Parallel()
					r, _ := setup(t)
					expect(t, r.gm("show", id), c.show, strings.NewReplacer())
				})
				t.Run(name("list", c.lst), func(t *testing.T) {
					t.Parallel()
					r, soundID := setup(t)
					expect(t, r.gm("list"), withSound(c.lst, soundID), strings.NewReplacer())
				})
			})
		}
	})
}
