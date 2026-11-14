---
id: 20261114T203423Z-cf7f45
type: decision
title: "Search runs BM25 in Go over the note files, with no SQLite index"
topic: decision/search-engine
keywords: search index sqlite fts5 bm25 ranking dependency stdlib
at: [b4523fbfef54:internal/search/search.go, b4523fbfef54:go.mod]
created: 2026-11-14T20:34:23Z
---
## Decision
internal/search computes FTS5's bm25 formula (k1 1.2, b 0.75, title 5, keywords 3, body 1) over the loaded notes on every search. No index file, no gm init, no .gitignore lines, no modules in go.mod.

## Why
Owner asked for stdlib over custom code and dependencies. On 610 islegal notes it matched FTS5: recall@5 0.86 vs 0.86, top-5 overlap 0.96-0.98, same first hit 99-100%, in 20 ms. Binary went from 7.0 to 3.4 MB.

## Rejected
modernc.org/sqlite FTS5: a 3.6 MB dependency, a git-ignored index with mtime staleness checks, and gm init, for 15 ms saved per warm search.
