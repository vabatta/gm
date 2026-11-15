---
id: 20261115T000952Z-694011
type: decision
title: "Imports skip memories that look like secrets, for every source"
topic: decision/import-secrets
keywords: secret import importer refuse credential migration leak
at: [66335d54f580:internal/importer/importer.go]
supersedes: [20261114T201953Z-0b5db9]
created: 2026-11-15T00:09:52Z
---
## Decision
importer.Import writes no note whose text matches the secret check, drops links to it, and prints its source id and the kind of secret. The rule is in the shared core, so every adapter gets it.

## Why
A note committed with a credential is pushed with the branch; lint would catch it only after it is on disk. The printout names the id, not the text, so it never echoes the secret.

## Rejected
Write everything and let lint fail: the secret is already in the work tree. A check in each adapter: one adapter could forget it.
