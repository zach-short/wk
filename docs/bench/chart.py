"""Draw the row 19 charts and SUMMARY.md from the CSVs in docs/bench/data/.

Python 3 standard library only. Every number in a chart or in SUMMARY.md comes from a CSV
row; nothing is typed here except labels. Run: python3 docs/bench/chart.py
"""

import csv
import html
import math
import os
import statistics
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
DATA = os.environ.get("BENCH_DATA", os.path.join(HERE, "data"))
OUT = os.environ.get("BENCH_OUT", HERE)
MACHINE = "Apple M5 Pro, 15 cores, 64 GiB, APFS"

# Mid-tone colors on a white card, so each chart reads the same on a light or dark page.
CARD = "#ffffff"
BORDER = "#d0d7de"
INK = "#1f2328"
MUTED = "#57606a"
GRID = "#eaeef2"
COLORS = {
    "baseline": "#6e7781",
    "wk_template": "#0969da",
    "wk_sparse": "#0969da",
    "wk_mp2": "#0969da",
    "wk_pool": "#1a7f37",
    "wk_pool_mp2": "#1a7f37",
    "wk_full": "#8250df",
    "wk_cold": "#bc4c00",
    "wk_mp10": "#cf222e",
    "plain": "#bc4c00",
    "slotted": "#0969da",
}
LABELS = {
    "baseline": "git worktree add + manual setup",
    "wk_template": "wk new, warm template",
    "wk_sparse": "wk new (sparse)",
    "wk_full": "wk new --full",
    "wk_pool": "wk new, pool slot",
    "wk_cold": "wk new, cold template",
    "wk_mp2": "wk new, max_parallel 2",
    "wk_pool_mp2": "wk new, pool of 4, max_parallel 2",
    "wk_mp10": "wk new, max_parallel 10",
    "plain": "6 plain builds",
    "slotted": "6 builds through wk run --slot heavy (2 places)",
}


def read(name):
    path = os.path.join(DATA, name)
    if not os.path.exists(path):
        return []
    with open(path, newline="") as f:
        return list(csv.DictReader(f))


def fnum(v):
    try:
        return float(v)
    except (TypeError, ValueError):
        return None


def fmt(v, nd=1):
    return f"{v:.{nd}f}"


def caption(rows):
    dates = sorted({r["started_at"][:10] for r in rows if r.get("started_at")})
    commits = sorted({r["wk_commit"] for r in rows if r.get("wk_commit")})
    versions = sorted({r["wk_version"] for r in rows if r.get("wk_version")})
    d = dates[0] if len(dates) == 1 else (f"{dates[0]} to {dates[-1]}" if dates else "")
    return f"{MACHINE}. {d}. wk {', '.join(versions)} ({', '.join(commits)}). Results hold for this machine only."


class Svg:
    def __init__(self, w, h):
        self.w, self.h, self.parts = w, h, []

    def add(self, s):
        self.parts.append(s)

    def text(self, x, y, s, size=12, color=INK, anchor="start", weight="normal"):
        self.add(
            f'<text x="{x:.1f}" y="{y:.1f}" font-size="{size}" fill="{color}" text-anchor="{anchor}" font-weight="{weight}">{html.escape(str(s))}</text>'
        )

    def rect(self, x, y, w, h, color, rx=0):
        self.add(f'<rect x="{x:.1f}" y="{y:.1f}" width="{max(w, 0):.1f}" height="{max(h, 0):.1f}" fill="{color}" rx="{rx}"/>')

    def line(self, x1, y1, x2, y2, color, width=1, dash=None):
        d = f' stroke-dasharray="{dash}"' if dash else ""
        self.add(f'<line x1="{x1:.1f}" y1="{y1:.1f}" x2="{x2:.1f}" y2="{y2:.1f}" stroke="{color}" stroke-width="{width}"{d}/>')

    def write(self, name, title):
        head = (
            f'<svg xmlns="http://www.w3.org/2000/svg" width="{self.w}" height="{self.h}" viewBox="0 0 {self.w} {self.h}" '
            f'font-family="-apple-system, BlinkMacSystemFont, Segoe UI, Helvetica, Arial, sans-serif" role="img" aria-label="{html.escape(title)}">'
            f"<title>{html.escape(title)}</title>"
            f'<rect x="0.5" y="0.5" width="{self.w - 1}" height="{self.h - 1}" fill="{CARD}" stroke="{BORDER}" rx="6"/>'
        )
        body = head + "".join(self.parts) + "</svg>\n"
        path = os.path.join(OUT, name)
        with open(path, "w") as f:
            f.write(body)
        return path, len(body.encode())


