---
id: 20261114T201953Z-1dcd9d
type: decision
title: "Lint checks all anchors in two batched git calls"
topic: decision/lint-batching
keywords: lint speed batch cat-file rev-list ancestor performance
at: [1c120516c3a9:internal/anchor/anchor.go]
created: 2026-11-14T20:19:53Z
---
## Decision
One cat-file --batch-check asks for every anchor commit and every HEAD:path; one rev-list --stdin with ^HEAD lists anchor commits HEAD does not contain. Renames are diffed once per commit, only for missing files.

## Why
The prototype ran three git calls per anchor (about 36 ms each). Batched, lint over 610 notes with 100 anchors takes 0.10 s on caraway.

## Rejected
merge-base --is-ancestor per commit: one process per anchor. git rev-list HEAD into a set: cost grows with the whole history.
