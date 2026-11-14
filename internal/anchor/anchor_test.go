package anchor_test

import (
	"testing"

	"github.com/vabatta/gm/internal/anchor"
	"github.com/vabatta/gm/internal/gittest"
)

func TestAnchorsOnPathsHoldingColonsCommasAndRangeShapesReadBackUnchanged(t *testing.T) {
	st := gittest.New(t)
	gittest.Write(t, st, "a:b, c.txt", "1\n2\n3\n")
	gittest.Write(t, st, "log:12", "x\n")
	gittest.Commit(t, st, "odd names")
	for spec, want := range map[string]anchor.Anchor{
		"a:b, c.txt:2-3": {Path: "a:b, c.txt", From: 2, To: 3},
		"a:b, c.txt":     {Path: "a:b, c.txt"},
		"log:12":         {Path: "log:12"},
	} {
		a, err := anchor.Pin(st.Git, spec)
		if err != nil {
			t.Fatal(err)
		}
		if a.Path != want.Path || a.From != want.From || a.To != want.To {
			t.Fatalf("Pin(%q) = %+v, want %+v", spec, a, want)
		}
		s, err := gittest.Fact(st, "Odd file "+spec, spec)
		if err != nil {
			t.Fatal(err)
		}
		n, err := st.Get(s.Note.ID)
		if err != nil || len(n.At) != 1 {
			t.Fatalf("want the note read back with one anchor, got %v %v", n.At, err)
		}
		if back, err := anchor.Parse(n.At[0]); err != nil || back != a {
			t.Fatalf("anchor %q read back as %+v, want %+v (%v)", n.At[0], back, a, err)
		}
	}
	if p := st.Lint(false).Problems; len(p) != 0 {
		t.Fatalf("want a clean lint, got %q", p)
	}
}
