"""Generate a gm memory corpus for a fictional logistics company, with ground-truth questions.

Usage: python3 -I gen.py SIZE OUTDIR
Writes OUTDIR/notes.json (every note with its era) and OUTDIR/questions.json.
"""
import json, os, random, sys
from datetime import datetime, timedelta, timezone

SIZE, OUT = int(sys.argv[1]), sys.argv[2]
R = random.Random(SIZE)

DOMAIN = "billing invoice ledger payout refund route dispatch fleet driver depot parcel label customs tariff warehouse pallet inventory stock pricing quote booking carrier tracking geocode address eta notification sms email webhook audit report forecast demand capacity slot returns claims insurance fuel toll telematics shift payroll vendor contract".split()
NOUN = "api worker gateway sync planner scheduler indexer exporter importer service engine store bridge collector notifier resolver validator aggregator".split()
TEAMS = "Atlas Borealis Comet Delta Ember Fjord Granite Harbor Iris Juniper Kestrel Lumen Meridian Nimbus Orchid Pioneer Quartz Raven Summit Tundra".split()

DECISIONS = {
    "datastore": ("primary datastore", "database datastore storage persistence db",
        {"Postgres": ("hit its connection limit at peak", "transactions and joins the team already knows"),
         "MySQL": ("replication lag broke read-after-write", "mature replication across regions"),
         "DynamoDB": ("hot partitions throttled writes", "single-digit millisecond reads at any scale"),
         "MongoDB": ("unbounded documents caused slow queries", "flexible documents for nested shipments"),
         "CockroachDB": ("cross-region writes added 200 ms", "a multi-region SQL database without manual sharding"),
         "Cassandra": ("tombstones made reads time out", "high write throughput for append-only events")}),
    "queue": ("message queue", "queue messaging broker events pubsub",
        {"Kafka": ("operating the cluster took a full engineer", "replayable ordered event logs"),
         "SQS": ("no ordering across message groups", "zero operations and pay per message"),
         "NATS": ("lost messages on restarts without JetStream", "very low latency fan-out"),
         "RabbitMQ": ("queues backed up and blocked publishers", "flexible routing with exchanges"),
         "Google Pub/Sub": ("duplicate deliveries forced dedup everywhere", "global topics without managing brokers")}),
    "deploy": ("deploy platform", "deploy hosting platform infrastructure runtime cluster",
        {"GKE": ("node upgrades restarted pods during peak", "autopilot clusters with little ops"),
         "EKS": ("IAM wiring for pods was hard to get right", "staying inside the AWS account with the data"),
         "ECS Fargate": ("cold starts of tasks slowed scale-out", "no nodes to manage"),
         "Fly.io": ("private networking dropped database connections", "cheap single-region machines"),
         "Nomad": ("the scheduler had no expert left on the team", "a simple scheduler for batch jobs"),
         "Cloud Run": ("request timeouts cut long exports", "scale to zero between bursts")}),
    "cache": ("cache", "cache caching memoization lookup",
        {"Redis": ("evicted keys under memory pressure", "shared cache across pods"),
         "Memcached": ("no persistence made warmups slow", "simple multithreaded caching"),
         "an in-process LRU": ("pods disagreed on cached values", "no network hop"),
         "Varnish": ("cache purges were slow to propagate", "HTTP caching in front of the API")}),
    "auth": ("authentication method", "auth authentication login tokens security",
        {"JWT bearer tokens": ("revoking tokens early was impossible", "stateless verification"),
         "session cookies": ("mobile clients handled cookies badly", "easy revocation"),
         "mTLS": ("certificate rotation caused outages", "strong service identity"),
         "API keys": ("leaked keys could not be scoped", "simple for partners"),
         "OAuth2 client credentials": ("token endpoint became a bottleneck", "standard scoped tokens for partners")}),
    "serialization": ("wire format", "serialization format encoding wire payload",
        {"JSON": ("payloads were three times larger than needed", "readable and universal"),
         "Protobuf": ("schema changes needed coordinated deploys", "compact typed messages"),
         "Avro": ("the schema registry was a single point of failure", "schema evolution with a registry"),
         "MessagePack": ("clients lacked good libraries", "compact without schemas")}),
    "language": ("implementation language", "language runtime rewrite codebase",
        {"Go": ("generics came too late for the data model", "fast builds and simple deploys"),
         "Rust": ("compile times slowed every change", "predictable latency without a garbage collector"),
         "Kotlin": ("JVM startup slowed autoscaling", "rich libraries on the JVM"),
         "TypeScript on Node 22": ("CPU-bound work blocked the event loop", "sharing types with the web app"),
         "Python 3.12": ("the GIL limited throughput", "fast iteration and data libraries"),
         "Java 21": ("memory use was high per pod", "virtual threads for blocking IO")}),
}
FACTS = {
    "retries": ("retry policy", "retry retries backoff resilience upstream", lambda: f"{R.randint(2, 8)} retries with {R.choice([100, 200, 250, 500, 1000])} ms exponential backoff"),
    "timeout": ("request timeout", "timeout deadline latency limit", lambda: f"{R.choice([2, 3, 5, 8, 10, 15, 20, 30, 45, 60])} seconds"),
    "ratelimit": ("rate limit", "rate limit throttling quota requests", lambda: f"{R.choice([50, 100, 200, 250, 500, 800, 1000, 2000, 5000])} requests per second"),
    "owner": ("owning team", "owner team ownership oncall responsible", lambda: f"team {R.choice(TEAMS)}"),
}
VENDORS = {"LaunchDarkly": "feature flags", "Sentry": "error tracking", "PagerDuty": "paging", "Datadog": "metrics",
           "Segment": "event tracking", "Algolia": "search", "Twilio": "SMS", "Mailgun": "email", "Auth0": "login"}
