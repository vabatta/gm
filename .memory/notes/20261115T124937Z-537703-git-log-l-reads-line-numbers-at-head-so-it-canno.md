---
id: 20261115T124937Z-537703
type: fact
title: "git log -L reads line numbers at HEAD, so it cannot follow an old anchor's lines"
topic: fact/log-l-vs-blame-reverse
keywords: git log -L blame reverse review anchor lines moved shifted
at: [f45202feb63d:internal/anchor/review.go]
created: 2026-11-15T12:49:37Z
---
## Fact
git log -L N,M:path A..HEAD takes N and M in the file at HEAD. After lines are inserted above an anchor, it tracks other lines. git blame --reverse -L N,M A..HEAD -- path takes N and M at A and names the last commit where each line existed unchanged; a commit other than HEAD means the line changed.

## Evidence
Repro: anchor line 1 at C1; C2 inserts 3 lines above; C3 edits the anchored line. log -L reported C2 and missed C3. blame --reverse named C2 as the line's last unchanged commit, so it changed in C3.
