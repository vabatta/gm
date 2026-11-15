package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestImport(t *testing.T) {
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
		has(t, r.ok("import", "engram", file), "imported 4 notes")
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
		has(t, r.ok("import", "engram", file), "1 supersedes links")
		has(t, r.ok("list", "--by", "time"), "New", "Other")
		lacks(t, r.ok("list"), "Old")
	})
	t.Run("shortens a long title and opens the body with the full title", func(t *testing.T) {
		r := newRepo(t)
		long := strings.TrimSpace(strings.Repeat("very long title ", 10))
		r.ok("import", "engram", export(r, obs("obs-1", "decision", long, "body"), ""))
		content := r.read(strings.TrimPrefix(r.notes()[0], r.dir+"/"))
		has(t, content, "...\"\n", "---\n"+long+"\n\nbody\n")
	})
	t.Run("writes nothing when two observations share an id", func(t *testing.T) {
		r := newRepo(t)
		refused(t, r.gm("import", "engram", export(r, obs("obs-1", "decision", "A", "a")+","+obs("obs-1", "decision", "B", "b"), "")), "obs-1")
		if len(r.notes()) != 0 {
			t.Fatal("want nothing written")
		}
	})
	t.Run("writes nothing when an observation has no title", func(t *testing.T) {
		r := newRepo(t)
		refused(t, r.gm("import", "engram", export(r, obs("obs-1", "decision", "A", "a")+","+obs("obs-2", "decision", " ", "b"), "")), "obs-2")
		if len(r.notes()) != 0 {
			t.Fatal("want nothing written")
		}
	})
	t.Run("keeps the migrated header on one line", func(t *testing.T) {
		r := newRepo(t)
		r.ok("import", "engram", export(r, obs("o2\ntype: retirement", "decision", "T", "b"), ""))
		content := r.read(strings.TrimPrefix(r.notes()[0], r.dir+"/"))
		has(t, content, "migrated: engram o2 type: retirement (decision)\n", "type: decision\n")
		lacks(t, content, "\ntype: retirement")
	})
	t.Run("moves a title on several lines into the body and keeps its first line", func(t *testing.T) {
		r := newRepo(t)
		r.ok("import", "engram", export(r, obs("obs-1", "decision", "First line\nsecond line", "body"), ""))
		has(t, r.read(strings.TrimPrefix(r.notes()[0], r.dir+"/")), `title: "First line"`, "---\nFirst line\nsecond line\n\nbody\n")
	})
	t.Run("makes the id from the time of the import when an observation has no creation time", func(t *testing.T) {
		r := newRepo(t)
		b, _ := json.Marshal(map[string]string{"sync_id": "obs-1", "type": "decision", "title": "T", "content": "c"})
		r.ok("import", "engram", export(r, string(b), ""))
		has(t, filepath.Base(r.notes()[0]), time.Now().UTC().Format("20060102T"))
	})
	t.Run("exits 2 for a missing file or a file that is not JSON", func(t *testing.T) {
		r := newRepo(t)
		refused(t, r.gm("import", "engram", filepath.Join(t.TempDir(), "absent.json")), "absent.json")
		bad := filepath.Join(t.TempDir(), "bad.json")
		if err := os.WriteFile(bad, []byte("not json"), 0o644); err != nil {
			t.Fatal(err)
		}
		refused(t, r.gm("import", "engram", bad), "invalid character")
	})
	t.Run("turns spaces in a topic into hyphens", func(t *testing.T) {
		r := newRepo(t)
		b, _ := json.Marshal(map[string]string{"sync_id": "obs-1", "type": "decision", "title": "T", "content": "c", "topic_key": "decision/cache store", "created_at": "2026-11-13 10:00:00"})
		r.ok("import", "engram", export(r, string(b), ""))
		has(t, r.read(strings.TrimPrefix(r.notes()[0], r.dir+"/")), "topic: decision/cache-store\n")
	})
	t.Run("an unknown source exits 2 and lists the sources gm reads", func(t *testing.T) {
		refused(t, newRepo(t).gm("import", "mem0", "export.json"), "unknown source \"mem0\"", "engram")
	})
}
