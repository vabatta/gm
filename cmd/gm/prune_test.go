package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// pruned makes a repository with a note, a revision that supersedes it, and a
// retirement, all committed, and returns the ids.
func pruned(t *testing.T) (r *repo, old, revision, retirement string) {
	t.Helper()
	r = newRepo(t)
	old = r.fact("Fetch retries twice")
	revision = r.fact("Fetch retries five times", "--supersedes", old)
	retirement = strings.Fields(r.ok("retire", r.fact("Fetch uses a proxy"), "--why", "The proxy is gone; fetch calls the upstream directly"))[0]
	r.commit("notes")
	return r, old, revision, retirement
}

func TestPruning(t *testing.T) {
	t.Run("deletes the file of every note that another note supersedes and prints each path", func(t *testing.T) {
		r, old, _, _ := pruned(t)
		out := r.ok("prune")
		if strings.Count(out, "pruned .agents/memory/") != 2 || !strings.Contains(out, "pruned .agents/memory/"+old+"-") {
			t.Fatalf("want the revised note and the retired note pruned, got\n%s", out)
		}
		if n := len(r.notes()); n != 2 {
			t.Fatalf("want the revision and the retirement left, got %d files", n)
		}
	})
	t.Run("keeps live notes, retirements included", func(t *testing.T) {
		r, _, revision, retirement := pruned(t)
		r.ok("prune")
		live := ids(t, r.ok("list", "--json"))
		slices.Sort(live)
		want := []string{retirement, revision}
		slices.Sort(want)
		equal(t, live, want)
	})
	t.Run("keeps a superseded note whose file is not committed as it is at HEAD", func(t *testing.T) {
		r := newRepo(t)
		old := r.fact("Fetch retries twice")
		r.fact("Fetch retries five times", "--supersedes", old)
		out := r.ok("prune")
		has(t, out, "kept .agents/memory/"+old+"-")
		lacks(t, out, "pruned ")
		if n := len(r.notes()); n != 2 {
			t.Fatalf("want both notes kept, got %d", n)
		}
	})
	t.Run("show reads a pruned note from git history and names the commit that pruned it", func(t *testing.T) {
		r, old, _, _ := pruned(t)
		r.ok("prune")
		r.commit("prune")
		out := r.ok("show", old)
		has(t, out, "# pruned in "+r.head()+"\n", "title: \"Fetch retries twice\"")
		has(t, r.ok("show", old, "--json"), `"pruned_in": "`+r.head()+`"`)
	})
	t.Run("search --all and list --all include pruned notes, marked with the commit that pruned them", func(t *testing.T) {
		r, old, _, _ := pruned(t)
		r.ok("prune")
		r.commit("prune")
		lacks(t, r.ok("search", "fetch", "retries"), old)
		has(t, r.ok("search", "fetch", "retries", "--all"), old, "(superseded, pruned in "+r.head()+")")
		has(t, r.ok("search", "fetch", "retries", "--all", "--json"), `"pruned_in": "`+r.head()+`"`)
		has(t, r.ok("list", "--all"), old)
	})
	t.Run("lint accepts a supersedes entry that names a pruned note", func(t *testing.T) {
		r, _, _, _ := pruned(t)
		r.ok("prune")
		r.commit("prune")
		if out := r.ok("lint"); out != "" {
			t.Fatalf("want a clean lint, got %q", out)
		}
	})
	t.Run("keeps a superseded note whose file git ignores", func(t *testing.T) {
		r := newRepo(t)
		r.write(".gitignore", ".agents/\n")
		r.commit("ignore notes")
		old := r.fact("Fetch retries twice")
		r.fact("Fetch retries five times", "--supersedes", old)
		has(t, r.ok("prune"), "kept .agents/memory/"+old+"-")
		if n := len(r.notes()); n != 2 {
			t.Fatalf("want both notes kept, got %d", n)
		}
	})
	t.Run("reads a note pruned in the same commit that adds a similar revision", func(t *testing.T) {
		r := newRepo(t)
		body := []string{"--fact", strings.Repeat("Fetch retries with exponential backoff and jitter. ", 12), "--evidence", strings.Repeat("Measured in the staging load test. ", 12)}
		old := r.fact("Fetch retries twice", body...)
		r.commit("note")
		r.fact("Fetch retries twice, then stops", append(body, "--supersedes", old)...)
		r.ok("prune")
		r.commit("revise and prune")
		has(t, r.ok("show", old), "# pruned in "+r.head()+"\n")
		if out := r.ok("lint"); out != "" {
			t.Fatalf("want a clean lint, got %q", out)
		}
	})
	t.Run("reads a note pruned in a merge commit", func(t *testing.T) {
		r := newRepo(t)
		old := r.fact("Fetch retries twice")
		r.commit("note")
		r.git("checkout", "-qb", "feat")
		r.write("x.go", "package x\n")
		r.commit("feat")
		r.git("checkout", "-q", "main")
		r.write("y.go", "package x\n")
		r.commit("main")
		r.git("merge", "-q", "--no-ff", "--no-commit", "feat")
		r.fact("Fetch retries five times", "--supersedes", old)
		r.ok("prune")
		r.commit("merge feat, revise and prune")
		has(t, r.ok("show", old), "# pruned in "+r.head()+"\n")
		if out := r.ok("lint"); out != "" {
			t.Fatalf("want a clean lint, got %q", out)
		}
	})
	t.Run("keeps a superseded note with uncommitted changes whose file name git quotes", func(t *testing.T) {
		r := newRepo(t)
		old := r.fact("Fetch retries twice")
		r.commit("note")
		odd := ".agents/memory/" + old + "-old öne.md"
		r.git("mv", strings.TrimPrefix(r.notes()[0], r.dir+"/"), odd)
		r.commit("rename")
		r.write(odd, r.read(odd)+"\nedited\n")
		r.fact("Fetch retries five times", "--supersedes", old)
		has(t, r.ok("prune"), "kept "+odd)
		if _, err := os.Stat(filepath.Join(r.dir, odd)); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("save --supersedes and retire accept the id of a pruned note", func(t *testing.T) {
		r, old, revision, _ := pruned(t)
		r.ok("prune")
		r.commit("prune")
		r.fact("Fetch retries seven times", "--supersedes", old, "--supersedes", revision)
		r.ok("retire", old, "--why", "Fetch no longer retries")
	})
	t.Run("a note file that one branch deleted and the other left unchanged ends up deleted", func(t *testing.T) {
		r, old, _, _ := pruned(t)
		r.git("checkout", "-qb", "work")
		r.fact("Unrelated work")
		r.commit("work")
		r.git("checkout", "-q", "main")
		r.ok("prune")
		r.commit("prune")
		r.git("merge", "-q", "--no-edit", "work")
		if _, err := os.Stat(filepath.Join(r.dir, ".agents", "memory")); err != nil {
			t.Fatal(err)
		}
		for _, f := range r.notes() {
			lacks(t, f, old)
		}
		r.ok("lint")
	})
	t.Run("a shallow clone lacks the history, so show cannot find a pruned note and --all leaves it out", func(t *testing.T) {
		r, old, _, _ := pruned(t)
		r.ok("prune")
		r.commit("prune")
		clone := &repo{t, t.TempDir()}
		clone.git("clone", "-q", "--depth", "1", "file://"+r.dir, ".")
		refused(t, clone.gm("show", old), "no note")
		lacks(t, clone.ok("search", "fetch", "retries", "--all"), old)
	})
}
