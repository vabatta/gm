---
id: 20261114T201953Z-40676a
type: decision
title: "Anchor paths ending in a line-range shape or a colon get one extra trailing colon"
topic: decision/anchor-encoding
keywords: anchor colon path escape parse format comma json
at: [1c120516c3a9:internal/anchor/anchor.go, 1c120516c3a9:internal/note/note.go]
created: 2026-11-14T20:19:53Z
---
## Decision
Anchor.String appends ':' when the path ends in ':' or ':N[-M]'; Parse strips one trailing ':'. Pin takes a spec whole when it names an existing file. Header lists switch to a JSON array when an item holds a comma, bracket or quote.

## Why
commit:path:lines is ambiguous for paths like log:12. This keeps the common anchor unchanged and every path round-trippable.

## Rejected
Quoting every anchor: changes the format of every existing note. A different separator: breaks the prototype's notes and git's commit:path habit.
