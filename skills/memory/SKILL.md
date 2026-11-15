---
name: memory
description: "Saves and recalls project memory as note files committed in the repository, through the gm command, so notes travel with branches and merge with them. Use when starting work in a repo, before reading code to answer a question the project may already have answered, and after a decision, lesson, or fact that a later agent would otherwise rediscover. Trigger on remember this, what did we decide, have we seen this before, save a note, or recall."
---

# Memory

Project memory lives in the repository it is about, one note per file in `.memory/notes/`. Git merges notes like code. `gm` writes, searches, and checks them. Run `gm help` for the commands, and `man gm` for every rule they apply. Add `--json` to `search`, `show`, `list`, and `lint` when you parse the output.

## Recall before you work

At the start of a task, and before reading code to answer a question, run `gm search` with the words of the question. When the first search misses, search again with two rewordings that use synonyms, the module name, or the error text. For what came before or how something changed, search with `--all`, which adds superseded notes, also those pruned into git history. Read a hit with `gm show ID`. It prints the note and the code its anchors point at, as that code was when the note was written. Done when a note answers the question or three searches found nothing.

A note is evidence from the time it was written, not a fact about the code now. When `gm show` and the current code disagree, the code wins. Save a revision.

## What a memory is

A memory is knowledge about the project that a later agent needs and cannot get from the repository. The repository is the code, the tests, the docs, AGENTS.md, and `git log`. A memory is one of three things, and nothing else:

- **Why, against the alternative.** Why the code is this way and not the obvious other way, with the evidence that ruled the other way out. Without it, a later agent "fixes" the code back.
- **What failed.** A failure, its cause, and the rule that prevents it.
- **What holds outside the repository.** How a tool, service, or platform that the code depends on behaves, observed or measured.

Test every note before you save it: if the note were deleted, would an agent that reads the code, the tests, the docs, and `git log` make a worse change? If not, do not save it.

Never save:

- what the code, a test name, a doc comment, the manual, or AGENTS.md already says;
- a summary of what a commit, a task, or a session did. The commit message holds it;
- a number that a script in the repository reproduces, such as a benchmark timing;
- a request without its reason. A standing rule from a person goes in AGENTS.md;
- a plan, a task list, or work in progress;
- a personal preference or how one person likes to work.

gm is the project's one memory for every agent. Save project knowledge with `gm save`, never in the agent's built-in memory, so every agent and every teammate reads the same notes.

In a decision, the Decision section names the choice in one line. The Why and Rejected sections are the memory. Most tasks produce no note. One note a reader needs is worth more than ten they skip.

## Save after you learn

Save when the work taught you something that passes the test above.

1. Commit the code the note is about. `gm save --at` refuses files with uncommitted changes, so the anchor shows the code the note describes. Done when `git status` is clean for those files.
2. Pick the type: `decision` (decision, why, rejected), `lesson` (happened, cause, rule), or `fact` (fact, evidence). Write the title as a one-line claim. Write at least three `--keywords`, including the words someone would search with that the note itself does not use. Anchor every file the note is about with `--at path[:from-to]`, with the path relative to the repository root. Give a `--topic` when a later note may revise this one. Done when every flag of the type has a value.
3. Run `gm save TYPE ...` from the repository root. When it refuses, fix every listed problem and run it again. When it warns that the topic has other live notes, decide whether the new note replaces them, and if so save it again with `--supersedes ID` for each one it replaces. Done when `gm save` prints the new note's id and path.
4. Commit the note on the same branch as the work. Done when the note file is committed.

Do not edit a note. To change one, save a new note with `--supersedes ID`. To withdraw one with no successor, run `gm retire ID --why "..."`, and say in the reason what replaced it. Done when the new note or retirement is committed.

`gm prune` deletes superseded notes from the working tree. `gm show` and `--all` still read them from git history. Prune only when the person asks, then commit the deletions. Done when the prune commit is in.

## Check before you hand over

Run `gm lint` after merging another branch and before opening a pull request. It fails on parallel revisions of one topic, anchors whose file is gone, notes on the branch whose anchored code changed after them, broken supersedes, duplicate ids and empty template sections. Fix each by saving a note: one that supersedes the parallel revisions, or a revision anchored to the code as it is now. Run `gm lint --review` to list notes whose anchored lines changed since, and re-read those. Done when `gm lint` exits 0.

## Never

- Never edit or delete a note file by hand. Notes are the history.
- Never put a secret, token, or personal data in a note.
