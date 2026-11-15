---
id: 20261115T124937Z-7729fd
type: decision
title: "Imports go through a Source port with one adapter per tool"
topic: decision/import-ports
keywords: import importer adapter port hexagonal source engram migration extend
at: [f45202feb63d:internal/importer/importer.go, f45202feb63d:internal/importer/engram/engram.go, f45202feb63d:cmd/gm/import.go]
created: 2026-11-15T12:49:37Z
---
## Decision
internal/importer defines Source (Name, Read) and Record, and Import applies the shared rules once. Each tool is an adapter subpackage, such as internal/importer/engram; cmd/gm/import.go maps names to adapters for gm import SOURCE FILE.

## Why
The rules that make a record a note (new ids, supersedes links, title shortening, the migrated header) are written once in Import, so they cannot drift between sources, and an adapter only parses its format. Adding a source is one adapter package and one entry in the sources table.

## Rejected
One import command per tool (import-engram): each would copy the shared rules. A plugin mechanism: no second source exists yet.
