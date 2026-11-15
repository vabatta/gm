---
id: 20261115T000941Z-ba7d1c
type: decision
title: "Imports go through a Source port with one adapter per tool"
topic: decision/import-ports
keywords: import importer adapter port hexagonal source engram migration extend
at: [66335d54f580:internal/importer/importer.go, 66335d54f580:cmd/gm/import.go]
created: 2026-11-15T00:09:41Z
---
## Decision
internal/importer defines Source (Name, Read) and Record, and Import applies the shared rules once. Each tool is an adapter subpackage, such as internal/importer/engram; cmd/gm/import.go maps names to adapters for gm import SOURCE FILE.

## Why
Owner asked for a ports-and-adapters import so new sources are easy to add. The shared rules (ids, links, titles, secrets, migrated header) then cannot drift between sources, and an adapter only parses its format.

## Rejected
One import command per tool (import-engram): each would copy the shared rules. A plugin mechanism: no second source exists yet.
