---
id: 20261114T201953Z-95fb67
type: fact
title: "At 610 notes gm searches in 7 ms warm and lints in 0.1 s"
topic: fact/speed
keywords: speed timing performance benchmark latency index size
at: [1c120516c3a9:internal/index/index.go]
created: 2026-11-14T20:19:53Z
---
## Fact
On caraway, an islegal clone with 610 notes (100 anchored): cold search with index build 0.045 s, warm search 0.007 s, save 0.015 s, lint 0.10 s, lint --review 0.27 s, context 0.012 s, index 1.0 MB. Nix binary 7.0 MB static.

## Evidence
time over the nix-built binary, 2026-11-14, with 500 synthetic Engram observations imported plus 100 saves anchored to internal/*.go.
