---
id: 20261115T124937Z-82d205
type: decision
title: "The skill is named memory, not gm, so other skills route to it by name only"
topic: decision/skill-name
keywords: skill name routing memory agents boundary
at: [f45202feb63d:skills/memory/SKILL.md]
created: 2026-11-15T12:49:37Z
---
## Decision
Ship the skill as skills/memory and install it as ~/.claude/skills/memory.

## Why
Other skills say 'save it with the memory skill' and never name gm commands or flags, so they never change when gm changes. A tool-neutral name keeps that boundary if the binary is renamed.

## Rejected
Naming the skill gm: ties every routing sentence to the binary name.
