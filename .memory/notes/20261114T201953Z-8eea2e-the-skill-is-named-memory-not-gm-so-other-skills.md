---
id: 20261114T201953Z-8eea2e
type: decision
title: "The skill is named memory, not gm, so other skills route to it by name only"
topic: decision/skill-name
keywords: skill name routing memory agents boundary
at: [1c120516c3a9:skills/memory/SKILL.md]
created: 2026-11-14T20:19:53Z
---
## Decision
Ship the skill as skills/memory and install it as ~/.claude/skills/memory.

## Why
Other skills say 'save it with the memory skill' and never name gm commands or flags, so they never change when gm changes. A tool-neutral name keeps that boundary if the binary is renamed.

## Rejected
Naming the skill gm: ties every routing sentence to the binary name.
