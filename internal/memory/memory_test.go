package memory_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vabatta/gm/internal/gittest"
	"github.com/vabatta/gm/internal/memory"
	"github.com/vabatta/gm/internal/note"
)

func problems(st *memory.Store) string { return strings.Join(st.Lint(false).Problems, "\n") }

func decision(st *memory.Store, title string, sup ...string) (memory.Saved, error) {
	return st.Save(note.Note{Type: "decision", Title: title, Keywords: "cache store choice", Topic: "decision/cache", Supersedes: sup,
		Body: note.Body("decision", map[string]string{"Decision": title, "Why": "w", "Rejected": "r"})}, nil)
}

func TestSaveRefusesANoteWithAnEmptyTemplateSectionOrASecret(t *testing.T) {
	st := gittest.New(t)
	_, err := st.Save(note.Note{Type: "decision", Title: "Use Postgres", Keywords: "cache store postgres",
		Body: note.Body("decision", map[string]string{"Decision": "Postgres", "Why": "token = ghp_0123456789abcdefghij0123456789abcdef"})}, nil)
	if err == nil || !strings.Contains(err.Error(), "section Rejected is empty") || !strings.Contains(err.Error(), "secret (GitHub token)") {
		t.Fatalf("want one refusal naming the empty section and the secret, got %v", err)
	}
	if len(st.Files()) != 0 {
		t.Fatal("a refused note must not be written")
	}
}

func TestSecretFindsCredentialsButNotPlaceholdersOrProse(t *testing.T) {
	for s, want := range map[string]bool{
		"key AKIAABCDEFGHIJKLMNOP here":                  true,
		"-----BEGIN OPENSSH PRIVATE KEY-----":            true,
		"API_KEY=a1b2c3d4e5f6g7h8":                       true,
		"postgres://app:hunter22@db:5432/x":              true,
		"xoxb-1234567890-abcdefghij":                     true,
		"postgres://app:$PASSWORD@db/x":                  false,
		"the token refresh path retries on 401":          false,
		"password: correcthorsebatterystaple":            false,
		"set OPENAI_API_KEY in .env before running just": false,
	} {
		if _, got := note.Secret(s); got != want {
			t.Errorf("Secret(%q) = %v, want %v", s, got, want)
		}
	}
}

func TestSaveRefusesAnAnchorOnUncommittedCodeOrNoPathAndPinsHEADOtherwise(t *testing.T) {
	st := gittest.New(t)
	if _, err := gittest.Fact(st, "Fetch retries", ""); err == nil || !strings.Contains(err.Error(), "needs a path") {
		t.Fatalf("want a refusal for an empty anchor, got %v", err)
	}
	gittest.Write(t, st, "fetch.go", "package x\n\nfunc Fetch() { retry() }\n")
	if _, err := gittest.Fact(st, "Fetch retries", "fetch.go:3"); err == nil || !strings.Contains(err.Error(), "uncommitted changes") {
		t.Fatalf("want a refusal for uncommitted code, got %v", err)
	}
	gittest.Commit(t, st, "retry")
	s, err := gittest.Fact(st, "Fetch retries", "fetch.go:3")
	if err != nil {
		t.Fatal(err)
	}
	if want := gittest.Git(t, st, "rev-parse", "--short=12", "HEAD") + ":fetch.go:3"; s.Note.At[0] != want {
		t.Fatalf("want anchor %s, got %v", want, s.Note.At)
	}
}

func TestLintFailsWhenAnAnchoredFileIsGoneAndNamesItsNewPathWhenMoved(t *testing.T) {
	st := gittest.New(t)
	if _, err := gittest.Fact(st, "Fetch lives here", "fetch.go"); err != nil {
		t.Fatal(err)
	}
	gittest.Commit(t, st, "note")
	gittest.Git(t, st, "mv", "fetch.go", "client.go")
	gittest.Commit(t, st, "move")
	if p := problems(st); !strings.Contains(p, "moved to client.go") {
		t.Fatalf("want the new path named, got %q", p)
	}
	gittest.Git(t, st, "rm", "-q", "client.go")
	gittest.Commit(t, st, "delete")
	if p := problems(st); !strings.Contains(p, "no longer exists") {
		t.Fatalf("want the deleted file reported, got %q", p)
	}
}

func TestLintFailsWhenAnAnchorsCommitIsNotInTheHistoryOfHEAD(t *testing.T) {
	st := gittest.New(t)
	gittest.Git(t, st, "checkout", "-qb", "side")
	gittest.Write(t, st, "side.go", "package x\n")
	gittest.Commit(t, st, "side")
	s, err := gittest.Fact(st, "Side code", "side.go")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(st.Git.Root, s.Note.File))
	gittest.Git(t, st, "checkout", "-q", "main") // as after a squash: the note arrives without the commit it names
	gittest.Write(t, st, "side.go", "package x\n")
	gittest.Write(t, st, s.Note.File, string(b))
	if p := problems(st); !strings.Contains(p, "not in the history of HEAD") {
		t.Fatalf("want the unreachable commit reported, got %q", p)
	}
}

func TestLintFlagsParallelRevisionsOfOneTopicUntilOneNoteSupersedesBoth(t *testing.T) {
	st := gittest.New(t)
	a, _ := decision(st, "Cache uses Postgres")
	b, err := decision(st, "Cache uses Redis")
	if err != nil || len(b.Others) != 1 {
		t.Fatalf("want a warning naming the other live note, got %v %v", b.Others, err)
	}
	if p := problems(st); !strings.Contains(p, "parallel revisions") {
		t.Fatalf("want parallel revisions reported, got %q", p)
	}
	if _, err := decision(st, "Cache uses Postgres behind Redis", a.Note.ID, b.Note.ID); err != nil {
		t.Fatal(err)
	}
	if p := problems(st); p != "" {
		t.Fatalf("want a clean lint after the note that supersedes both, got %q", p)
	}
}

func TestNotesSavedOnTwoBranchesMergeWithoutConflictAndBothStayLive(t *testing.T) {
	st := gittest.New(t)
	if err := st.Init(); err != nil {
		t.Fatal(err)
	}
	gittest.Commit(t, st, "init memory")
	for _, branch := range []string{"a", "b"} {
		gittest.Git(t, st, "checkout", "-qb", branch, "main")
		if _, err := gittest.Fact(st, "Note from "+branch, "fetch.go"); err != nil {
			t.Fatal(err)
		}
		gittest.Commit(t, st, "note "+branch)
	}
	gittest.Git(t, st, "checkout", "-q", "main")
	gittest.Git(t, st, "merge", "-q", "--no-edit", "a")
	gittest.Git(t, st, "merge", "-q", "--no-edit", "b")
	ns, _ := st.Load()
	if len(memory.Live(ns)) != 2 || problems(st) != "" {
		t.Fatalf("want both notes live and a clean lint after plain merges, got %d notes, %q", len(ns), problems(st))
	}
}

func TestInitIgnoresTheIndexAndChangesNothingTheSecondTime(t *testing.T) {
	st := gittest.New(t)
	gittest.Write(t, st, ".gitignore", "bin/")
	for range 2 {
		if err := st.Init(); err != nil {
			t.Fatal(err)
		}
	}
	b, _ := os.ReadFile(filepath.Join(st.Git.Root, ".gitignore"))
	if want := "bin/\n.memory/index.sqlite\n.memory/index.sqlite.tmp*\n"; string(b) != want {
		t.Fatalf("want %q, got %q", want, b)
	}
	if fi, err := os.Stat(st.NotesDir()); err != nil || !fi.IsDir() {
		t.Fatal("want .memory/notes created")
	}
}
