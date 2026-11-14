package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/vabatta/gm/internal/gittest"
)

func gm(t *testing.T, dir string, args ...string) (string, int) {
	t.Helper()
	var out, errb bytes.Buffer
	code := run(args, dir, &out, &errb)
	return out.String() + errb.String(), code
}

func TestContextIsSilentAndSucceedsOutsideARepoOrWithoutNotes(t *testing.T) {
	if out, code := gm(t, t.TempDir(), "context"); out != "" || code != 0 {
		t.Fatalf("outside a repo: want silence and 0, got %q %d", out, code)
	}
	st := gittest.New(t)
	if out, code := gm(t, st.Git.Root, "context"); out != "" || code != 0 {
		t.Fatalf("without notes: want silence and 0, got %q %d", out, code)
	}
	gm(t, st.Git.Root, "save", "fact", "--title", "Fetch retries", "--keywords", "a b c", "--fact", "f", "--evidence", "e")
	if out, _ := gm(t, st.Git.Root, "context"); !strings.Contains(out, "Fetch retries") {
		t.Fatalf("want the note listed, got %q", out)
	}
}

func TestSaveTakesFlagsInAnyOrderAndSearchPrintsJSONForAgents(t *testing.T) {
	st := gittest.New(t)
	if out, code := gm(t, st.Git.Root, "save", "fact", "--fact", "three tries", "--title", "Fetch retries", "--evidence", "test", "--keywords", "retry backoff network", "--at", "fetch.go:3"); code != 0 {
		t.Fatalf("save: %s", out)
	}
	out, code := gm(t, st.Git.Root, "search", "retry", "--json", "-n", "3")
	var hits []struct{ ID, Title, File string }
	if code != 0 || json.Unmarshal([]byte(out), &hits) != nil || len(hits) != 1 || hits[0].Title != "Fetch retries" {
		t.Fatalf("want one JSON hit, got %q %d", out, code)
	}
	if out, code := gm(t, st.Git.Root, "show", hits[0].ID); code != 0 || !strings.Contains(out, "    3  func Fetch() {}") {
		t.Fatalf("want show to print the anchored line, got %q", out)
	}
	if out, code := gm(t, st.Git.Root, "lint", "--json"); code != 0 || !strings.Contains(out, `"problems": []`) {
		t.Fatalf("want a clean JSON lint, got %q %d", out, code)
	}
}
