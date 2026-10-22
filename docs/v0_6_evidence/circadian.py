"""Compare availability models on 7 weeks of human message timestamps (rolling, no lookahead)."""
import json, math, random, statistics
from datetime import datetime, timedelta, timezone
ts = sorted(datetime.fromisoformat(t.replace("Z", "+00:00")) for _, t in json.load(open("human_ts.json")))
SAST = timedelta(hours=2)
H = lambda d: d.replace(minute=0, second=0, microsecond=0)
active = set(H(t) for t in ts)
start, end = H(ts[0]), H(ts[-1])
hours = []; h = start
while h <= end: hours.append(h); h += timedelta(hours=1)
print(f"events={len(ts)} span={(end-start).days}d active hours={len(active)}/{len(hours)} ({100*len(active)/len(hours):.0f}%)")
loc = [(t + SAST).hour for t in ts]
print("local hour histogram (SAST):", " ".join(f"{h:02d}:{loc.count(h)}" for h in range(24)))
# nights: longest daily gap
days = {}
for t in ts: days.setdefault((t + SAST - timedelta(hours=12)).date(), []).append(t)
gaps = []
for d, v in sorted(days.items()):
    v.sort(); g = max(((b - a), a) for a, b in zip(v, v[1:])) if len(v) > 1 else None
    if g: gaps.append(((g[1] + SAST).strftime("%H:%M"), round(g[0].total_seconds() / 3600, 1)))
print("longest gap per 'night' (start SAST, hours):", gaps[-21:])

def nxt(t):  # time to next human event after t
    for x in ts:
        if x > t: return (x - t).total_seconds() / 3600
    return None
TEST0 = start + timedelta(days=21)
def evaluate(name, pfun, sub=None):
    ll, br, n = 0, 0, 0
    for h in hours:
        if h < TEST0: continue
        p = min(max(pfun(h, sub), 1e-3), 1 - 1e-3); y = h in active
        ll += -math.log(p if y else 1 - p); br += (p - y) ** 2; n += 1
    return ll / n, br / n
def hod(h, sub, window=21, wk=False):
    a = b = 0
    for d in range(1, window + 1):
        x = h - timedelta(days=d if not wk else 7 * d)
        if x < start: break
        b += 1; a += x in (sub or active)
    return (a + 1) / (b + 2)
def hardcoded(h, sub):
    l = (h + SAST); m = l.hour + l.minute / 60
    return 0.05 if (m >= 23 or m < 7.5) else 0.6
def recency(h, sub):
    src = sub or active
    base = hod(h, sub)
    prev = h - timedelta(hours=1)
    return min(0.98, base * (2.2 if prev in src else 0.7))
base = sum(1 for h in hours if h >= TEST0 and h in active) / sum(1 for h in hours if h >= TEST0)
for name, f in (("constant base rate", lambda h, s: base), ("hardcoded 23:00-07:30", hardcoded),
                ("hour-of-day, 21d", hod), ("hour-of-week, 3wk", lambda h, s: hod(h, s, 3, True)), ("hour-of-day + last-hour", recency)):
    ll, br = evaluate(name, f)
    print(f"{name:26s} logloss={ll:.3f} brier={br:.3f}")
# sparse source: keep ~1 event in 30 (Telegram-like volume)
random.seed(1)
for keep in (0.3, 0.1, 0.03):
    sub = set(H(t) for t in ts if random.random() < keep)
    print(f"sparse {keep:>4}: hour-of-day logloss={evaluate('', hod, sub)[0]:.3f}  vs full {evaluate('', hod)[0]:.3f}; events kept={sum(1 for t in ts if H(t) in sub)}")
# reply latency prediction at escalation times: random test hours
lat_true, pred_hod, pred_hard = [], [], []
for h in hours:
    if h < TEST0: continue
    y = nxt(h)
    if y is None: continue
    lat_true.append(y)
    # hod model: expected wait = first future hour whose p>=0.5 (median-ish), cumulative
    surv, w = 1.0, 0
    for k in range(48):
        p = hod(h + timedelta(hours=k), None); 
        if surv * (1 - p) < 0.5: break
        surv *= (1 - p); w += 1
    pred_hod.append(w + 0.5)
    l = h + SAST; m = l.hour
    pred_hard.append(((7.5 - m) % 24) if (m >= 23 or m < 7.5) else 0.5)
mae = lambda p: statistics.mean(abs(a - b) for a, b in zip(p, lat_true))
print(f"reply-latency MAE hours: hour-of-day={mae(pred_hod):.2f} hardcoded={mae(pred_hard):.2f} always-0.5h={mae([0.5]*len(lat_true)):.2f}; true p50={statistics.median(lat_true):.2f}h p90={sorted(lat_true)[int(.9*len(lat_true))]:.1f}h")
night = [y for h, y in zip([h for h in hours if h >= TEST0 and nxt(h) is not None], lat_true) if (h + SAST).hour in (0, 1, 2)]
print(f"true wait when escalating at 00-03 SAST: p50={statistics.median(night):.1f}h, share answered within 1h={sum(y<1 for y in night)/len(night):.0%}")
