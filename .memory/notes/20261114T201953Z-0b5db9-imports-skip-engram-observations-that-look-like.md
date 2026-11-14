---
id: 20261114T201953Z-0b5db9
type: decision
title: "Imports skip Engram observations that look like secrets"
topic: decision/import-secrets
keywords: secret import engram refuse credential migration leak
at: [1c120516c3a9:internal/engram/engram.go]
created: 2026-11-14T20:19:53Z
---
## Decision
import-engram writes no note whose text matches the secret check, drops relations to it, and prints its sync id and the kind of secret.

## Why
A note committed with a credential is pushed with the branch; lint would catch it only after it is on disk. The printout names the id, not the text, so it never echoes the secret.

## Rejected
Write everything and let lint fail: the secret is already in the work tree and easy to commit.
