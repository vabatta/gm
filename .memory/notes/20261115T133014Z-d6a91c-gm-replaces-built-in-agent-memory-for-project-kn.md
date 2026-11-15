---
id: 20261115T133014Z-d6a91c
type: decision
title: "gm replaces built-in agent memory for project knowledge"
topic: decision/scope
keywords: scope built-in auto memory claude codex copilot harness unified preferences
at: [29df42c1fe6b:skills/memory/SKILL.md, 29df42c1fe6b:README.md]
created: 2026-11-15T13:30:14Z
---
## Decision
Project knowledge lives only in gm notes, for every agent. Personal preferences are out of scope: each harness and team manages them its own way.

## Why
Built-in stores split the project per tool and per machine. Claude Code auto memory is machine-local, Codex memories are local and off by default, Copilot Memory is GitHub-only, per repo not per branch, and expires after 90 days. None survives a squash or links notes to code with a check. One store in git gives every agent and teammate the same notes on every branch.

## Rejected
gm alongside the built-ins with a routing rule for preferences: keeps two project memories and ties the skill to vendor features that change. Replacing gm with AGENTS.md plus Copilot Memory: AGENTS.md is loaded whole every session, and Copilot Memory expires and is not per branch.
