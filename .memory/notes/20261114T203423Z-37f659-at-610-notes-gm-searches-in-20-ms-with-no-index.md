---
id: 20261114T203423Z-37f659
type: fact
title: "At 610 notes gm searches in 20 ms with no index and lints in 0.1 s"
topic: fact/speed
keywords: speed timing performance benchmark latency
at: [b4523fbfef54:internal/search/search.go]
supersedes: [20261114T201953Z-95fb67]
created: 2026-11-14T20:34:23Z
---
## Fact
On caraway, an islegal clone with 610 notes (100 anchored): search 0.02 s every call (no index to build or keep warm), lint 0.10 s, context 0.01 s. Static binary 3.4 MB.

## Evidence
time over the nix-built binary after the SQLite removal, 2026-11-14; FTS5 baseline on the same corpus: 0.03 s cold, about 0.005 s warm.
