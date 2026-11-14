---
id: 20261114T223830Z-cf650d
type: decision
title: "Behaviour is specified in the man pages and tested only through the gm command"
topic: decision/testing
keywords: tests spec bdd manual manpage asciidoc behaviour cli testing
at: [bed9b4d1d781:cmd/gm/gm_test.go, bed9b4d1d781:docs/gm.adoc]
created: 2026-11-14T22:38:30Z
---
## Decision
docs/gm.adoc and docs/gm-note.adoc describe every behaviour. cmd/gm/gm_test.go has one subtest per sentence and runs the real gm process (the test binary re-runs main) against throwaway git repositories, checking exit status, output and files only.

## Why
Owner's scriptorium pattern: manual, then BDD spec, then code. Tests of internal packages pinned the implementation; tests of the command pin the promises. Writing the spec from the manual surfaced two gaps at once: no --version, and lint --review losing moved lines.

## Rejected
Gherkin feature files: a second copy of the spec. Unit tests per package: they test the implementation, not what a user sees. testscript: a dependency.
