---
name: memory
description: "Saves and recalls project memory as note files committed in the repository, through the gm command, so notes travel with branches and align when they merge. Use when starting work in a repo, before reading code to answer a question the project may already have answered, and after a decision, lesson, or fact that a later agent would otherwise rediscover. Trigger on remember this, what did we decide, have we seen this before, save a note, or recall."
---

# Memory

Project memory lives in the repository it is about, one note per file in `.memory/notes/`. Git merges notes like code. `gm` writes, searches, and checks them. Run `gm help` for every command and flag. Add `--json` to `search`, `show`, `list`, and `lint` when you parse the output.

## Set up once per repository

When `.memory/notes/` does not exist and the work produced something worth saving, run `gm init` at the repository root and commit the `.gitignore` change with the first note. Done when `.gitignore` lists `.memory/index.sqlite`.

## Recall before you work

At the start of a task, and before reading code to answer a question, run `gm search` with the words of the question. When the first search misses, search again with two rewordings: synonyms, the module name, the error text. Read a hit with `gm show ID`, which prints the note and the code its anchors point at, read from the commit the note was written against. Done when a note answers the question or three searches found nothing.

A note is evidence from the time it was written, not a fact about the code now. When `gm show` and the current code disagree, the code wins. Save a revision.

## Save after you learn

Save when the work produced something a later agent would otherwise rediscover: a decision with its reason and the option rejected, a lesson from a failure, or a measured fact. Skip what the code, a test name, or a doc comment already says.

1. Commit the code the note is about. `gm save --at` refuses files with uncommitted changes, so the anchor shows the code the note describes. Done when `git status` is clean for those files.
2. Pick the type: `decision` (decision, why, rejected), `lesson` (happened, cause, rule), or `fact` (fact, evidence). Write the title as a one-line claim. Write at least three `--keywords`, including the words someone would search with that the note itself does not use. Anchor every file the note is about with `--at path[:from-to]`, the path taken from the repository root. Give a `--topic` when the note may be revised later. Done when every flag of the type has a value.
3. Run `gm save TYPE ...` from the repository root. When it refuses, fix every listed problem and run it again. When it warns that the topic has other live notes, decide whether the new note replaces them, and if so save it again with `--supersedes ID` for each one it replaces. Done when `gm save` prints the new note's id and path.
4. Commit the note on the same branch as the work. Done when the note file is committed.

A note is never edited. To change one, save a new note with `--supersedes ID`. To withdraw one with no successor, run `gm retire ID --why "..."`. Done when the new note or retirement is committed.

## Check before you hand over

Run `gm lint` after merging another branch and before opening a pull request. It fails on parallel revisions of one topic, anchors whose commit left the history or whose file is gone, broken supersedes, duplicate ids, empty template sections, and text that looks like a secret. Fix each by saving a note: one that supersedes the parallel revisions, or a revision anchored to the code as it is now. Run `gm lint --review` to list notes whose anchored lines changed since, and re-read those. Done when `gm lint` exits 0.

## Never

- Never edit or delete a note file by hand. Notes are the history.
- Never commit `.memory/index.sqlite`. It is a local index rebuilt from the notes.
- Never put a secret, token, or personal data in a note.
- Never squash or rebase a branch whose notes anchor its commits. Merge it, so the anchored commits stay in the history.
