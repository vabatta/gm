// Command gm keeps project memory in the repository it is about, as one
// markdown file per note in .memory/notes. Notes are committed with the work
// and merged by git like code. gm help prints the commands.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"strings"

	"github.com/vabatta/gm/internal/memory"
)

// usage is the text that gm help prints.
const usage = `gm: project memory as note files in .memory/notes, merged by git.

gm save decision --title T --keywords "a b c" --decision D --why W --rejected R [options]
gm save lesson   --title T --keywords "a b c" --happened H --cause C --rule R [options]
gm save fact     --title T --keywords "a b c" --fact F --evidence E [options]
    --at path[:from[-to]]  code the note is about, pinned to HEAD; path from the repo root; repeatable
    --topic K              stable key, like decision/cache-store, when the note may be revised
    --supersedes ID        a note this one replaces; repeatable
gm retire ID... --why W  withdraw notes that have no successor
gm search WORDS... [-n 5] [--type T] [--all] [--json]
gm show ID [--json]      the note and the code its anchors point at, read from their commit
gm lint [--review] [--json]  exit 1 on broken notes; --review also lists anchors whose code changed
gm list [--by type|time] [-n N] [--all] [--json]   the catalogue or the log of live notes
gm context [-n 15]       recent live notes for a session-start hook; silent outside a repo with notes
gm prune                 delete superseded notes; git history keeps them for show and --all
gm import SOURCE FILE    write the memories exported from another tool; SOURCE: engram
gm --version

The manual is gm(1); the note format is gm-note(5).

Exit status: 0 ok, 1 lint found problems, 2 error.
`

// version is the gm version without a leading "v", such as 0.1.0. The nix
// package sets it with -ldflags "-X main.version=...". Without it, the Go
// toolchain's build info supplies it: go install github.com/vabatta/gm/cmd/gm@v0.1.0
// records v0.1.0 there, and go build in a checkout records a pseudo-version
// of the commit, such as 0.0.0-20261115122924-d5a43d04d3e4. A build with no
// version from either, such as go run, prints dev.
var version = ""

func init() {
	if version != "" {
		return
	}
	version = "dev"
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		version = strings.TrimPrefix(bi.Main.Version, "v")
	}
}

func main() { os.Exit(run(os.Args[1:], ".", os.Stdout, os.Stderr)) }

// run executes one gm command in dir and returns the exit status: 0 on
// success, 1 when lint finds problems, 2 on any other error.
//
// gm context always returns 0, and it prints nothing outside a repository
// with notes. Claude Code runs it as a SessionStart hook in every directory,
// and a failing hook would get in the way of the session.
func run(args []string, dir string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		fmt.Fprint(stdout, usage)
		return 0
	}
	if args[0] == "--version" {
		fmt.Fprintln(stdout, "gm", version)
		return 0
	}
	cmd, args := args[0], args[1:]
	st, err := memory.Open(dir)
	if cmd == "context" {
		if err == nil && len(st.Files()) > 0 {
			_ = context(st, args, stdout)
		}
		return 0
	}
	if err == nil {
		if do, ok := commands[cmd]; ok {
			err = do(st, args, stdout)
		} else {
			err = fmt.Errorf("unknown command %q; run gm help", cmd)
		}
	}
	var lintErr lintFailed
	switch {
	case errors.As(err, &lintErr):
		return 1
	case err != nil:
		fmt.Fprintln(stderr, "gm:", err)
		return 2
	}
	return 0
}

// commands maps each command name to the function that runs it. context is
// not here, because run handles it before opening the store.
var commands = map[string]func(st *memory.Store, args []string, w io.Writer) error{
	"save":   save,
	"retire": retire,
	"search": find,
	"show":   show,
	"lint":   lint,
	"list":   list,
	"prune":  prune,
	"import": importFrom,
}
