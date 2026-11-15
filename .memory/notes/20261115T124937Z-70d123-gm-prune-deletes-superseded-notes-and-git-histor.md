---
id: 20261115T124937Z-70d123
type: decision
title: "gm prune deletes superseded notes and git history keeps them readable"
topic: decision/prune
keywords: prune pruning cleanup delete superseded history git scale growth
at: [f45202feb63d:internal/memory/prune.go]
created: 2026-11-15T12:49:37Z
---
## Decision
gm prune deletes the files of superseded notes that are committed as they are; retirements and other live notes stay. show, search --all and list --all read pruned notes from git (git log --raw names each deleted blob, one cat-file --batch reads them). lint accepts supersedes of pruned ids.

## Why
Every command reads every note file, so superseded notes slow gm linearly. On a 10k-note benchmark, pruning 39% of files kept retrieval identical to the unpruned copy on every question type (history @5 1.00 with --all) and halved default search time. Eval agents answered 72/72 with or without pruning.

## Rejected
Pruning without git read-back: --all history recall fell from 1.00 to 0.42, and only agents that dug in git themselves recovered. Pruning retirements: they hold what replaced a retired thing. Reading pruned notes via commit^:path: 4.6 s at 10k, one tree walk per note.
