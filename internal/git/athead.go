package git

import (
	"os"
	"path/filepath"
	"strings"
)

// AtHEAD reports, for each path relative to the worktree root, whether the
// file in the worktree holds exactly what HEAD holds at that path. It asks
// git for the blob at HEAD with ls-tree and hashes the worktree file with
// hash-object, which applies the same filters as git add, such as
// core.autocrlf, and it compares the executable bit with the mode at HEAD. A
// symlink matches when its target is the blob's content. A file missing from
// the worktree matches only when sparse-checkout left it out, which git marks
// skip-worktree.
//
// It compares content instead of asking git status, because status trusts
// the index: a change hidden by update-index --assume-unchanged or
// --skip-worktree, or made inside a submodule, does not show there. A path
// that HEAD does not hold as a file, such as an untracked, ignored or deleted
// file, or one inside a submodule, maps to false.
func (r *Repo) AtHEAD(paths []string) map[string]bool {
	at := map[string]bool{}
	if len(paths) == 0 {
		return at
	}
	out, err := r.Run(append([]string{"ls-tree", "-z", "HEAD", "--"}, paths...)...)
	if err != nil {
		return at // no commits yet, so nothing is at HEAD
	}
	blobs, modes := map[string]string{}, map[string]string{}
	for entry := range strings.SplitSeq(out, "\x00") {
		// <mode> SP <type> SP <id> TAB <path>
		meta, path, ok := strings.Cut(entry, "\t")
		if f := strings.Fields(meta); ok && len(f) == 3 && f[1] == "blob" {
			blobs[path], modes[path] = f[2], f[0]
		}
	}
	var files []string
	for _, p := range paths {
		full := filepath.Join(r.Root, filepath.FromSlash(p))
		info, err := os.Lstat(full)
		switch {
		case blobs[p] == "":
		case os.IsNotExist(err):
			flags, _ := r.Run("ls-files", "-v", "--", p)
			at[p] = strings.HasPrefix(flags, "S ")
		case err != nil:
		case modes[p] == "120000":
			target, err := os.Readlink(full)
			content, _ := r.Raw("", "cat-file", "-p", blobs[p])
			at[p] = err == nil && target == content
		case info.Mode().IsRegular() && (info.Mode()&0o111 != 0) == (modes[p] == "100755"):
			files = append(files, p)
		}
	}
	if len(files) == 0 {
		return at
	}
	hashes, err := r.Input(strings.Join(files, "\n")+"\n", "hash-object", "--stdin-paths")
	if err != nil {
		return at
	}
	for i, h := range strings.Split(hashes, "\n") {
		if i < len(files) {
			at[files[i]] = h == blobs[files[i]]
		}
	}
	return at
}
