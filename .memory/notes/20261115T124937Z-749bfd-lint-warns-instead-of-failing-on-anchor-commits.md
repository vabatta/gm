---
id: 20261115T124937Z-749bfd
type: decision
title: "Lint warns instead of failing on anchor commits a shallow clone lacks"
topic: decision/shallow-lint
keywords: shallow clone depth ci checkout lint warning anchors
at: [f45202feb63d:internal/anchor/check.go]
created: 2026-11-15T12:49:37Z
---
## Decision
When git rev-parse --is-shallow-repository is true, anchors whose commit is missing are counted and reported as one warning; their files are still checked at HEAD.

## Why
actions/checkout clones at depth 1 by default. A depth-1 clone of gm failed lint on every anchor, which would teach people to ignore lint in CI.

## Rejected
Failing as before: unusable in default CI. Fetching history inside gm: hidden network calls.
