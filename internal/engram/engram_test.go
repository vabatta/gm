package engram_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vabatta/gm/internal/engram"
	"github.com/vabatta/gm/internal/gittest"
	"github.com/vabatta/gm/internal/memory"
)

func TestImportKeepsJudgedSupersedesAndFullTitlesAndRefusesSecrets(t *testing.T) {
	st := gittest.New(t)
	long := strings.Repeat("very long title ", 10)
	export := `{"observations": [
	  {"sync_id": "obs-1", "type": "decision", "title": "Old choice", "content": "a", "topic_key": "decision/x", "created_at": "2026-11-13 10:00:00"},
	  {"sync_id": "obs-2", "type": "learning", "title": "` + long + `", "content": "b", "topic_key": "decision/x", "created_at": "2026-11-14 10:00:00"},
	  {"sync_id": "obs-3", "type": "discovery", "title": "Leaky", "content": "AKIAABCDEFGHIJKLMNOP", "created_at": "2026-11-14 11:00:00"}],
	 "relations": [
	  {"source_id": "obs-2", "target_id": "obs-1", "relation": "supersedes", "judgment_status": "judged"},
	  {"source_id": "obs-1", "target_id": "obs-2", "relation": "compatible", "judgment_status": "judged"}]}`
	file := filepath.Join(t.TempDir(), "export.json")
	os.WriteFile(file, []byte(export), 0o644)
	res, err := engram.Import(st, file)
	if err != nil || res.Notes != 2 || res.Links != 1 || len(res.Refused) != 1 {
		t.Fatalf("want 2 notes, 1 link, 1 refused, got %+v %v", res, err)
	}
	ns, _ := st.Load()
	live := memory.Live(ns)
	if len(live) != 1 || live[0].Type != "lesson" || len([]rune(live[0].Title)) > 120 || !strings.HasPrefix(live[0].Body, strings.TrimSpace(long)) {
		t.Fatalf("want the long lesson live, shortened, full title in its body, got %+v", live)
	}
	if p := st.Lint(false).Problems; len(p) != 0 {
		t.Fatalf("want a clean lint, got %q", p)
	}
}
