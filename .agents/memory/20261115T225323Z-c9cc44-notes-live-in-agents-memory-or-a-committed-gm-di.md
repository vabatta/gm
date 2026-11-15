---
id: 20261115T225323Z-c9cc44
type: decision
title: "Notes live in .agents/memory or a committed gm.dir; history spans every past gm.dir"
topic: decision/memory-directory
keywords: memory directory gmconfig agents default location history move
at: [954205614036:internal/memory/memory.go]
supersedes: [20261115T223557Z-6a4ebc]
created: 2026-11-15T22:53:23Z
---
## Decision
Note files live directly in .agents/memory, or in gm.dir from .gmconfig at the worktree root, read with git config --file. History reads cover .agents/memory and every gm.dir in the history of .gmconfig, matching notes by file name and skipping commits that only move them.

## Why
The location must be the same in every clone, so it is committed. Notes cannot be edited, so after a move a supersedes entry naming a note pruned in the old directory must still resolve, or lint fails for good. The history of .gmconfig says which directories to read, so no fixed legacy list is needed.

## Rejected
git config gm.dir: per clone. Reading only the current directory: a move breaks lint, anchor bases and pruned notes permanently. A fallback to .memory/notes: the owner chose to drop that layout; the manual gives the upgrade step.
