---
id: 20261114T201953Z-0cae2c
type: decision
title: "The skill ships inside the gm package so binary and skill always match"
topic: decision/skill-packaging
keywords: skill package version share install home-manager
at: [1c120516c3a9:package.nix, 1c120516c3a9:nix/home-manager.nix]
created: 2026-11-14T20:19:53Z
---
## Decision
package.nix copies skills/ to share/gm/skills; the home-manager module links ~/.claude/skills/memory from the package.

## Why
The skill names commands and flags. Shipping it from the same derivation means an override of the package also overrides the skill.

## Rejected
Linking the skill from the flake source: a package override would leave a skill describing another version.
