# gm

gm keeps project memory as note files in the repository they concern, merged by git. `gm help` lists the commands. The skill agents follow is `skills/memory/SKILL.md`; it ships in the package so binary and skill always match.

## Rules of the code

1. The code is the source of truth. Behaviour lives in test names and types. Write no plan, ADR, or doc that restates code; record a design decision as a gm note in `.memory/notes/` (`gm save decision ...`).
2. Doc comments on every exported identifier, Go standard library style. Comments state constraints the code cannot express, nothing more.
3. Few tests, each named for the promise it keeps, run against real temporary git repositories (`internal/gittest`). A new promise gets one test.
4. Stay one static binary: pure Go (`CGO_ENABLED=0`), standard library plus `modernc.org/sqlite`. git is the only runtime dependency.
5. The note format is a contract with every repository that holds notes. Change it only in ways older notes still parse.

## Rules of the work

1. Work in the dev shell: `nix develop`.
2. Done means `nix flake check` passes. It builds the binary and runs `go test ./...` in the sandbox. After a dependency change, update `vendorHash` in `package.nix` from the hash nix reports.
3. Before handing over, run `gm lint` in this repository.
4. Merge branches; never squash or rebase them. Notes anchor commits, and lint fails when a squash drops them.
