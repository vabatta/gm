package main

import (
	"strings"
	"testing"
)

func TestRetire(t *testing.T) {
	t.Run("writes a retirement that hides the retired notes", func(t *testing.T) {
		r := newRepo(t)
		a, b := r.fact("Fetch retries twice"), r.fact("Fetch backs off")
		out := r.ok("retire", a, b, "--why", "Fetch no longer retries")
		has(t, r.read(strings.Fields(out)[1]), "type: retirement", "supersedes: ["+a+", "+b+"]", "## Why\nFetch no longer retries")
		lacks(t, r.ok("list"), a, b)
		r.ok("lint")
	})
	t.Run("requires --why", func(t *testing.T) {
		r := newRepo(t)
		refused(t, r.gm("retire", r.fact("Fetch retries twice")), "--why")
	})
	t.Run("refuses an id that names no note", func(t *testing.T) {
		refused(t, newRepo(t).gm("retire", "20200101T000000Z-000000", "--why", "w"), "no note 20200101T000000Z-000000")
	})
}
