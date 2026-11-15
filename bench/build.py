"""Build the full and the pruned repository from OUTDIR/notes.json.

Usage: python3 -I build.py OUTDIR GM
Notes are committed one quarter at a time. At 80 % of the timeline the pruned
copy runs `GM prune` and commits the deletions. Both copies then get the
remaining notes in weekly commits, which buries the prune commit under later
history.
"""
import json, os, re, subprocess, sys

OUT, GM = sys.argv[1], sys.argv[2]
notes = json.load(open(f"{OUT}/notes.json"))
ORDER = {"decision": ["Decision", "Why", "Rejected"], "lesson": ["What happened", "Cause", "Rule now"],
         "fact": ["Fact", "Evidence"], "retirement": ["Why"]}
ENV = dict(os.environ, GIT_CONFIG_GLOBAL="/dev/null", GIT_CONFIG_NOSYSTEM="1", GIT_AUTHOR_NAME="dev", GIT_AUTHOR_EMAIL="dev@nw.test",
           GIT_COMMITTER_NAME="dev", GIT_COMMITTER_EMAIL="dev@nw.test")

def fname(n):
    s = re.sub(r"[^a-z0-9]+", "-", n["title"].lower()).strip("-")[:48].rstrip("-")
    return f"{n['id']}-{s}.md"

def render(n):
    h = ["---", "id: " + n["id"], "type: " + n["type"], "title: " + json.dumps(n["title"], ensure_ascii=False)]
    if n["topic"]: h.append("topic: " + n["topic"])
    if n["keywords"]: h.append("keywords: " + n["keywords"])
    if n["supersedes"]: h.append("supersedes: [" + ", ".join(n["supersedes"]) + "]")
    h.append("created: " + n["created"])
    body = "".join(f"## {s}\n{n['sections'][s]}\n\n" for s in ORDER[n["type"]]).strip() + "\n"
    return "\n".join(h + ["---"]) + "\n" + body

def git(repo, *args):
    subprocess.run(["git", *args], cwd=repo, env=ENV, check=True, stdout=subprocess.DEVNULL)

def commit(repo, batch, msg):
    d = f"{repo}/.memory/notes"
    os.makedirs(d, exist_ok=True)
    for n in batch:
        open(f"{d}/{fname(n)}", "w").write(render(n))
    git(repo, "add", "-A")
    git(repo, "commit", "-q", "-m", msg)

PRUNE = 0.8
early = [n for n in notes if n["frac"] < PRUNE]
late = [n for n in notes if n["frac"] >= PRUNE]
for copy in ["full", "pruned"]:
    repo = f"{OUT}/{copy}/nw"
    os.makedirs(repo, exist_ok=True)
    git(repo, "init", "-q", "-b", "main")
    open(f"{repo}/README.md", "w").write("# Northwind Logistics platform\n")
    commit(repo, [], "start")
    quarters = {}
    for n in early:
        quarters.setdefault(n["created"][:4] + "Q" + str((int(n["created"][5:7]) - 1) // 3 + 1), []).append(n)
    for q, batch in quarters.items():
        commit(repo, batch, f"memory: notes from {q}")
    if copy == "pruned":
        subprocess.run([GM, "prune"], cwd=repo, env=ENV, check=True, stdout=subprocess.DEVNULL)
        git(repo, "add", "-A")
        git(repo, "commit", "-q", "-m", "memory: gm prune")
    weeks = {}
    for n in late:
        weeks.setdefault(n["created"][:8] + str(int(n["created"][8:10]) // 7), []).append(n)
    for w, batch in weeks.items():
        commit(repo, batch, f"memory: notes from week of {batch[0]['created'][:10]}")
print(f"{len(notes)} notes; {len(os.listdir(OUT + '/pruned/nw/.memory/notes'))} left after pruning; {len(weeks)} later commits")
