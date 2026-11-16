---
id: 20261115T124937Z-b0a1d0
type: decision
title: "Search goes through a Ranker port with BM25 as the adapter"
topic: decision/search-port
keywords: search ranker port adapter bm25 hexagonal index swap
at: [f45202feb63d:internal/search/search.go, f45202feb63d:internal/search/bm25/bm25.go, f45202feb63d:cmd/gm/search.go]
created: 2026-11-15T12:49:37Z
---
## Decision
internal/search defines Ranker (Rank notes against query words) and Find, which owns stop words, the --all and --type rules, sorting and the limit. internal/search/bm25 is the adapter; cmd/gm/search.go picks it.

## Why
Search already moved once (SQLite FTS5 to BM25 in Go), and a git grep prefilter or an index are the next candidates at scale. With a port, an adapter swap changes one line, and bench/run.sh compares adapters on the same corpus.

## Rejected
Ports for git, note storage or anchors: one implementation each with no plausible second; an interface there is speculation.
