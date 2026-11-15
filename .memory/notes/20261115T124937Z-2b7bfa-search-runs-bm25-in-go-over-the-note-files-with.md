---
id: 20261115T124937Z-2b7bfa
type: decision
title: "Search runs BM25 in Go over the note files, with no SQLite index"
topic: decision/search-engine
keywords: search index sqlite fts5 bm25 ranking dependency stdlib
at: [f45202feb63d:internal/search/bm25/bm25.go, f45202feb63d:go.mod]
created: 2026-11-15T12:49:37Z
---
## Decision
internal/search/bm25 computes FTS5's bm25 formula (k1 1.2, b 0.75, title 5, keywords 3, body 1) over the loaded notes on every search, summing scores in query-word order so results are deterministic. No index file, no gm init, no .gitignore lines, no modules in go.mod.

## Why
gm stays one static binary on the standard library. On 610 islegal notes it matched FTS5: recall@5 0.86 vs 0.86, top-5 overlap 0.96-0.98, same first hit 99-100%, in 20 ms. The binary went from 7.0 to 3.4 MB.

## Rejected
modernc.org/sqlite FTS5: a 3.6 MB dependency, a git-ignored index with mtime staleness checks, and gm init, for 15 ms saved per warm search.
