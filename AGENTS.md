# gm

gm keeps project memory as note files in the repository they concern, merged by git. The manual is `docs/gm.adoc` (gm(1)) and `docs/gm-note.adoc` (gm-note(5)). The skill agents follow is `skills/memory/SKILL.md`. All three ship in the package, so they always match the binary.

## Rules of the code

1. A change goes manual first, then spec, then code. Describe the behaviour in `docs/*.adoc`, add one subtest per new sentence to the spec file of that section, `cmd/gm/<section>_test.go`, and watch it fail, then implement until it passes.
2. Behaviour lives in the manual and the spec. The spec runs the real `gm` command against throwaway git repositories and checks only what a user sees: exit status, output and files. It never calls internal packages. Write no plan or ADR; record a design decision as a gm note in `.memory/notes/` (`gm save decision ...`).
3. One file per command in `cmd/gm`, one file per concern in each internal package (for example `note`: format, rules). A file is read whole, so keep it to one idea.
4. Doc comments on every exported identifier, Go standard library style. A comment says what the code does and why, true to the current code.
5. Stay one static binary on the Go standard library alone (`CGO_ENABLED=0`, no modules in `go.mod`). git is the only runtime dependency. Prefer a standard library call to custom code.
6. The note format is a contract with every repository that holds notes. Change it only in ways older notes still parse.

## Rules of the work

1. Work in the dev shell: `direnv allow` once (the `.envrc` loads the flake), or `nix develop`.
2. Done means `nix flake check` passes. It builds the binary and runs `go test ./...` in the sandbox. `vendorHash` is null because there are no dependencies.
3. Before handing over, run `gm lint` in this repository.
4. Measure a change to search, lint, show or pruning with `bench/run.sh 10000 result/bin/gm` before and after, and quote both in the pull request. The benchmark generates a corpus with known answers, so it reports speed and whether search still finds the answer.
