package main

// Behaviour specs for gm, one test per section of gm(1) and gm-note(5), one
// subtest per sentence of the manual. Every spec runs the real gm command in
// its own process against a throwaway git repository and checks only what a
// user sees: the exit status, stdout, stderr and the files in the repository.

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestMain lets the test binary act as gm: with GM_SPEC_RUN_MAIN set it runs
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

// notes lists the note files.
func (r *repo) notes() []string {
	files, _ := filepath.Glob(filepath.Join(r.dir, ".memory", "notes", "*.md"))
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

// ok runs gm and fails the spec unless it exits 0; it returns stdout.
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

// ids returns the ids that `gm search` or `gm list` printed with --json, in order.
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

func TestCommands(t *testing.T) {
	t.Run("help, -h and --help print a summary of the commands", func(t *testing.T) {
		for _, arg := range []string{"help", "-h", "--help"} {
			res := gmIn(t, t.TempDir(), arg)
			if res.code != 0 {
				t.Fatalf("%s: %s", arg, res)
			}
			has(t, res.stdout, "gm save", "gm search", "gm lint")
		}
	})
	t.Run("--version prints the version", func(t *testing.T) {
		res := gmIn(t, t.TempDir(), "--version")
		if res.code != 0 || strings.TrimSpace(res.stdout) == "" {
			t.Fatalf("want a version, got %s", res)
		}
	})
	t.Run("an unknown command exits 2", func(t *testing.T) {
		refused(t, newRepo(t).gm("frobnicate"), "frobnicate")
	})
	t.Run("a command outside a git worktree exits 2", func(t *testing.T) {
		refused(t, gmIn(t, t.TempDir(), "list"), "git worktree")
	})
}

func TestSave(t *testing.T) {
	t.Run("writes .memory/notes/ID-SLUG.md and prints the id and the path", func(t *testing.T) {
		r := newRepo(t)
		out := r.ok("save", "fact", "--title", "Fetch retries 3 times!", "--keywords", "a b c", "--fact", "f", "--evidence", "e")
		m := regexp.MustCompile(`^(\d{8}T\d{6}Z-[0-9a-f]{6}) (\.memory/notes/\d{8}T\d{6}Z-[0-9a-f]{6}-fetch-retries-3-times\.md)\n$`).FindStringSubmatch(out)
		if m == nil || !strings.HasPrefix(m[2], ".memory/notes/"+m[1]) {
			t.Fatalf("want ID and .memory/notes/ID-SLUG.md, got %q", out)
		}
		r.read(m[2])
	})
	t.Run("renders the header and one section per flag in template order", func(t *testing.T) {
		r := newRepo(t)
		out := r.ok("save", "decision", "--rejected", "Redis", "--why", "One database", "--decision", "Postgres",
			"--title", `Cache uses "Postgres"`, "--keywords", "cache store database", "--topic", "decision/cache")
		id, file := strings.Fields(out)[0], strings.Fields(out)[1]
		got := regexp.MustCompile(`created: \S+`).ReplaceAllString(r.read(file), "created: T")
		want := "---\nid: " + id + "\ntype: decision\ntitle: \"Cache uses \\\"Postgres\\\"\"\ntopic: decision/cache\n" +
			"keywords: cache store database\ncreated: T\n---\n## Decision\nPostgres\n\n## Why\nOne database\n\n## Rejected\nRedis\n"
		if got != want {
			t.Fatalf("want\n%s\ngot\n%s", want, got)
		}
	})
	t.Run("refuses an unknown type", func(t *testing.T) {
		refused(t, newRepo(t).gm("save", "idea", "--title", "x"), "decision, lesson or fact")
	})
	t.Run("refuses a word that belongs to no flag", func(t *testing.T) {
		r := newRepo(t)
		refused(t, r.gm("save", "fact", "--title", "Fetch", "retries", "--keywords", "a b c", "--fact", "f", "--evidence", "e"), "quote")
		if len(r.notes()) != 0 {
			t.Fatal("want nothing written")
		}
	})
	t.Run("refuses a note with every broken rule at once and writes nothing", func(t *testing.T) {
		r := newRepo(t)
		res := r.gm("save", "decision", "--title", strings.Repeat("t", 121), "--keywords", "two words", "--topic", "two words",
			"--decision", "d", "--why", strings.Repeat("w", 801), "--rejected", "## not a heading")
		refused(t, res, "title", "at least 3 words", "topic", "section Why is over 800", `"## "`, "section Rejected")
		if len(r.notes()) != 0 {
			t.Fatal("want nothing written")
		}
	})
	t.Run("refuses an empty section and names its flag", func(t *testing.T) {
		refused(t, newRepo(t).gm("save", "lesson", "--title", "t", "--keywords", "a b c", "--happened", "h", "--cause", "c"), "section Rule now is empty (flag --rule)")
	})
	t.Run("refuses text that looks like a credential and names the kind, never the text", func(t *testing.T) {
		for kind, secret := range map[string]string{
			"private key":           "-----BEGIN OPENSSH PRIVATE KEY-----",
			"AWS access key":        "AKIAABCDEFGHIJKLMNOP",
			"GitHub token":          "ghp_0123456789abcdefghij0123456789abcdef",
			"API key":               "sk-ant-0123456789abcdefghijklmn",
			"JWT":                   "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NSJ9.c2lnbmF0dXJlMTIz",
			"password in a URL":     "postgres://app:hunter22@db:5432/x",
			"credential assignment": "API_KEY=a1b2c3d4e5f6g7h8",
		} {
			res := newRepo(t).gm("save", "fact", "--title", "t", "--keywords", "a b c", "--fact", "uses "+secret, "--evidence", "e")
			refused(t, res, kind)
			lacks(t, res.stdout+res.stderr, secret)
		}
	})
	t.Run("accepts placeholders and prose about credentials", func(t *testing.T) {
		r := newRepo(t)
		for _, text := range []string{"postgres://app:$PASSWORD@db/x", "the token refresh path retries on 401", "set OPENAI_API_KEY in .env"} {
			r.fact("Placeholder "+text, "--evidence", text)
		}
	})
	t.Run("refuses a supersedes id that names no note", func(t *testing.T) {
		refused(t, newRepo(t).gm("save", "fact", "--title", "t", "--keywords", "a b c", "--fact", "f", "--evidence", "e",
			"--supersedes", "20200101T000000Z-000000"), "supersedes unknown note 20200101T000000Z-000000")
	})
	t.Run("warns but saves when other live notes share the topic", func(t *testing.T) {
		r := newRepo(t)
		first := r.decision("Cache uses Postgres")
		res := r.gm("save", "decision", "--title", "Cache uses Redis", "--keywords", "cache store choice", "--topic", "decision/cache",
			"--decision", "d", "--why", "w", "--rejected", "r")
		if res.code != 0 || len(r.notes()) != 2 {
			t.Fatalf("want the note saved, got %s", res)
		}
		has(t, res.stdout+res.stderr, "topic decision/cache has other live notes: "+first)
	})
}

func TestAnchors(t *testing.T) {
	t.Run("pins the anchor to HEAD with 12 hex characters", func(t *testing.T) {
		r := newRepo(t)
		r.fact("Fetch lives here", "--at", "fetch.go:3", "--at", "./fetch.go")
		has(t, r.read(strings.TrimPrefix(r.notes()[0], r.dir+"/")), "at: ["+r.head()+":fetch.go:3, "+r.head()+":fetch.go]")
	})
	t.Run("refuses a file with uncommitted changes", func(t *testing.T) {
		r := newRepo(t)
		r.write("fetch.go", "package x\n\nfunc Fetch() { retry() }\n")
		refused(t, r.gm("save", "fact", "--title", "t", "--keywords", "a b c", "--fact", "f", "--evidence", "e", "--at", "fetch.go"), "uncommitted changes")
	})
	t.Run("refuses a file that is not committed at HEAD", func(t *testing.T) {
		r := newRepo(t)
		r.write("new.go", "package x\n")
		refused(t, r.gm("save", "fact", "--title", "t", "--keywords", "a b c", "--fact", "f", "--evidence", "e", "--at", "new.go"), "not committed at HEAD")
	})
	t.Run("refuses an empty path", func(t *testing.T) {
		refused(t, newRepo(t).gm("save", "fact", "--title", "t", "--keywords", "a b c", "--fact", "f", "--evidence", "e", "--at", ""), "needs a path")
	})
	t.Run("refuses a line range that is malformed or ends past the last line", func(t *testing.T) {
		for spec, want := range map[string]string{"fetch.go:0": "lines must be", "fetch.go:3-2": "lines must be", "fetch.go:2-9": "has 3 lines"} {
			refused(t, newRepo(t).gm("save", "fact", "--title", "t", "--keywords", "a b c", "--fact", "f", "--evidence", "e", "--at", spec), want)
		}
	})
	t.Run("takes a path containing : or , whole when it names a committed file", func(t *testing.T) {
		r := newRepo(t)
		r.write("a:b, c.txt", "1\n2\n3\n")
		r.write("log:12", "x\n")
		r.commit("odd names")
		for spec, shown := range map[string]string{"a:b, c.txt:2-3": "    2  2\n    3  3\n", "log:12": "    1  x\n"} {
			id := r.fact("Odd file "+spec, "--at", spec)
			has(t, r.ok("show", id), shown)
		}
		if out := r.ok("lint"); out != "" {
			t.Fatalf("want a clean lint, got %q", out)
		}
	})
}

func TestRevisions(t *testing.T) {
	t.Run("branches that save in parallel merge without conflicts and both notes stay live", func(t *testing.T) {
		r := newRepo(t)
		for _, b := range []string{"a", "b"} {
			r.git("checkout", "-qb", b, "main")
			r.fact("Note from "+b, "--at", "fetch.go")
			r.commit("note " + b)
		}
		r.git("checkout", "-q", "main")
		r.git("merge", "-q", "--no-edit", "a")
		r.git("merge", "-q", "--no-edit", "b")
		if n := len(ids(t, r.ok("list", "--json"))); n != 2 {
			t.Fatalf("want 2 live notes, got %d", n)
		}
		r.ok("lint")
	})
	t.Run("a revision hides the note it supersedes from search, list and context", func(t *testing.T) {
		r := newRepo(t)
		old := r.fact("Fetch retries twice")
		r.fact("Fetch retries three times", "--supersedes", old)
		for _, args := range [][]string{{"search", "fetch"}, {"list"}, {"context"}} {
			lacks(t, r.ok(args...), old)
		}
	})
	t.Run("parallel revisions of one topic fail lint until one note supersedes both", func(t *testing.T) {
		r := newRepo(t)
		a, b := r.decision("Cache uses Postgres"), r.decision("Cache uses Redis")
		res := r.gm("lint")
		if res.code != 1 {
			t.Fatalf("want exit 1, got %s", res)
		}
		has(t, res.stdout, "topic decision/cache has 2 live notes", a, b)
		r.decision("Cache uses Postgres behind Redis", a, b)
		r.ok("lint")
	})
}

func TestRetire(t *testing.T) {
	t.Run("writes a retirement that hides the retired notes", func(t *testing.T) {
		r := newRepo(t)
		a, b := r.fact("Fetch retries twice"), r.fact("Fetch backs off")
		out := r.ok("retire", a, b, "--why", "Fetch no longer retries")
		has(t, r.read(strings.Fields(out)[1]), "type: retirement", "supersedes: ["+a+", "+b+"]", "## Why\nFetch no longer retries")
		lacks(t, r.ok("list"), a, b)
		r.ok("lint")
	})
	t.Run("requires --why", func(t *testing.T) {
		r := newRepo(t)
		refused(t, r.gm("retire", r.fact("Fetch retries twice")), "--why")
	})
	t.Run("refuses an id that names no note", func(t *testing.T) {
		refused(t, newRepo(t).gm("retire", "20200101T000000Z-000000", "--why", "w"), "no note 20200101T000000Z-000000")
	})
}

func TestSearch(t *testing.T) {
	search := func(r *repo, args ...string) []string {
		return ids(t, r.ok(append([]string{"search", "--json"}, args...)...))
	}
	t.Run("matches notes that hold any query word and ranks title matches first", func(t *testing.T) {
		r := newRepo(t)
		body := r.ok("save", "fact", "--title", "Workers share one queue", "--keywords", "jobs backlog pool", "--fact", "the proxy budget is per job", "--evidence", "e")
		title := r.fact("Proxy budget ignores bytes")
		r.fact("Unrelated")
		equal(t, search(r, "proxy", "budget"), []string{title, strings.Fields(body)[0]})
	})
	t.Run("ignores case and common English words", func(t *testing.T) {
		r := newRepo(t)
		id := r.fact("Proxy budget ignores bytes")
		equal(t, search(r, "WHAT", "is", "the", "PROXY"), []string{id})
	})
	t.Run("exits 2 when the query keeps no word", func(t *testing.T) {
		refused(t, newRepo(t).gm("search", "what", "is", "the"), "empty query")
	})
	t.Run("finds notes by topic and anchor path words", func(t *testing.T) {
		r := newRepo(t)
		id := r.ok("save", "decision", "--title", "T", "--keywords", "a b c", "--topic", "decision/storage-engine",
			"--decision", "d", "--why", "w", "--rejected", "r", "--at", "fetch.go")
		equal(t, search(r, "engine"), []string{strings.Fields(id)[0]})
		equal(t, search(r, "fetch"), []string{strings.Fields(id)[0]})
	})
	t.Run("ranks notes with equal scores oldest first", func(t *testing.T) {
		r := newRepo(t)
		r.note("20200101T000000Z-bbbbbb", "type: fact\ntitle: \"Proxy\"\nkeywords: a b c\ncreated: 2020-01-01T00:00:00Z\n", "## Fact\nf\n\n## Evidence\ne\n")
		r.note("20210101T000000Z-aaaaaa", "type: fact\ntitle: \"Proxy\"\nkeywords: a b c\ncreated: 2021-01-01T00:00:00Z\n", "## Fact\nf\n\n## Evidence\ne\n")
		equal(t, search(r, "proxy"), []string{"20200101T000000Z-bbbbbb", "20210101T000000Z-aaaaaa"})
	})
	t.Run("--all includes superseded notes and marks them", func(t *testing.T) {
		r := newRepo(t)
		old := r.fact("Proxy budget v1")
		r.fact("Proxy budget v2", "--supersedes", old)
		has(t, r.ok("search", "proxy", "--all"), old, "(superseded)")
	})
	t.Run("--type matches only notes of that type", func(t *testing.T) {
		r := newRepo(t)
		r.fact("Cache facts")
		id := r.decision("Cache uses Postgres")
		equal(t, search(r, "cache", "--type", "decision"), []string{id})
	})
	t.Run("-n prints at most N results and the default is 5", func(t *testing.T) {
		r := newRepo(t)
		for i := range 7 {
			r.fact(fmt.Sprintf("Proxy note %d", i))
		}
		if n := len(search(r, "proxy")); n != 5 {
			t.Fatalf("want 5, got %d", n)
		}
		if n := len(search(r, "proxy", "-n", "2")); n != 2 {
			t.Fatalf("want 2, got %d", n)
		}
	})
	t.Run("prints the id, type, date and title, then the file path", func(t *testing.T) {
		r := newRepo(t)
		out := r.ok("save", "fact", "--title", "Proxy budget", "--keywords", "a b c", "--fact", "f", "--evidence", "e")
		id, file := strings.Fields(out)[0], strings.Fields(out)[1]
		if got, want := r.ok("search", "proxy"), id+"  fact "+id[:4]+"-"+id[4:6]+"-"+id[6:8]+"  Proxy budget\n    "+file+"\n"; got != want {
			t.Fatalf("want %q, got %q", want, got)
		}
	})
	t.Run("--json prints id, type, created, title, file and score", func(t *testing.T) {
		r := newRepo(t)
		r.fact("Proxy budget")
		has(t, r.ok("search", "proxy", "--json"), `"id"`, `"type": "fact"`, `"created"`, `"title": "Proxy budget"`, `"file": ".memory/notes/`, `"score"`)
	})
}

func TestShow(t *testing.T) {
	t.Run("prints the note, then the numbered anchored lines read at the anchor's commit", func(t *testing.T) {
		r := newRepo(t)
		id := r.fact("Fetch is empty", "--at", "fetch.go:3")
		r.write("fetch.go", "package x\n\nfunc Fetch() { retry() }\n")
		r.commit("retry")
		out := r.ok("show", id)
		has(t, out, "title: \"Fetch is empty\"", "\n# "+r.git("rev-parse", "--short=12", "HEAD~1")+":fetch.go:3\n    3  func Fetch() {}\n")
		lacks(t, out, "retry()")
	})
	t.Run("prints the first 40 lines of a whole-file anchor", func(t *testing.T) {
		r := newRepo(t)
		var b strings.Builder
		for i := 1; i <= 50; i++ {
			fmt.Fprintf(&b, "line %d\n", i)
		}
		r.write("long.txt", b.String())
		r.commit("long")
		out := r.ok("show", r.fact("Long file", "--at", "long.txt"))
		has(t, out, "   40  line 40\n")
		lacks(t, out, "line 41")
	})
	t.Run("prints the reason when git cannot read an anchor's code", func(t *testing.T) {
		r := newRepo(t)
		r.note("20200101T000000Z-aaaaaa", "type: fact\ntitle: \"Gone\"\nkeywords: a b c\nat: [deadbeefdead:fetch.go]\ncreated: 2020-01-01T00:00:00Z\n", "## Fact\nf\n\n## Evidence\ne\n")
		has(t, r.ok("show", "20200101T000000Z-aaaaaa"), "\n# deadbeefdead:fetch.go: ")
	})
	t.Run("exits 2 for an unknown id", func(t *testing.T) {
		refused(t, newRepo(t).gm("show", "20200101T000000Z-000000"), "no note")
	})
	t.Run("--json prints the note's fields and its code", func(t *testing.T) {
		r := newRepo(t)
		out := r.ok("show", r.fact("Fetch", "--at", "fetch.go:3"), "--json")
		has(t, out, `"title": "Fetch"`, `"code": [`, `"from": 3`, `"func Fetch() {}"`)
	})
}

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
	t.Run("checks a migrated note only for title, keywords, topic and secrets", func(t *testing.T) {
		r := newRepo(t)
		r.note("20200101T000000Z-aaaaaa", "type: fact\ntitle: \"T\"\ncreated: 2020-01-01T00:00:00Z\nmigrated: engram obs-1 (discovery)\n", "free text\n")
		r.ok("lint")
		r.note("20200101T000000Z-bbbbbb", "type: fact\ntitle: \"T\"\ncreated: 2020-01-01T00:00:00Z\nmigrated: engram obs-2 (discovery)\n", "AKIAABCDEFGHIJKLMNOP\n")
		fails(t, lint(r), "AWS access key")
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
	t.Run("reports an anchor whose commit is not in the history of HEAD", func(t *testing.T) {
		r := newRepo(t)
		r.git("checkout", "-qb", "side")
		r.write("side.go", "package x\n")
		r.commit("side")
		r.fact("Side code", "--at", "side.go")
		note := strings.TrimPrefix(r.notes()[0], r.dir+"/")
		content := r.read(note)
		r.git("checkout", "-q", "main") // as after a squash: the note arrives without the commit it names
		r.write("side.go", "package x\n")
		r.write(note, content)
		fails(t, lint(r), "commit is not in the history of HEAD")
	})
	t.Run("reports an anchor whose commit is missing", func(t *testing.T) {
		r := newRepo(t)
		r.note("20200101T000000Z-aaaaaa", "type: fact\ntitle: \"T\"\nkeywords: a b c\nat: [deadbeefdead:fetch.go]\ncreated: 2020-01-01T00:00:00Z\n", sound)
		fails(t, lint(r), "deadbeefdead:fetch.go: commit not found")
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

func TestList(t *testing.T) {
	hand := func(r *repo, id, typ, created string) {
		body := map[string]string{"fact": "## Fact\nf\n\n## Evidence\ne\n", "decision": "## Decision\nd\n\n## Why\nw\n\n## Rejected\nr\n"}[typ]
		r.note(id, "type: "+typ+"\ntitle: \""+id+"\"\nkeywords: a b c\ncreated: "+created+"\n", body)
	}
	seed := func(t *testing.T) *repo {
		r := newRepo(t)
		hand(r, "20200101T000000Z-000001", "fact", "2020-01-01T00:00:00Z")
		hand(r, "20200102T000000Z-000002", "decision", "2020-01-02T00:00:00Z")
		hand(r, "20200103T000000Z-000003", "fact", "2020-01-03T00:00:00Z")
		return r
	}
	t.Run("groups live notes by type, newest first in each group", func(t *testing.T) {
		got := seed(t).ok("list")
		want := "\ndecision\n- 2020-01-02  20200102T000000Z-000002  (20200102T000000Z-000002)\n" +
			"\nfact\n- 2020-01-03  20200103T000000Z-000003  (20200103T000000Z-000003)\n- 2020-01-01  20200101T000000Z-000001  (20200101T000000Z-000001)\n"
		if got != want {
			t.Fatalf("want\n%q\ngot\n%q", want, got)
		}
	})
	t.Run("--by time lists newest first without groups", func(t *testing.T) {
		equal(t, ids(t, seed(t).ok("list", "--by", "time", "--json")), []string{"20200103T000000Z-000003", "20200102T000000Z-000002", "20200101T000000Z-000001"})
	})
	t.Run("-n lists at most N notes", func(t *testing.T) {
		equal(t, ids(t, seed(t).ok("list", "--by", "time", "-n", "1", "--json")), []string{"20200103T000000Z-000003"})
	})
	t.Run("--all includes superseded notes and retirements", func(t *testing.T) {
		r := newRepo(t)
		id := r.fact("Old")
		r.ok("retire", id, "--why", "gone")
		if n := len(ids(t, r.ok("list", "--json"))); n != 0 {
			t.Fatalf("want no live notes, got %d", n)
		}
		if n := len(ids(t, r.ok("list", "--all", "--json"))); n != 2 {
			t.Fatalf("want the note and its retirement, got %d", n)
		}
	})
	t.Run("--json lists notes without their bodies", func(t *testing.T) {
		lacks(t, seed(t).ok("list", "--json"), "## Fact")
	})
}

func TestContext(t *testing.T) {
	t.Run("prints nothing and exits 0 outside a git worktree", func(t *testing.T) {
		if res := gmIn(t, t.TempDir(), "context"); res.code != 0 || res.stdout+res.stderr != "" {
			t.Fatalf("want silence and exit 0, got %s", res)
		}
	})
	t.Run("prints nothing and exits 0 in a repository without notes", func(t *testing.T) {
		if res := newRepo(t).gm("context"); res.code != 0 || res.stdout+res.stderr != "" {
			t.Fatalf("want silence and exit 0, got %s", res)
		}
	})
	t.Run("prints a line about gm, then the 15 newest live notes", func(t *testing.T) {
		r := newRepo(t)
		for day := 1; day <= 16; day++ {
			r.note(fmt.Sprintf("202001%02dT000000Z-0000%02d", day, day), fmt.Sprintf("type: fact\ntitle: \"Note %d\"\nkeywords: a b c\ncreated: 2020-01-%02dT00:00:00Z\n", day, day), "## Fact\nf\n\n## Evidence\ne\n")
		}
		out := r.ok("context")
		lines := strings.Split(strings.TrimSpace(out), "\n")
		if len(lines) != 16 || !strings.Contains(lines[0], "gm search") || !strings.Contains(lines[1], "Note 16") || strings.Contains(out, "Note 1 ") {
			t.Fatalf("want a header and notes 16 down to 2, got\n%s", out)
		}
		if n := len(strings.Split(strings.TrimSpace(r.ok("context", "-n", "3")), "\n")); n != 4 {
			t.Fatalf("want a header and 3 notes with -n 3, got %d lines", n)
		}
	})
	t.Run("exits 0 even when the notes are broken", func(t *testing.T) {
		r := newRepo(t)
		r.write(".memory/notes/stray.md", "hello\n")
		if res := r.gm("context", "--bogus"); res.code != 0 {
			t.Fatalf("want exit 0, got %s", res)
		}
	})
}

func TestImportEngram(t *testing.T) {
	export := func(r *repo, observations, relations string) string {
		file := filepath.Join(t.TempDir(), "export.json")
		if err := os.WriteFile(file, []byte(`{"observations": [`+observations+`], "relations": [`+relations+`]}`), 0o644); err != nil {
			t.Fatal(err)
		}
		return file
	}
	obs := func(id, typ, title, content string) string {
		b, _ := json.Marshal(map[string]string{"sync_id": id, "type": typ, "title": title, "content": content, "topic_key": "topic/" + id, "created_at": "2026-11-13 10:00:00"})
		return string(b)
	}
	t.Run("writes one note per observation with a new id, a mapped type and a migrated header", func(t *testing.T) {
		r := newRepo(t)
		file := export(r, obs("obs-1", "decision", "A", "a")+","+obs("obs-2", "learning", "B", "b")+","+obs("obs-3", "bugfix", "C", "c")+","+obs("obs-4", "discovery", "D", "d"), "")
		has(t, r.ok("import-engram", file), "imported 4 notes")
		var all string
		for _, f := range r.notes() {
			has(t, filepath.Base(f), "20261113T100000Z-")
			all += r.read(strings.TrimPrefix(f, r.dir+"/"))
		}
		has(t, all, "type: decision\ntitle: \"A\"", "type: lesson\ntitle: \"B\"", "type: lesson\ntitle: \"C\"", "type: fact\ntitle: \"D\"", "migrated: engram obs-4 (discovery)")
		r.ok("lint")
	})
	t.Run("keeps only the supersedes relations Engram marked judged", func(t *testing.T) {
		r := newRepo(t)
		file := export(r, obs("obs-1", "decision", "Old", "a")+","+obs("obs-2", "decision", "New", "b")+","+obs("obs-3", "decision", "Other", "c"),
			`{"source_id": "obs-2", "target_id": "obs-1", "relation": "supersedes", "judgment_status": "judged"},
			 {"source_id": "obs-3", "target_id": "obs-2", "relation": "supersedes", "judgment_status": "pending"},
			 {"source_id": "obs-3", "target_id": "obs-1", "relation": "compatible", "judgment_status": "judged"}`)
		has(t, r.ok("import-engram", file), "1 supersedes links")
		has(t, r.ok("list", "--by", "time"), "New", "Other")
		lacks(t, r.ok("list"), "Old")
	})
	t.Run("shortens a long title and opens the body with the full title", func(t *testing.T) {
		r := newRepo(t)
		long := strings.TrimSpace(strings.Repeat("very long title ", 10))
		r.ok("import-engram", export(r, obs("obs-1", "decision", long, "body"), ""))
		content := r.read(strings.TrimPrefix(r.notes()[0], r.dir+"/"))
		has(t, content, "...\"\n", "---\n"+long+"\n\nbody\n")
	})
	t.Run("skips an observation that looks like a credential and prints its id and kind, never the text", func(t *testing.T) {
		r := newRepo(t)
		out := r.ok("import-engram", export(r, obs("obs-1", "decision", "Leaky", "AKIAABCDEFGHIJKLMNOP")+","+obs("obs-2", "decision", "Fine", "f"), ""))
		has(t, out, "imported 1 notes", "obs-1 (AWS access key)")
		lacks(t, out, "AKIAABCDEFGHIJKLMNOP")
		if len(r.notes()) != 1 {
			t.Fatal("want only the clean observation written")
		}
	})
}

func TestNoteFormat(t *testing.T) {
	t.Run("reads CRLF line ends and ignores unknown header keys", func(t *testing.T) {
		r := newRepo(t)
		r.write(".memory/notes/20200101T000000Z-aaaaaa-crlf.md", strings.ReplaceAll("---\nid: 20200101T000000Z-aaaaaa\ntype: fact\ntitle: \"Proxy budget\"\n"+
			"keywords: a b c\ncolor: blue\ncreated: 2020-01-01T00:00:00Z\n---\n## Fact\nf\n\n## Evidence\ne\n", "\n", "\r\n"))
		equal(t, ids(t, r.ok("search", "proxy", "--json")), []string{"20200101T000000Z-aaaaaa"})
		r.ok("lint")
	})
	t.Run("writes a list as a JSON array when an item holds a comma", func(t *testing.T) {
		r := newRepo(t)
		r.write("a, b.txt", "x\n")
		r.commit("comma")
		r.fact("Comma", "--at", "a, b.txt")
		has(t, r.read(strings.TrimPrefix(r.notes()[0], r.dir+"/")), `at: ["`+r.head()+`:a, b.txt"]`)
	})
}