REPLACEMENTS = {"feature flags": ["a flags table edited in the admin UI", "Unleash self-hosted"], "error tracking": ["GlitchTip self-hosted", "structured logs in Loki"],
                "paging": ["Grafana OnCall", "Opsgenie"], "metrics": ["Prometheus and Grafana", "VictoriaMetrics"], "event tracking": ["RudderStack", "direct writes to BigQuery"],
                "search": ["Postgres full-text search", "Meilisearch"], "SMS": ["MessageBird", "Vonage"], "email": ["Amazon SES", "Postmark"], "login": ["Keycloak", "Zitadel"]}
FAILURES = [("a deploy dropped a column old pods still read", "keep migrations backward compatible for one release"),
            ("a retry storm overloaded the upstream after an outage", "cap retries and add jitter"),
            ("a full disk stopped the write-ahead log", "alert at 70 percent disk usage"),
            ("a clock skew of 4 minutes expired tokens early", "run chrony on every node and alert on skew"),
            ("a config typo routed all traffic to one region", "validate config in CI before deploy"),
            ("an expired TLS certificate broke partner calls", "renew certificates automatically 30 days ahead"),
            ("a slow query locked the orders table", "set statement timeouts on every connection"),
            ("a cache stampede followed a cold restart", "warm the cache before taking traffic"),
            ("duplicate webhook deliveries charged customers twice", "make handlers idempotent by event id"),
            ("a timezone bug shifted delivery slots by one hour", "store UTC and convert only when rendering")]

START = datetime(2024, 1, 1, tzinfo=timezone.utc)
SPAN = (datetime(2026, 10, 31, tzinfo=timezone.utc) - START).total_seconds()
notes, questions, used = [], [], set()

def when(frac):
    return START + timedelta(seconds=SPAN * min(max(frac, 0), 0.9999))

def nid(t):
    return t.strftime("%Y%m%dT%H%M%SZ") + "-" + "%06x" % R.getrandbits(24)

def add(t, typ, title, keywords, sections, topic="", supersedes=(), extra=None):
    n = dict(id=nid(t), created=t.strftime("%Y-%m-%dT%H:%M:%SZ"), type=typ, title=title, keywords=keywords,
             topic=topic, supersedes=list(supersedes), sections=sections, frac=(t - START).total_seconds() / SPAN)
    notes.append(n)
    return n

def service_name():
    while True:
        s = f"{R.choice(DOMAIN)}-{R.choice(DOMAIN)}-{R.choice(NOUN)}"
        if s not in used and s.split("-")[0] != s.split("-")[1]:
            used.add(s)
            return s

