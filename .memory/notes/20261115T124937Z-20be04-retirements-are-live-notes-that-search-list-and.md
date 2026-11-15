---
id: 20261115T124937Z-20be04
type: decision
title: "Retirements are live notes that search, list and context show"
topic: decision/retirements-live
keywords: retire retirement live hidden search replaced withdrawn
at: [f45202feb63d:internal/memory/memory.go, f45202feb63d:internal/search/search.go]
created: 2026-11-15T12:49:37Z
---
## Decision
memory.Live keeps retirements; only notes that another note supersedes are hidden. Search marks only those as superseded.

## Why
A retirement's reason often says what replaced the retired notes, which is current knowledge. Hidden, it scored 0.00 on retired-vendor questions at 1k, 10k and 100k notes; live, 1.00 at rank 1 on 10k. Two eval agents also flagged the (superseded) label on retirements as wrong.

## Rejected
Keeping retirements hidden and telling agents to search with --all: --all drops rank-1 accuracy on current questions from 0.97 to 0.65 at 100k.
