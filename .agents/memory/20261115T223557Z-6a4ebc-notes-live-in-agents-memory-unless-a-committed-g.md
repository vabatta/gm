---
id: 20261115T223557Z-6a4ebc
type: decision
title: "Notes live in .agents/memory unless a committed .gmconfig says otherwise"
topic: decision/memory-directory
keywords: memory directory gmconfig agents default location
at: [4c877d1b0212:internal/memory/memory.go]
supersedes: [20261115T223017Z-728045]
created: 2026-11-15T22:35:57Z
---
## Decision
Note files live directly in .agents/memory, or in gm.dir from .gmconfig at the worktree root, read with git config --file. gm reads notes and their history only from that directory.

## Why
The location must be the same in every clone, so it is committed. Nothing but notes lives in the directory since the SQLite index went, so a notes/ level adds nothing.

## Rejected
git config gm.dir: per clone, so a collaborator without it writes notes where others never read them. A fallback to .memory/notes with history read across both: extra rules for repositories the owner chose not to support.
