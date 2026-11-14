---
id: 20261114T231626Z-47f353
type: decision
title: "Behaviour is specified in the man pages and tested only through the gm command"
topic: decision/testing
keywords: tests spec bdd manual manpage asciidoc behaviour cli testing
at: [ce57d1fe8b34:cmd/gm/spec_test.go, ce57d1fe8b34:docs/gm.adoc]
supersedes: [20261114T223830Z-cf650d]
created: 2026-11-14T23:16:26Z
---
## Decision
docs/gm.adoc and docs/gm-note.adoc describe every behaviour. cmd/gm/<section>_test.go holds one manual section each, with one subtest per sentence; spec_test.go is the shared harness. Each spec runs the real gm process (the test binary re-runs main) against a throwaway git repository and checks exit status, output and files only.

## Why
Owner's scriptorium pattern: manual, then BDD spec, then code. Tests of internal packages pinned the implementation; tests of the command pin the promises. The spec let the code split land with no test change, and it found two gaps when written: no --version, and lint --review losing moved lines.

## Rejected
Gherkin feature files: a second copy of the spec. Unit tests per package: they test the implementation, not what a user sees. testscript: a dependency.
