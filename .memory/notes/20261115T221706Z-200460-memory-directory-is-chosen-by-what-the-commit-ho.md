---
id: 20261115T221706Z-200460
type: decision
title: "Memory directory is chosen by what the commit holds, not by config"
topic: decision/memory-directory
keywords: memory directory agents config location
at: [7cb860351cba:internal/memory/memory.go]
created: 2026-11-15T22:17:06Z
---
## Decision
gm uses .agents/memory when that directory exists at the worktree root, and .memory otherwise. History reads cover both directories and match notes by file name.

## Why
The location must be the same in every clone, branch and worktree, and only the commit is shared. A git config key such as gm.dir lives per clone, so a collaborator without it would write notes where others never read them.

## Rejected
git config gm.dir: not shared through the repository. A committed config file: one more format to keep as a contract, for a choice that the directory itself already records.
