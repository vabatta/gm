---
id: 20261114T211018Z-02d19b
type: decision
title: "Lint warns instead of failing on anchor commits a shallow clone lacks"
topic: decision/shallow-lint
keywords: shallow clone depth ci checkout lint warning anchors
at: [0858812c4816:internal/anchor/anchor.go, 0858812c4816:internal/memory/memory.go]
created: 2026-11-14T21:10:18Z
---
## Decision
When git rev-parse --is-shallow-repository is true, anchors whose commit is missing are counted and reported as one warning; their files are still checked at HEAD.

## Why
actions/checkout clones at depth 1 by default. A depth-1 clone of gm failed lint on every anchor, which would teach people to ignore lint in CI.

## Rejected
Failing as before: unusable in default CI. Fetching history inside gm: hidden network calls.