def nice_max(v):
    v = v * 1.04
    for step in (0.5, 1, 2, 5, 10, 20, 25, 50, 100, 200, 250, 500, 1000, 2000, 5000, 10000, 20000, 50000):
        if v / step <= 6:
            return math.ceil(v / step) * step, step
    return v, v / 5


def spread(vals):
    vals = sorted(vals)
    return statistics.median(vals), vals[0], vals[-1]


def chart_a(summary):
    rows = read("A.csv")
    if not rows:
        return
    ways = [w for w in ("baseline", "wk_template", "wk_pool", "wk_cold") if any(r["way"] == w for r in rows)]
    steps = [("ready", "Ready (s)"), ("tsc", "First web tsc (s)"), ("build", "First web build (s)")]
    pw, ph, left, top = 250, 260, 50, 70
    svg = Svg(left + pw * 3 + 30, top + ph + 120)
    svg.text(20, 28, "Scenario A: time to a workspace, then its first two gates", 16, weight="bold")
    svg.text(20, 48, "Bar: median of runs 2 to 5 (runs 1 to 3 for the cold template). Whisker: lowest to highest of those runs. Ring: run 1.", 11, MUTED)
    summary.append("## Scenario A, time to a workspace that passes its first gate\n")
    summary.append("| Way | Step | Runs | Median s | Min s | Max s | Run 1 s | Non-zero exits |")
    summary.append("|---|---|---|---|---|---|---|---|")
    for i, (step, title) in enumerate(steps):
        x0 = left + i * pw
        sel = {}
        for w in ways:
            rs = [r for r in rows if r["way"] == w and r["step"] == step]
            first = [fnum(r["seconds"]) for r in rs if "first_run" in r["flag"]]
            rest = [fnum(r["seconds"]) for r in rs if "first_run" not in r["flag"]]
            if w == "wk_cold":
                rest = [fnum(r["seconds"]) for r in rs]
                first = []
            fails = sum(1 for r in rs if r["exit"] != "0")
            if rest:
                sel[w] = (spread(rest), first[0] if first else None, len(rs), fails)
        top_v = max([v[0][2] for v in sel.values()] + [v[1] for v in sel.values() if v[1]] + [1])
        vmax, tick = nice_max(top_v)
        svg.text(x0 + (pw - 40) / 2, top - 8, title, 12, anchor="middle", weight="bold")
        y_of = lambda v: top + ph - v / vmax * ph
        t = 0
        while t <= vmax + 1e-9:
            svg.line(x0, y_of(t), x0 + pw - 40, y_of(t), GRID)
            svg.text(x0 - 4, y_of(t) + 4, fmt(t, 0 if tick >= 1 else 1), 10, MUTED, "end")
            t += tick
        bw = (pw - 60) / max(len(ways), 1)
        for j, w in enumerate(ways):
            if w not in sel:
                continue
            (med, lo, hi), first, n, fails = sel[w]
            bx = x0 + 10 + j * bw
            svg.rect(bx, y_of(med), bw - 8, top + ph - y_of(med), COLORS[w], 2)
            cx = bx + (bw - 8) / 2
            svg.line(cx, y_of(lo), cx, y_of(hi), INK, 1.2)
            svg.line(cx - 4, y_of(lo), cx + 4, y_of(lo), INK, 1.2)
            svg.line(cx - 4, y_of(hi), cx + 4, y_of(hi), INK, 1.2)
            if first is not None:
                svg.add(f'<circle cx="{cx:.1f}" cy="{y_of(first):.1f}" r="3.5" fill="none" stroke="{INK}" stroke-width="1.2"/>')
            svg.text(cx, y_of(hi) - 6, fmt(med, 1 if med < 100 else 0), 10, INK, "middle")
            summary.append(f"| {w} | {step} | {n} | {fmt(med, 2)} | {fmt(lo, 2)} | {fmt(hi, 2)} | {fmt(first, 2) if first is not None else ''} | {fails} |")
    ly = top + ph + 30
    for j, w in enumerate(ways):
        lx = 20 + (j % 2) * 380
        yy = ly + (j // 2) * 18
        svg.rect(lx, yy - 10, 12, 12, COLORS[w], 2)
        svg.text(lx + 18, yy, LABELS[w], 12)
    svg.text(20, svg.h - 14, caption(rows), 10, MUTED)
    summary.append("")
    return svg.write("A-time-to-first-gate.svg", "Scenario A: time to a workspace and its first gates")


def chart_b(summary):
    rows = read("B.csv")
    if not rows:
        return
    ways = [w for w in ("baseline", "wk_sparse", "wk_full") if any(r["way"] == w for r in rows)]
    per = {}
    summary.append("## Scenario B, disk per workspace by df\n")
    summary.append("| Way | Sets kept | MiB per workspace, median | Min | Max | Drift MiB per minute, max | Failed creates | Seconds for 10, median |")
    summary.append("|---|---|---|---|---|---|---|---|")
    for w in ways:
        sets = {}
        for r in rows:
            if r["way"] != w:
                continue
            sets.setdefault(r["run"], []).append(r)
        mbs, drifts, secs, fails = [], [], [], 0
        for run, rs in sets.items():
            makes = [r for r in rs if r["step"] == "make10"]
            last = makes[-1]
            mbs.append((int(last["df_free_kb_before"]) - int(last["df_free_kb_after"])) / 1024 / 10)
            secs.append(fnum(last["seconds"]))
            fails += int(last["exit"])
            for d in rs:
                if d["step"] == "drift":
                    drifts.append(abs(int(d["df_free_kb_before"]) - int(d["df_free_kb_after"])) / 1024)
        per[w] = spread(mbs)
        med, lo, hi = per[w]
        summary.append(f"| {w} | {len(mbs)} | {fmt(med)} | {fmt(lo)} | {fmt(hi)} | {fmt(max(drifts))} | {fails} | {fmt(statistics.median(secs))} |")
    summary.append("\nThe threshold `max_mb_per_workspace` is 120 MB, which is 114.4 MiB. df -k counts 1,024-byte blocks, so these figures are MiB.\n")
    left, top, bh, gap, pw = 250, 70, 34, 16, 480
    svg = Svg(left + pw + 60, top + len(ways) * (bh + gap) + 80)
    svg.text(20, 28, "Scenario B: real disk per extra workspace (df, not du)", 16, weight="bold")
    svg.text(20, 48, "Bar: median MiB per workspace over 3 sets of 10. Whisker: lowest to highest set. Dashed line: the 120 MB (114.4 MiB) threshold.", 11, MUTED)
    vmax, tick = nice_max(max(max(v[2] for v in per.values()), 120e6 / 2**20))
    x_of = lambda v: left + v / vmax * pw
    t = 0
    base_y = top + len(ways) * (bh + gap)
    while t <= vmax + 1e-9:
        svg.line(x_of(t), top - 4, x_of(t), base_y, GRID)
        svg.text(x_of(t), base_y + 16, fmt(t, 0), 10, MUTED, "middle")
        t += tick
    for j, w in enumerate(ways):
        med, lo, hi = per[w]
        y = top + j * (bh + gap)
        svg.text(left - 8, y + bh / 2 + 4, LABELS[w], 12, INK, "end")
        svg.rect(left, y, x_of(med) - left, bh, COLORS[w], 2)
        svg.line(x_of(lo), y + bh / 2, x_of(hi), y + bh / 2, INK, 1.2)
        svg.text(x_of(max(hi, med)) + 6, y + bh / 2 + 4, f"{fmt(med, 0)} MiB", 11)
    svg.line(x_of(120e6 / 2**20), top - 4, x_of(120e6 / 2**20), base_y, INK, 1, "4 3")
    svg.text(left + pw / 2, base_y + 34, "MiB of df free space used per workspace", 11, MUTED, "middle")
    svg.text(20, svg.h - 14, caption(rows), 10, MUTED)
    return svg.write("B-disk-per-workspace.svg", "Scenario B: disk per workspace")


def chart_c(summary):
    rows = read("C.csv")
    if not rows:
        return
    ways = [w for w in ("baseline", "wk_mp2", "wk_pool_mp2", "wk_mp10") if any(r["way"] == w for r in rows)]
    summary.append("## Scenario C, a burst of 10\n")
    summary.append("| Way | Bursts | Wall to last ready, median s | Min | Max | Job time, median s | Failed jobs | Bursts over 60 s |")
    summary.append("|---|---|---|---|---|---|---|---|")
    pick = {}
    for w in ways:
        walls = {r["run"]: fnum(r["seconds"]) for r in rows if r["way"] == w and r["step"] == "wall"}
        jobs = [r for r in rows if r["way"] == w and r["step"].startswith("job")]
        med, lo, hi = spread(list(walls.values()))
        fails = sum(1 for r in jobs if r["exit"] != "0")
        summary.append(
            f"| {w} | {len(walls)} | {fmt(med, 2)} | {fmt(lo, 2)} | {fmt(hi, 2)} | {fmt(statistics.median(fnum(r['seconds']) for r in jobs), 2)} | {fails} | {sum(1 for v in walls.values() if v > 60)} |"
        )
        run = sorted(walls, key=lambda k: (walls[k], k))[len(walls) // 2]
        pick[w] = (run, walls[run], [r for r in jobs if r["run"] == run])
    summary.append("\nThe threshold is `burst_max_seconds = 60` for `burst_size = 10`.\n")
    left, top, lane, pw = 250, 70, 9, 520
    ph = 10 * lane + 18
    svg = Svg(left + pw + 40, top + len(ways) * (ph + 12) + 70)
    svg.text(20, 28, "Scenario C: ten workspaces asked for in the same second", 16, weight="bold")
    svg.text(20, 48, "Each line is one workspace, from its request to ready. The burst shown is the median one of 5 by wall time.", 11, MUTED)
    vmax, tick = nice_max(max(v[1] for v in pick.values()))
    x_of = lambda v: left + v / vmax * pw
    base_y = top + len(ways) * (ph + 12)
    t = 0
    while t <= vmax + 1e-9:
        svg.line(x_of(t), top - 4, x_of(t), base_y, GRID)
        svg.text(x_of(t), base_y + 16, fmt(t, 0), 10, MUTED, "middle")
        t += tick
    for j, w in enumerate(ways):
        run, wall, jobs = pick[w]
        y0 = top + j * (ph + 12)
        svg.text(left - 8, y0 + ph / 2, LABELS[w], 12, INK, "end")
        svg.text(left - 8, y0 + ph / 2 + 15, f"last ready at {fmt(wall)} s", 11, MUTED, "end")
        for k, r in enumerate(sorted(jobs, key=lambda r: int(r["step"][3:]))):
            off = fnum(r["flag"].split("start_offset=")[1].split(";")[0])
            sec = fnum(r["seconds"])
            y = y0 + 6 + k * lane
            color = COLORS[w] if r["exit"] == "0" else "#cf222e"
            svg.line(x_of(off), y, x_of(off + sec), y, color, 5)
    svg.text(left + pw / 2, base_y + 34, "seconds from the burst start", 11, MUTED, "middle")
    svg.text(20, svg.h - 14, caption(rows), 10, MUTED)
    return svg.write("C-burst-timeline.svg", "Scenario C: burst timeline")


def chart_d(summary):
    rows = read("D.csv")
    samples = read("D-samples.csv")
    if not rows or not samples:
        return
    total_mb = 65536
    summary.append("## Scenario D, six web builds at once\n")
    summary.append("Build seconds run from the request to the end, so a slotted build includes its wait for a place.\n")
    summary.append("| Way | Run | Wall s | Build s, median | Failed builds | Free MB at start | Lowest free MB | Largest drop MB | Swap MB at start | Peak swap MB | Lowest memory_pressure free % | Peak load1 | Flag |")
    summary.append("|---|---|---|---|---|---|---|---|---|---|---|---|---|")
    for w in ("plain", "slotted"):
        for run in sorted({r["run"] for r in rows if r["way"] == w}):
            rs = [r for r in rows if r["way"] == w and r["run"] == run]
            wall = next(r for r in rs if r["step"] == "wall")
            builds = [r for r in rs if r["step"].startswith("build")]
            ss = [s for s in samples if s["way"] == w and s["run"] == run]
            frees = [fnum(s["free_mb"]) for s in ss]
            swaps = [fnum(s["swap_mb"]) or 0 for s in ss]
            mps = [fnum(s["mem_free_pct"]) for s in ss if fnum(s["mem_free_pct"]) is not None]
            loads = [fnum(s["load1"]) for s in ss]
            bsecs = [fnum(r["seconds"]) for r in builds if fnum(r["seconds"]) is not None]
            summary.append(
                f"| {w} | {run} | {fmt(fnum(wall['seconds']))} | {fmt(statistics.median(bsecs)) if bsecs else ''} | {sum(1 for r in builds if r['exit'] != '0')} | {fmt(frees[0], 0)} | {fmt(min(frees), 0)} | {fmt(frees[0] - min(frees), 0)} | {fmt(swaps[0], 0)} | {fmt(max(swaps), 0)} | {fmt(min(mps), 0) if mps else ''} | {fmt(max(loads), 2)} | {wall['flag']} |"
            )
    summary.append("")
    left, top, pw, ph = 70, 70, 640, 280
    svg = Svg(left + pw + 40, top + ph + 110)
    svg.text(20, 28, "Scenario D: memory in use while six web builds start together", 16, weight="bold")
    svg.text(20, 48, "Used = 64 GiB minus free, inactive and speculative pages (vm_stat), sampled each second. 3 runs per way.", 11, MUTED)
    tmax = max(fnum(s["t"]) for s in samples)
    xmax, xt = nice_max(tmax)
    used = [total_mb - fnum(s["free_mb"]) for s in samples]
    ymax = total_mb
    ytick = 8192
    x_of = lambda v: left + v / xmax * pw
    y_of = lambda v: top + ph - v / ymax * ph
    t = 0
    while t <= ymax:
        svg.line(left, y_of(t), left + pw, y_of(t), GRID)
        svg.text(left - 6, y_of(t) + 4, f"{t // 1024} GiB", 10, MUTED, "end")
        t += ytick
    t = 0
    while t <= xmax + 1e-9:
        svg.text(x_of(t), top + ph + 16, fmt(t, 0), 10, MUTED, "middle")
        t += xt
    for w in ("plain", "slotted"):
        for run in sorted({s["run"] for s in samples if s["way"] == w}):
            ss = [s for s in samples if s["way"] == w and s["run"] == run]
            pts = " ".join(f"{x_of(fnum(s['t'])):.1f},{y_of(total_mb - fnum(s['free_mb'])):.1f}" for s in ss)
            svg.add(f'<polyline points="{pts}" fill="none" stroke="{COLORS[w]}" stroke-width="1.6" stroke-opacity="0.85"/>')
    svg.text(left + pw / 2, top + ph + 34, "seconds from the start of the run", 11, MUTED, "middle")
    for j, w in enumerate(("plain", "slotted")):
        lx = left + j * 330
        svg.rect(lx, top + ph + 48, 12, 12, COLORS[w], 2)
        svg.text(lx + 18, top + ph + 58, LABELS[w], 12)
    svg.text(20, svg.h - 14, caption(rows), 10, MUTED)
    return svg.write("D-memory-during-builds.svg", "Scenario D: memory during six builds")


def chart_e(summary):
    rows = read("E-verdicts.csv")
    meta = read("E.csv")
    if not rows:
        return
    groups = ["merged", "held", "dirty_tracked", "dirty_untracked", "unmerged", "held_differs"]
    names = {
        "merged": "Merged into main",
        "held": "Only untracked files main holds",
        "dirty_tracked": "Tracked edit, not committed",
        "dirty_untracked": "New untracked file",
        "unmerged": "Commit main does not hold",
        "held_differs": "Untracked file, bytes differ from main",
    }
    summary.append("## Scenario E, cleanup verdicts\n")
    summary.append("| Group | Workspaces | Expected | Verdicts seen | Removed | Kept work intact | Matches |")
    summary.append("|---|---|---|---|---|---|---|")
    table = []
    for g in groups:
        rs = [r for r in rows if r["group"] == g]
        if not rs:
            continue
        seen = sorted({r["verdict"] for r in rs})
        removed = sum(1 for r in rs if r["dir_gone"] == "yes")
        intact = [r for r in rs if r["kept_work_intact"] != "n/a"]
        ok = sum(1 for r in rs if r["match"] == "yes")
        it = f"{sum(1 for r in intact if r['kept_work_intact'] == 'yes')} of {len(intact)}" if intact else "n/a"
        summary.append(f"| {g} | {len(rs)} | {rs[0]['expected']} | {', '.join(seen)} | {removed} | {it} | {ok} of {len(rs)} |")
        table.append((names[g], rs[0]["expected"], ", ".join(seen), f"{removed} of {len(rs)}", it, f"{ok} of {len(rs)}", ok == len(rs)))
    gc_times = {}
    for r in meta:
        gc_times.setdefault(r["step"], []).append(fnum(r["seconds"]))
    for step, vals in sorted(gc_times.items()):
        med, lo, hi = spread(vals)
        summary.append(f"\n`wk gc` {'--apply' if step == 'apply' else '(dry run)'}: median {fmt(med, 2)} s over {len(vals)} runs, lowest {fmt(lo, 2)}, highest {fmt(hi, 2)}.")
    summary.append("")
    cols = [("Workspace state", 20), ("Expected", 300), ("Verdict seen", 390), ("Removed", 500), ("Work kept", 580), ("Right", 670)]
    top, rh = 80, 30
    svg = Svg(740, top + rh * (len(table) + 1) + 50)
    svg.text(20, 28, "Scenario E: what wk gc --apply removed", 16, weight="bold")
    svg.text(20, 48, f"{len(rows)} workspaces over {len({r['rep'] for r in rows})} repetitions, gc.quiet_minutes = 1 in the bench config.", 11, MUTED)
    for name, x in cols:
        svg.text(x, top, name, 12, MUTED, weight="bold")
    svg.line(16, top + 8, 724, top + 8, BORDER)
    for i, (name, exp, seen, rem, it, ok, good) in enumerate(table):
        y = top + rh * (i + 1)
        if i % 2 == 0:
            svg.rect(16, y - 19, 708, rh, "#f6f8fa")
        for (h, x), v in zip(cols, (name, exp, seen, rem, it, ok)):
            svg.text(x, y, v, 12, INK if (h != "Right" or good) else "#cf222e", weight="bold" if h == "Right" else "normal")
    svg.text(20, svg.h - 14, caption(meta), 10, MUTED)
    return svg.write("E-gc-verdicts.svg", "Scenario E: gc verdicts")


def summary_f(summary):
    rows = read("F.csv")
    if not rows:
        return
    stops = sum(int(r["gate_stops"]) for r in rows)
    pairs = sum(int(r["pairs_within_60s_other_folder"]) for r in rows)
    near = sum(int(r["stops_with_such_a_neighbor"]) for r in rows)
    summary.append("## Scenario F, Stop gates that end close together\n")
    summary.append(f"{stops} Stop-hook gates with a `ran=` folder in the copied log, {rows[0]['day_utc']} to {rows[-1]['day_utc']} (UTC days). {pairs} pairs from different folders ended within 60 s of each other; {near} gates were in such a pair.\n")
    summary.append("| Day (UTC) | Gates | Folders | Pairs within 60 s | Gates in a pair |")
    summary.append("|---|---|---|---|---|")
    for r in rows:
        summary.append(f"| {r['day_utc']} | {r['gate_stops']} | {r['folders']} | {r['pairs_within_60s_other_folder']} | {r['stops_with_such_a_neighbor']} |")
    summary.append("")


def summary_conditions(summary):
    summary.append("## Run conditions\n")
    summary.append("| Scenario | Rows | Start stamps | load1 at start, lowest | highest | Memory free %, lowest | Rows marked load_high | Rows marked stopped |")
    summary.append("|---|---|---|---|---|---|---|---|")
    for sc in "ABCDE":
        rows = read(f"{sc}.csv")
        if not rows:
            continue
        starts = {r["started_at"] for r in rows}
        loads = [fnum(r["load1"]) for r in rows if fnum(r["load1"]) is not None]
        mems = [fnum(r["mem_free_pct"]) for r in rows if fnum(r["mem_free_pct"]) is not None]
        hi = sum(1 for r in rows if "load_high" in r["flag"])
        st = sum(1 for r in rows if "stopped" in r["flag"])
        summary.append(f"| {sc} | {len(rows)} | {len(starts)} | {fmt(min(loads), 2)} | {fmt(max(loads), 2)} | {fmt(min(mems), 0)} | {hi} | {st} |")
    summary.append("")


def main():
    summary = ["# Benchmark summary", "", "Generated by `docs/bench/chart.py` from `docs/bench/data/*.csv`. Do not edit by hand.", ""]
    made = []
    for fn in (chart_a, chart_b, chart_c, chart_d, chart_e):
        r = fn(summary)
        if r:
            made.append(r)
    summary_f(summary)
    summary_conditions(summary)
    with open(os.path.join(OUT, "SUMMARY.md"), "w") as f:
        f.write("\n".join(summary) + "\n")
    for path, size in made:
        print(f"{os.path.basename(path)} {size} bytes", file=sys.stderr)
        if size > 50000:
            print(f"chart.py: {path} is over 50 KB", file=sys.stderr)
            sys.exit(1)


if __name__ == "__main__":
    main()
