// Package git runs the git command inside one worktree.
package git

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// ErrNoRepo reports a directory outside any git worktree.
var ErrNoRepo = errors.New("not inside a git worktree")

// Repo is a git worktree, named by its top-level directory.
type Repo struct{ Root string }

// Open returns the worktree that contains dir.
func Open(dir string) (*Repo, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil, ErrNoRepo
	}
	return &Repo{Root: strings.TrimSpace(string(out))}, nil
}

// Run runs git with args in the worktree root and returns its output without trailing newlines.
// Pathspecs are literal, so a path holding * or : names only itself.
func (r *Repo) Run(args ...string) (string, error) { return r.Input("", args...) }

// Input is Run with stdin.
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
