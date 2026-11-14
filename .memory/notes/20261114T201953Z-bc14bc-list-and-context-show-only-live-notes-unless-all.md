---
id: 20261114T201953Z-bc14bc
type: decision
title: "list and context show only live notes unless --all"
topic: decision/list-live
keywords: list catalogue log superseded retired live all
at: [1c120516c3a9:cmd/gm/main.go]
created: 2026-11-14T20:19:53Z
---
## Decision
gm list hides superseded notes and retirements by default, like search; --all shows them.

## Why
The catalogue answers what holds now; history stays one flag away.

## Rejected
The prototype listed every note, so a revised topic showed twice.
