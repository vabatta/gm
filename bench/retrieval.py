"""Score gm search against the ground truth that gen.py wrote.

Usage: python3 -I retrieval.py DIR GM [PER_KIND]
For a stratified sample of questions, run `gm search QUESTION -n 5 --json`, with
and without --all, in the full and the pruned copy. A question is answered at k
when every gold string appears in the text of the top k notes.
"""
import json, os, random, subprocess, sys
from concurrent.futures import ThreadPoolExecutor

DIR, GM = sys.argv[1], sys.argv[2]
PER = int(sys.argv[3]) if len(sys.argv) > 3 else 40
qs = json.load(open(f"{DIR}/questions.json"))
R = random.Random(7)
by = {}
for q in qs:
    by.setdefault(q["kind"], []).append(q)
sample = [q for k in sorted(by) for q in R.sample(by[k], min(PER, len(by[k])))]
json.dump(sample, open(f"{DIR}/sample.json", "w"))

def run(args):
    copy, q, all_ = args
    repo = f"{DIR}/{copy}/nw"
    cmd = [GM, "search", q["q"], "-n", "5", "--json"] + (["--all"] if all_ else [])
    hits = json.loads(subprocess.run(cmd, cwd=repo, capture_output=True, text=True).stdout or "[]") or []
    def text(h):
        if not h.get("pruned_in"):
            return open(f"{repo}/{h['file']}").read().lower()
        n = json.loads(subprocess.run([GM, "show", h["id"], "--json"], cwd=repo, capture_output=True, text=True).stdout)
        return (n["title"] + "\n" + n["body"]).lower()
    texts = [text(h) for h in hits]
    gold = [g.lower() for g in q["gold"]]
    ans = lambda k: all(any(g in t for t in texts[:k]) for g in gold)
    return copy, all_, q["kind"], ans(1), ans(5)

jobs = [(c, q, a) for c in ["full", "pruned"] for a in [False, True] for q in sample]
with ThreadPoolExecutor(24) as ex:
    res = list(ex.map(run, jobs))
kinds = sorted(by)
print(f"{'kind':10} {'n':>3} | " + " | ".join(f"{c}{' --all' if a else ''}".ljust(14) for c in ["full", "pruned"] for a in [False, True]))
for k in kinds + ["ALL"]:
    row = []
    for c in ["full", "pruned"]:
        for a in [False, True]:
            rs = [r for r in res if r[0] == c and r[1] == a and (k == "ALL" or r[2] == k)]
            row.append(f"@1 {sum(r[3] for r in rs)/len(rs):.2f} @5 {sum(r[4] for r in rs)/len(rs):.2f}")
    n = len([q for q in sample if k == "ALL" or q["kind"] == k])
    print(f"{k:10} {n:>3} | " + " | ".join(row))
