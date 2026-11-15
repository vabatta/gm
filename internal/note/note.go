// Package note defines a memory note, its file format, and the rules a new
// note must meet.
//
// A note file starts with a header between two "---" lines, one "key: value"
// per line. The title is a JSON string, so any title fits on one line. The at
// and supersedes lists are written as [a, b], or as a JSON array when an item
// needs quoting. The body follows the header, with one "## Heading" section for
// each section in the note type's template.
package note

import (
	"crypto/rand"
	"encoding/hex"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// Note is one memory note as stored in a file.
type Note struct {
	// ID is the creation time plus 6 random hex characters; see NewID.
	ID string `json:"id"`
	// Type is decision, lesson, fact or retirement, and picks the template in Templates.
	Type string `json:"type"`
	// Title is a one-line claim of at most MaxTitle characters.
	Title string `json:"title"`
	// Topic is an optional stable key, such as decision/cache-store. Lint
	// fails when two live notes share a topic.
	Topic string `json:"topic,omitempty"`
	// Keywords are space-separated search words, including synonyms the text
	// does not use. Search weights them between the title and the body.
	Keywords string `json:"keywords,omitempty"`
	// At lists the anchors, written commit:path[:from[-to]]; see package anchor.
	At []string `json:"at,omitempty"`
	// Supersedes lists the ids of the notes this note replaces. A superseded
	// note stays on disk as history but is no longer live.
	Supersedes []string `json:"supersedes,omitempty"`
	// Created is the creation time in RFC 3339, UTC. gm list and gm context sort by it.
	Created string `json:"created"`
	// Migrated records where an imported note came from, such as
	// "engram obs-1 (decision)". A migrated note keeps the body it was imported
	// with, so Problems skips the template checks for it.
	Migrated string `json:"migrated,omitempty"`
	// Body holds the "## Heading" sections.
	Body string `json:"body"`
	// File is the note's path relative to the repository root. It is not
	// written in the header, because the file's own path holds it.
	File string `json:"file"`
	// PrunedIn is the commit that deleted the note's file, for a note read
	// back from git history after gm prune. It is not written in the header.
	PrunedIn string `json:"pruned_in,omitempty"`
}

// Limits a new note must meet. They keep each note to one idea that a search
// result can show in full. Lengths count characters, not bytes.
const (
	MaxTitle    = 120
	MaxSection  = 800 // per section
	MinKeywords = 3   // words, so that synonyms the text does not use can find the note
)

// NewID returns the id of a note created at t: the UTC time as
// 20060102T150405Z, a hyphen, and 6 random hex characters. The random part keeps
// ids unique when agents on parallel branches save in the same second. The time
// part makes ids, and so note file names, sort by creation time.
func NewID(t time.Time) string {
	b := make([]byte, 3)
	_, _ = rand.Read(b)
	return t.UTC().Format("20060102T150405Z") + "-" + hex.EncodeToString(b)
}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

// FileName returns the note's file name: the id, a hyphen, and the title in
// lowercase with each run of other characters turned into a hyphen, cut to 48
// characters. The id alone identifies the note. The title part helps a person
// who reads the directory listing.
func (n Note) FileName() string {
	s := strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(n.Title), "-"), "-")
	if len(s) > 48 {
		s = strings.TrimRight(s[:48], "-")
	}
	return n.ID + "-" + s + ".md"
}

// Shorten cuts s to at most max characters and ends a cut string with "...".
// It counts characters, not bytes, so it never splits a multibyte character.
func Shorten(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	return strings.TrimSpace(string([]rune(s)[:max-3])) + "..."
}
