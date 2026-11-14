---
id: 20261114T201953Z-a95637
type: decision
title: "CI runs nix flake check on pull requests and main"
topic: decision/ci
keywords: ci github actions continuous integration workflow checks
at: [1c120516c3a9:.github/workflows/check.yml]
created: 2026-11-14T20:19:53Z
---
## Decision
One GitHub Actions job installs nix and runs nix flake check.

## Why
Agents on several machines open pull requests; a check on GitHub catches a change that only passed on one machine, and nix flake check is the same done command agents run locally.

## Rejected
No CI: relies on every agent running the check. A Go-only workflow: would skip the vendorHash and sandbox test run.
