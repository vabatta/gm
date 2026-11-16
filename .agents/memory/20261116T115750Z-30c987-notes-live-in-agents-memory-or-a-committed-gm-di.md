---
id: 20261116T115750Z-30c987
type: decision
title: "Notes live in .agents/memory or a committed gm.dir; history also spans .memory/notes"
topic: decision/memory-directory
keywords: memory directory gmconfig agents default location history move upgrade legacy symlink
at: [803d34840d33:internal/memory/history.go]
supersedes: [20261115T225323Z-c9cc44]
created: 2026-11-16T11:57:50Z
---
## Decision
Note files live directly in .agents/memory, or in gm.dir from .gmconfig, read with git config --file. On disk, only that one directory is read. History reads cover .agents/memory, .memory/notes, and every gm.dir in the history of .gmconfig. They read only files named ID-SLUG.md directly in those directories, and a commit that adds a name it deletes elsewhere is a move, not an addition or a prune. A gm.dir through a symbolic link is refused.

## Why
Notes cannot be edited, so after a move a supersedes entry naming a note pruned in the old directory must still resolve, or lint fails for good. The manual's upgrade step is git mv .memory/notes .agents/memory with no .gmconfig, so .memory/notes must stay in history reads. .agents/memory is shared with other agent tools, so the file name shape tells notes apart. git tracks files under a symlink by the target's path, which history reads would never match.

## Rejected
Reading .memory/notes on disk as a fallback: the owner dropped that layout. Only .gmconfig's history for past directories: misses the documented upgrade. Resolving symlinks to the target path: more code for a setup that a plain path serves.
