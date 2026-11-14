// Package index searches the notes through .memory/index.sqlite, a
// git-ignored SQLite FTS5 projection of the note files. The index is rebuilt
// whenever a note file or the notes directory is newer than it, and replaced
// by rename, so a concurrent reader sees the old index or the new one, never half.
package index

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/vabatta/gm/internal/anchor"
	"github.com/vabatta/gm/internal/memory"

	_ "modernc.org/sqlite"
)

// schema changes whenever the table changes, so an index from an older gm is rebuilt.
const schema = 1

// Query is a search: words matched with OR and ranked by BM25 with the title
// weighted 5, keywords 3 and body 1. Topic and anchor paths count as keywords.
type Query struct {
	Words string
	Limit int
	Type  string // only notes of this type when set
	All   bool   // include superseded and retired notes
}

// Hit is one search result.
type Hit struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Created    string `json:"created"`
	Title      string `json:"title"`
	File       string `json:"file"`
	Superseded bool   `json:"superseded,omitempty"`
}

var (
	wordRe = regexp.MustCompile(`[\pL\pN_]+`)
	stop   = map[string]bool{}
)

func init() {
	for _, w := range strings.Fields("the a an of to in on for and or is are was were why what how did do does we our it that this with from by at as be not no") {
		stop[w] = true
	}
}

// Search returns the best matches for q.
func Search(st *memory.Store, q Query) ([]Hit, error) {
	var terms []string
	for _, w := range wordRe.FindAllString(strings.ToLower(q.Words), -1) {
		if !stop[w] {
			terms = append(terms, `"`+w+`"`)
		}
	}
	if len(terms) == 0 {
		return nil, errors.New("empty query")
	}
	db, err := open(st)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := db.Query(`select id, type, created, title, file, superseded from n
		where n match ? and (? or superseded = 0) and (? = '' or type = ?)
		order by bm25(n, 0, 5.0, 3.0, 1.0) limit ?`, strings.Join(terms, " OR "), q.All, q.Type, q.Type, q.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var hits []Hit
	for rows.Next() {
		var h Hit
		if err := rows.Scan(&h.ID, &h.Type, &h.Created, &h.Title, &h.File, &h.Superseded); err != nil {
			return nil, err
		}
		hits = append(hits, h)
	}
	return hits, rows.Err()
}

func open(st *memory.Store) (*sql.DB, error) {
	path := filepath.Join(st.Dir(), "index.sqlite")
	if fresh(st, path) {
		return sql.Open("sqlite", path)
	}
	if err := os.MkdirAll(st.Dir(), 0o755); err != nil {
		return nil, err
	}
	tmp := fmt.Sprintf("%s.tmp%d", path, os.Getpid())
	if err := build(st, tmp); err != nil {
		os.Remove(tmp)
		return nil, err
	}
	if err := os.Rename(tmp, path); err != nil {
		return nil, err
	}
	return sql.Open("sqlite", path)
}

// fresh reports whether the index exists, has this schema, and is newer than every note.
func fresh(st *memory.Store, path string) bool {
	fi, err := os.Stat(path)
	if err != nil {
		return false
	}
	var newest time.Time
	for _, f := range append(st.Files(), st.NotesDir()) {
		if gi, err := os.Stat(f); err == nil && gi.ModTime().After(newest) {
			newest = gi.ModTime()
		}
	}
	if fi.ModTime().Before(newest) {
		return false
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return false
	}
	defer db.Close()
	var v int
	return db.QueryRow("pragma user_version").Scan(&v) == nil && v == schema
}

func build(st *memory.Store, path string) error {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return err
	}
	defer db.Close()
	if _, err := db.Exec(fmt.Sprintf(`create virtual table n using fts5(id unindexed, title, keywords, body,
		type unindexed, created unindexed, file unindexed, superseded unindexed, tokenize='unicode61');
		pragma user_version = %d`, schema)); err != nil {
		return err
	}
	ns, _ := st.Load()
	dead := memory.Dead(ns)
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	ins, err := tx.Prepare(`insert into n values(?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	split := strings.NewReplacer("/", " ", ".", " ", "_", " ", "-", " ")
	for _, n := range ns {
		kw := []string{n.Keywords, split.Replace(n.Topic)}
		for _, s := range n.At {
			if a, err := anchor.Parse(s); err == nil {
				kw = append(kw, split.Replace(a.Path))
			}
		}
		gone := dead[n.ID] || n.Type == "retirement"
		if _, err := ins.Exec(n.ID, n.Title, strings.Join(kw, " "), n.Body, n.Type, n.Created, n.File, gone); err != nil {
			return err
		}
	}
	return tx.Commit()
}
