---
id: 20261115T222310Z-c50d73
type: decision
title: "Memory directory is set in a committed .gmconfig, not git config"
topic: decision/memory-directory
keywords: memory directory gmconfig config location
at: [363844048485:internal/memory/memory.go]
supersedes: [20261115T221706Z-200460]
created: 2026-11-15T22:23:10Z
---
## Decision
gm.dir in .gmconfig at the worktree root names the memory directory, default .memory. gm reads it with git config --file. History reads cover .memory/notes and the configured notes folder, matching notes by file name.

## Why
The location must be the same in every clone, branch and worktree, and only the commit is shared. git config --file gives a known syntax without a parser of our own.

## Rejected
git config gm.dir: per clone, so a collaborator without it writes notes where others never read them. Auto-detecting .agents/memory: not a setting, only one fixed alternative.
