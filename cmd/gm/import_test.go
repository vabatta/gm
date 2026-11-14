package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
