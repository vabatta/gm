// Package git runs the git binary in one worktree. gm asks git for every
// repository fact instead of reading .git itself, so worktrees, shallow clones
// and partial clones behave the way git defines them.
package git

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// ErrNoRepo is the error Open returns for a directory outside any git worktree.
var ErrNoRepo = errors.New("not inside a git worktree")

// Repo is one git worktree. Root is its top-level directory, as printed by
// git rev-parse --show-toplevel.
type Repo struct{ Root string }

// Open returns the worktree that contains dir, found with git rev-parse
// --show-toplevel. It returns ErrNoRepo when git finds no worktree.
func Open(dir string) (*Repo, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil, ErrNoRepo
	}
	return &Repo{Root: strings.TrimSpace(string(out))}, nil
}

// Run runs git with args in r.Root and returns its stdout without trailing
// newlines. Every call adds --literal-pathspecs, so a path that contains * or :
// matches only the file with that name. An error holds the arguments and
// git's stderr.
func (r *Repo) Run(args ...string) (string, error) { return r.Input("", args...) }

// Input is Run with stdin, for git commands that read their input from stdin,
// such as cat-file --batch-check and rev-list --stdin.
func (r *Repo) Input(stdin string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"--literal-pathspecs"}, args...)...)
	cmd.Dir = r.Root
	cmd.Stdin = strings.NewReader(stdin)
	out, err := cmd.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			err = errors.New(strings.TrimSpace(string(exit.Stderr)))
		}
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimRight(string(out), "\n"), nil
}
