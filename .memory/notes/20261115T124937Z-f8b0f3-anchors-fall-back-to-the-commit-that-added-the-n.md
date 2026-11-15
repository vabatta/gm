---
id: 20261115T124937Z-f8b0f3
type: decision
title: "Anchors fall back to the commit that added the note after a squash or rebase"
topic: decision/anchor-base
keywords: anchor squash rebase merge commit base fallback github
at: [f45202feb63d:internal/anchor/check.go, f45202feb63d:internal/memory/base.go, f45202feb63d:internal/memory/lint.go]
created: 2026-11-15T12:49:37Z
---
## Decision
An anchor keeps its commit:path:lines form. gm reads it at its own commit while HEAD reaches it, else at the commit that added the note, else HEAD. On a branch not yet in the default branch, lint fails when a line-range anchor's lines changed after its base; whole-file changes are review lines.

## Why
Most projects squash or rebase, which drops the anchor commits. The commit that added the note survives every merge strategy and holds the code, since code is committed before or with the note. A real-git prototype passed merge, squash and rebase; its hole, code edited after the note before a squash, is what the branch rule catches. Failing whole-file anchors too flagged 17 sound notes in gm itself.

## Rejected
Fingerprints of the anchored lines: weak matching. Storing the lines in the note: copies code into notes. Keep-alive refs refs/gm/*: every clone must push and fetch them, and they pile up.
