---
id: 20261115T223017Z-728045
type: decision
title: "Notes default to .agents/memory, set by a committed .gmconfig"
topic: decision/memory-directory
keywords: memory directory gmconfig agents default location
at: [31db0ea274c3:internal/memory/memory.go]
supersedes: [20261115T222310Z-c50d73]
created: 2026-11-15T22:30:17Z
---
## Decision
Note files live directly in .agents/memory by default, or in gm.dir from .gmconfig at the worktree root, read with git config --file. With no gm.dir, .memory/notes stays in use while it exists and .agents/memory does not. History reads cover .memory/notes and the current directory, matching notes by file name.

## Why
The location must be the same in every clone, so it is committed. Nothing but notes lives in the directory since the SQLite index went, so a notes/ level adds nothing. The fallback keeps repositories from earlier releases working instead of searching an empty directory.

## Rejected
git config gm.dir: per clone, so a collaborator without it writes notes where others never read them. Auto-detecting .agents/memory only: not a setting. A plain switch of the default with no fallback: old repositories would silently search nothing.