while len(notes) < SIZE:
    svc = service_name()
    for attr in R.sample(sorted(DECISIONS), R.randint(2, 4)):
        label, kw, vals = DECISIONS[attr]
        chain = R.sample(sorted(vals), R.choices([1, 2, 3], [5, 4, 2])[0])
        times = sorted(R.random() for _ in chain)
        prev, heads = None, []
        for i, v in enumerate(chain):
            problem, strength = vals[v]
            why = f"{v} gives {strength}."
            names_prev = prev is not None and R.random() < 0.5
            if names_prev:
                why = f"{prev['value']} {vals[prev['value']][0]}. " + why
            alts = [a for a in sorted(vals) if a not in chain]
            rej = R.choice(alts) if alts else "Keeping the status quo"
            n = add(when(times[i]), "decision", f"{svc} uses {v} as its {label}", f"{svc} {kw} {v.lower()}",
                    {"Decision": f"{svc} uses {v} as its {label}.", "Why": why,
                     "Rejected": f"{rej}: {vals[rej][0] if rej in vals else 'it did not fix the problem'}."},
                    topic=f"decision/{svc}-{attr}", supersedes=[prev["id"]] if prev else [])
            n["value"], n["names_prev"] = v, names_prev
            heads.append(n)
            prev = n
        cur = heads[-1]
        questions.append(dict(kind="current", q=f"What does {svc} use as its {label} now?", gold=[cur["value"]], notes=[cur["id"]]))
        if len(heads) > 1:
            p = heads[-2]
            questions.append(dict(kind="previous", q=f"What did {svc} use as its {label} before the current choice?", gold=[p["value"]],
                                  notes=[p["id"]] + ([cur["id"]] if cur["names_prev"] else []), live_has_answer=cur["names_prev"]))
            if cur["names_prev"]:
                questions.append(dict(kind="why", q=f"Why did {svc} stop using {p['value']} as its {label}?", gold=[DECISIONS[attr][2][p['value']][0]], notes=[cur["id"]]))
    for attr in R.sample(sorted(FACTS), R.randint(1, 3)):
        label, kw, gen = FACTS[attr]
        k = R.choices([1, 2, 3], [5, 4, 2])[0]
        vals, times, prev, chainf = [], sorted(R.random() for _ in range(k)), None, []
        for i in range(k):
            v = gen()
            while v in vals:
                v = gen()
            vals.append(v)
            n = add(when(times[i]), "fact", f"{svc} {label} is {v}", f"{svc} {kw}",
                    {"Fact": f"The {label} of {svc} is {v}.", "Evidence": f"Measured in production on {when(times[i]).date()}, dashboard {svc}."},
                    topic=f"fact/{svc}-{attr}", supersedes=[prev["id"]] if prev else [])
            n["value"] = v
            prev = n
            chainf.append(n["id"])
        questions.append(dict(kind="fact", q=f"What is the {label} of {svc}?", gold=[vals[-1]], notes=[prev["id"]]))
        if k > 1:
            questions.append(dict(kind="history", q=f"How has the {label} of {svc} changed over time?", gold=vals,
                                  notes=chainf, live_has_answer=False))
    if R.random() < 0.3:
        vendor = R.choice(sorted(VENDORS))
        use = VENDORS[vendor]
        t0 = R.random() * 0.7
        d = add(when(t0), "decision", f"{svc} uses {vendor} for {use}", f"{svc} {vendor.lower()} {use} vendor saas",
                {"Decision": f"{svc} sends its {use} to {vendor}.", "Why": f"{vendor} was the fastest way to get {use}.", "Rejected": f"Building {use} in house: no time."})
        if R.random() < 0.6:
            repl = R.choice(REPLACEMENTS[use])
            r = add(when(t0 + R.random() * (1 - t0)), "retirement", f"Retired: {svc} uses {vendor} for {use}", f"retired retirement obsolete {svc} {vendor.lower()} {use}",
                    {"Why": f"{svc} no longer uses {vendor}: {use} now goes to {repl}. {vendor} cost {R.choice([200, 400, 900, 1500])} USD a month."}, supersedes=[d["id"]])
            questions.append(dict(kind="retired", q=f"Does {svc} still use {vendor} for {use}? If not, what replaced it?", gold=[repl], notes=[r["id"]], live_has_answer=False))
    for _ in range(R.choices([0, 1, 2], [5, 4, 1])[0]):
        inc = f"INC-{R.randint(1000, 99999)}"
        fail, rule = R.choice(FAILURES)
        add(when(R.random()), "lesson", f"{inc}: {svc} {fail}", f"{svc} incident outage {inc.lower()} postmortem",
            {"What happened": f"In {inc}, {fail} in {svc}.", "Cause": "The failure mode was not covered by a check.", "Rule now": rule[0].upper() + rule[1:] + "."})
        questions.append(dict(kind="lesson", q=f"What rule came out of incident {inc}?", gold=[rule], notes=[notes[-1]["id"]]))

notes.sort(key=lambda n: n["created"])
os.makedirs(OUT, exist_ok=True)
json.dump(notes, open(f"{OUT}/notes.json", "w"))
json.dump(questions, open(f"{OUT}/questions.json", "w"))
print(len(notes), "notes,", len(questions), "questions,", len(used), "services")
