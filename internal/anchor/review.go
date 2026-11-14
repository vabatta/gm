package anchor

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/vabatta/gm/internal/git"
)

// Changed reports whether the anchored code changed after commit base, the
// anchor's base (see Resolved). It is false when base is HEAD.
//
// For a line range it runs git blame --reverse from base to HEAD. That names, for each anchored line, the last commit in which the line
// still existed unchanged. The code changed when any of those commits is not
// HEAD. blame follows each line forward from base, so lines that
// moved because lines above them were added or removed are still tracked. For
// a whole-file anchor, any later commit that touched the file counts.
func Changed(r *git.Repo, base string, a Anchor) bool {
	if base == "HEAD" {
		return false
	}
	if a.From == 0 {
		out, _ := r.Run("log", "-1", "--format=%h", base+"..HEAD", "--", a.Path)
		return out != ""
	}
	head, err := r.Run("rev-parse", "HEAD")
	if err != nil {
		return false
	}
	out, err := r.Run("blame", "--reverse", "--porcelain", fmt.Sprintf("-L%d,%d", a.From, a.To), base+"..HEAD", "--", a.Path)
	if err != nil {
		return false
	}
	for line := range strings.SplitSeq(out, "\n") {
		if sha, _, _ := strings.Cut(line, " "); blameHeader.MatchString(sha) && sha != head {
			return true
		}
	}
	return false
}

// blameHeader matches the commit id that starts each line header in git blame --porcelain output.
var blameHeader = regexp.MustCompile(`^[0-9a-f]{40}$`)
