package main

import (
	"testing"
)

func TestLint(t *testing.T) {
	lint := func(r *repo, args ...string) result { return r.gm(append([]string{"lint"}, args...)...) }
	fails := func(t *testing.T, res result, want ...string) {
		t.Helper()
		if res.code != 1 {
			t.Fatalf("want exit 1, got %s", res)
		}
		has(t, res.stdout, want...)
	}
	sound := "## Fact\nf\n\n## Evidence\ne\n"
	t.Run("exits 0 and prints nothing when every note is sound", func(t *testing.T) {
		r := newRepo(t)
		r.fact("Fetch", "--at", "fetch.go")
		if out := r.ok("lint"); out != "" {
			t.Fatalf("want no output, got %q", out)
		}
	})
	t.Run("reports a file that is not a note", func(t *testing.T) {
		r := newRepo(t)
		r.write(".memory/notes/stray.md", "hello\n")
		fails(t, lint(r), "stray.md: no header")
	})
	t.Run("reports an id used by more than one file", func(t *testing.T) {
		r := newRepo(t)
		head := "type: fact\ntitle: \"T\"\nkeywords: a b c\ncreated: 2020-01-01T00:00:00Z\n"
		r.note("20200101T000000Z-aaaaaa", head, sound)
		r.write(".memory/notes/20200101T000000Z-aaaaaa-copy.md", "---\nid: 20200101T000000Z-aaaaaa\n"+head+"---\n"+sound)
		fails(t, lint(r), "id 20200101T000000Z-aaaaaa is used by 2 notes")
	})
	t.Run("reports a supersedes entry that names no note", func(t *testing.T) {
		r := newRepo(t)
		r.note("20200101T000000Z-aaaaaa", "type: fact\ntitle: \"T\"\nkeywords: a b c\nsupersedes: [20190101T000000Z-000000]\ncreated: 2020-01-01T00:00:00Z\n", sound)
		fails(t, lint(r), "supersedes unknown note 20190101T000000Z-000000")
	})
	t.Run("reports a live note that breaks a rule for a new note", func(t *testing.T) {
		r := newRepo(t)
		r.note("20200101T000000Z-aaaaaa", "type: fact\ntitle: \"T\"\nkeywords: a b\ncreated: 2020-01-01T00:00:00Z\n", "## Fact\nf\n")
		fails(t, lint(r), "at least 3 words", "section Evidence is empty")
	})
	t.Run("checks a migrated note only for title, keywords and topic", func(t *testing.T) {
		r := newRepo(t)
		r.note("20200101T000000Z-aaaaaa", "type: fact\ntitle: \"T\"\ncreated: 2020-01-01T00:00:00Z\nmigrated: engram obs-1 (discovery)\n", "free text\n")
		r.ok("lint")
		r.note("20200101T000000Z-bbbbbb", "type: fact\ntitle: \"T\"\ncreated: 2020-01-01T00:00:00Z\ntopic: a b\nmigrated: engram obs-2 (discovery)\n", "free text\n")
		fails(t, lint(r), "topic one word")
	})
	t.Run("reports an anchor whose file is gone when the path holds a space", func(t *testing.T) {
		r := newRepo(t)
		r.write("my file.go", "package x\n")
		r.commit("spaced")
		r.fact("Spaced", "--at", "my file.go")
		r.commit("note")
		r.git("rm", "-q", "my file.go")
		r.commit("remove")
		fails(t, lint(r), "my file.go: the file no longer exists at HEAD")
	})
	t.Run("reports an anchor whose file is gone, and names the new path after a rename", func(t *testing.T) {
		r := newRepo(t)
		r.fact("Fetch lives here", "--at", "fetch.go")
		r.commit("note")
		r.git("mv", "fetch.go", "client.go")
		r.commit("move")
		fails(t, lint(r), "moved to client.go")
		r.git("rm", "-q", "client.go")
		r.commit("delete")
		fails(t, lint(r), "the file no longer exists at HEAD")
	})
	t.Run("after a squash or a rebase merge, reads anchors at the commit that added the note", func(t *testing.T) {
		for _, strategy := range []string{"squash", "rebase"} {
			clone, id := merged(t, strategy)
			if out := clone.ok("lint"); out != "" {
				t.Fatalf("%s: want a clean lint, got %q", strategy, out)
			}
			added := clone.git("log", "--diff-filter=A", "--format=%h", "--abbrev=12", "--", ".memory/notes")
			has(t, clone.ok("show", id), ":fetch.go:3 (read at "+added+", the commit that added the note)\n    3  func Fetch() { retry() }\n")
			clone.write("fetch.go", "// Package x.\npackage x\n\nfunc Fetch() { retry() }\n")
			clone.commit("comment above")
			if out := clone.ok("lint", "--review"); out != "" {
				t.Fatalf("%s: want no review after lines above moved, got %q", strategy, out)
			}
			clone.write("fetch.go", "// Package x.\npackage x\n\nfunc Fetch() { retry(5) }\n")
			clone.commit("edit")
			has(t, clone.ok("lint", "--review"), "review: ", ":fetch.go:3")
		}
	})
	t.Run("reads an anchor whose commit is missing at the commit that added the note", func(t *testing.T) {
		r := newRepo(t)
		r.note("20200101T000000Z-aaaaaa", "type: fact\ntitle: \"T\"\nkeywords: a b c\nat: [deadbeefdead:fetch.go:3]\ncreated: 2020-01-01T00:00:00Z\n", sound)
		r.commit("note")
		if out := r.ok("lint"); out != "" {
			t.Fatalf("want a clean lint, got %q", out)
		}
		has(t, r.ok("show", "20200101T000000Z-aaaaaa"), "    3  func Fetch() {}\n")
	})
	t.Run("fails while a note on a branch not yet in the default branch describes code that changed after it", func(t *testing.T) {
		r := newRepo(t)
		r.git("checkout", "-qb", "feat")
		r.fact("Fetch is empty", "--at", "fetch.go:3")
		r.commit("note")
		r.write("fetch.go", "package x\n\nfunc Fetch() { retry() }\n")
		r.commit("change after the note")
		fails(t, lint(r), ":fetch.go:3: the anchored code changed after the note")
		r.git("checkout", "-q", "main")
		r.git("merge", "-q", "--no-ff", "--no-edit", "feat")
		has(t, r.ok("lint", "--review"), "review: ", ":fetch.go:3")
	})
	t.Run("on a branch lists a whole-file anchor whose file changed after the note, without failing", func(t *testing.T) {
		r := newRepo(t)
		r.git("checkout", "-qb", "feat")
		r.fact("Fetch lives here", "--at", "fetch.go")
		r.commit("note")
		r.write("fetch.go", "package x\n\nfunc Fetch() { retry() }\n")
		r.commit("change after the note")
		has(t, r.ok("lint", "--review"), "review: ", ":fetch.go changed")
	})
	t.Run("in a shallow clone checks anchored files and warns about the commits it cannot see", func(t *testing.T) {
		r := newRepo(t)
		r.fact("Fetch lives here", "--at", "fetch.go")
		r.commit("note")
		r.write("other.go", "package x\n")
		r.commit("later")
		clone := &repo{t, t.TempDir()}
		clone.git("clone", "-q", "--depth", "1", "file://"+r.dir, ".")
		res := lint(clone)
		if res.code != 0 {
			t.Fatalf("want exit 0, got %s", res)
		}
		has(t, res.stdout, "warning: shallow clone: 1 anchor commits")
		clone.git("rm", "-q", "fetch.go")
		clone.commit("delete")
		fails(t, lint(clone), "the file no longer exists at HEAD")
	})
	t.Run("--review lists an anchor whose lines changed since, without failing", func(t *testing.T) {
		r := newRepo(t)
		r.fact("Fetch is empty", "--at", "fetch.go:3")
		r.write("fetch.go", "package x\n\nfunc Fetch() { retry() }\n")
		r.commit("retry")
		out := r.ok("lint", "--review")
		has(t, out, "review: ", ":fetch.go:3")
	})
	t.Run("--review follows the anchored lines when lines above them move", func(t *testing.T) {
		r := newRepo(t)
		r.fact("Fetch is empty", "--at", "fetch.go:3")
		r.write("fetch.go", "// Package x.\npackage x\n\nfunc Fetch() {}\n")
		r.commit("comment above")
		if out := r.ok("lint", "--review"); out != "" {
			t.Fatalf("want no review after lines above moved, got %q", out)
		}
		r.write("fetch.go", "// Package x.\npackage x\n\nfunc Fetch() { retry() }\n")
		r.commit("retry")
		has(t, r.ok("lint", "--review"), "review: ", ":fetch.go:3")
	})
	t.Run("--review lists a whole-file anchor when the file changed", func(t *testing.T) {
		r := newRepo(t)
		r.fact("Fetch", "--at", "fetch.go")
		if out := r.ok("lint", "--review"); out != "" {
			t.Fatalf("want no review before a change, got %q", out)
		}
		r.write("fetch.go", "package x\n\nfunc Fetch() { retry() }\n")
		r.commit("retry")
		has(t, r.ok("lint", "--review"), "review: ", ":fetch.go ")
	})
	t.Run("--json prints problems always, and review and warnings", func(t *testing.T) {
		r := newRepo(t)
		r.fact("Fetch", "--at", "fetch.go")
		has(t, r.ok("lint", "--json"), `"problems": []`)
		r.write("fetch.go", "package x\n\nfunc Fetch() { retry() }\n")
		r.commit("retry")
		has(t, r.ok("lint", "--review", "--json"), `"problems": []`, `"review": [`)
	})
}
