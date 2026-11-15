---
id: 20261115T130457Z-6da924
type: decision
title: "Agents reach memory through the skill; there is no session-start hook"
topic: decision/no-session-hook
keywords: hook sessionstart context recall startup engram mem_context titles
at: [46315e9d4393:nix/home-manager.nix, 46315e9d4393:skills/memory/SKILL.md]
created: 2026-11-15T13:04:57Z
---
## Decision
gm has no context command, and the home-manager module adds no SessionStart hook. Agents find notes because the memory skill tells them to search first.

## Why
Container trial, 12 benchmark questions per arm on Sonnet: with a hook printing the 15 newest titles, 11/12 correct and 0/12 loaded the skill; without it, 12/12 correct, 12/12 found the notes, 9/12 via the skill. The hook replaced the skill, and the one miss was a history question that needs --all, which only the skill explains. The hook saved about 1.6 turns per question.

## Rejected
Keeping the hook, as Engram's mem_context does: recent titles are not the task's notes, and agents skip the skill. Keeping gm context as a manual command: unproven use, so it goes until a need shows up.
