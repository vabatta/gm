package anchor

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/vabatta/gm/internal/git"
)

// Changed reports whether the anchored code changed after commit base, the
// anchor's Resolved.Base. It returns false when base is HEAD or HEAD's commit,
// and an error when git cannot tell.
//
// For a line range it runs git blame --reverse from base to HEAD. blame names,
// for each anchored line, the last commit in which the line still existed
// unchanged. The code changed when any of those commits is not HEAD. blame
// follows each line forward from base, so it still tracks lines that moved
// because a commit added or removed lines above them. For a whole-file anchor,
// any later commit that touched the file counts.
//
// --no-ignore-revs-file clears the blame.ignoreRevsFile setting, which an
// empty --ignore-revs-file or -c cannot. The setting hides changes made in the
// listed commits and makes blame fail when the file is missing, but a note
// goes stale whoever changed its code.
func Changed(r *git.Repo, base string, a Anchor) (bool, error) {
	if base == "HEAD" {
		return false, nil
	}
	if a.From == 0 {
		out, err := r.Run("log", "-1", "--format=%h", base+"..HEAD", "--", a.Path)
		return out != "", err
	}
	head, err := r.Run("rev-parse", "HEAD")
	if err != nil || head == base {
		// base..HEAD is empty when base is HEAD, and blame refuses it.
		return false, err
	}
	out, err := r.Run("blame", "--reverse", "--porcelain", "--no-ignore-revs-file", fmt.Sprintf("-L%d,%d", a.From, a.To), base+"..HEAD", "--", a.Path)
	if err != nil {
		return false, err
	}
	for line := range strings.SplitSeq(out, "\n") {
		if sha, _, _ := strings.Cut(line, " "); blameHeader.MatchString(sha) && sha != head {
			return true, nil
		}
	}
	return false, nil
}

// blameHeader matches the commit id that starts each line header in git blame
// --porcelain output, SHA-1 or SHA-256.
var blameHeader = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)
