---
id: 20261115T124937Z-6250cb
type: decision
title: "The skill ships inside the gm package so binary and skill always match"
topic: decision/skill-packaging
keywords: skill package version share install home-manager
at: [f45202feb63d:package.nix, f45202feb63d:nix/home-manager.nix]
created: 2026-11-15T12:49:37Z
---
## Decision
package.nix copies skills/ to share/gm/skills; the home-manager module links ~/.claude/skills/memory from the package.

## Why
The skill names commands and flags. Shipping it from the same derivation means an override of the package also overrides the skill.

## Rejected
Linking the skill from the flake source: a package override would leave a skill describing another version.
