#!/bin/sh
# The row 19 benchmark. See docs/bench/METHOD.md for what each scenario measures and why.
#
#   sh docs/bench/run.sh --scenario A|B|C|D|E|F [--fresh]
#
# BENCH_DATA where the CSVs go (default: docs/bench/data)
# RUNS       run every scenario this many times instead (a smoke test)
# BENCH_DIR  scratch directory for the clone and WK_HOME (made on first use; --fresh remakes it)
# SRC        the repository to clone (default: the ezhomesteading main checkout)
# WK         the installed wk binary (default: /opt/homebrew/opt/wk/bin/wk)
#
# The gate commands below run inside this script on purpose: the owner allowed it on
# 2026-11-21 for the benchmark only, so the live PreToolUse hook does not wrap them in the
# live slots. Every wk call here uses the scratch WK_HOME and its scratch slots.
set -u

SCENARIO=""
FRESH=0
while [ $# -gt 0 ]; do
	case "$1" in
	--scenario) SCENARIO="$2"; shift 2 ;;
	--fresh) FRESH=1; shift ;;
	--build-job)
		# One scenario D build: <way> <web dir> <result file>. Writes "start end exit".
		js=$(perl -MTime::HiRes=time -e 'printf "%.3f", time')
		cd "$3" || exit 1
		if [ "$2" = slotted ]; then
			"${WK:-/opt/homebrew/opt/wk/bin/wk}" run --slot heavy --trim web/.next/cache -- bun run build >"$4.log" 2>&1
		else
			bun run build >"$4.log" 2>&1
		fi
		e=$?
		echo "$js $(perl -MTime::HiRes=time -e 'printf "%.3f", time') $e" > "$4"
		exit 0
		;;
	*) echo "usage: sh run.sh --scenario A|B|C|D|E|F [--fresh]" >&2; exit 2 ;;
	esac
done
[ -n "$SCENARIO" ] || { echo "usage: sh run.sh --scenario A|B|C|D|E|F [--fresh]" >&2; exit 2; }

HERE=$(cd "$(dirname "$0")" && pwd)
DATA=${BENCH_DATA:-$HERE/data}
BENCH_DIR=${BENCH_DIR:-/private/tmp/wk-bench}
SRC=${SRC:-/Users/zachshort/Developer/dan-short/ezhomesteading}
WK=${WK:-/opt/homebrew/opt/wk/bin/wk}
B="$BENCH_DIR/ezh-bench"
H="$BENCH_DIR/wkhome"
LOGS="$BENCH_DIR/logs"
export WK_HOME="$H"
export NEXT_TELEMETRY_DISABLED=1
unset WK_SLOT_HELD
UNTRACKED="web/.env backend/.env native/.env web/next-env.d.ts native/expo-env.d.ts .claude/settings.local.json"
BASELINE_COPIES="web/.env backend/.env native/.env web/next-env.d.ts native/expo-env.d.ts"

mkdir -p "$DATA"

# runs <n>: 1..n, or 1..$RUNS when RUNS is set (a smoke test).
runs() { seq 1 "${RUNS:-$1}"; }

now() { perl -MTime::HiRes=time -e 'printf "%.3f\n", time'; }
iso() { date +%Y-%m-%dT%H:%M:%S%z; }
dfree() { df -k / | awk 'NR==2 {print $4}'; }
load1() { sysctl -n vm.loadavg | awk '{print $2}'; }
load5() { sysctl -n vm.loadavg | awk '{print $3}'; }
memfree() { memory_pressure 2>/dev/null | awk '/free percentage/ {sub("%","",$NF); print $NF}'; }
sub() { echo "$1 - $2" | bc -l | awk '{printf "%.3f", $1}'; }

wkv() { "$WK" --version | awk '{print $2}'; }
wkc() { GOTOOLCHAIN=local go version -m "$WK" 2>/dev/null | awk -F= '/vcs.revision/ {print substr($2,1,12)}'; }

HEADER="scenario,way,run,step,seconds,df_free_kb_before,df_free_kb_after,exit,started_at,load1,load5,mem_free_pct,wk_version,wk_commit,ezh_commit,flag"

# row way run step seconds dfb dfa exit started flag
row() {
	f="$DATA/$SCENARIO.csv"
	[ -f "$f" ] || echo "$HEADER" > "$f"
	echo "$SCENARIO,$1,$2,$3,$4,$5,$6,$7,$8,$RUN_L1,$RUN_L5,$RUN_MEM,$WKV,$WKC,$EZHC,$9" >> "$f"
}

setup() {
	if [ "$FRESH" = 1 ] && [ -d "$BENCH_DIR" ]; then
		for w in $(git -C "$B" worktree list --porcelain 2>/dev/null | awk '/^worktree / {print $2}' | grep -v "^$B\$"); do
			git -C "$B" worktree remove --force "$w" >/dev/null 2>&1
		done
		rm -rf "$B" "$H" "$LOGS"
	fi
	mkdir -p "$BENCH_DIR" "$LOGS"
	if [ ! -d "$B/.git" ]; then
		git clone --no-hardlinks -q "$SRC" "$B" || exit 1
		git -C "$B" remote remove origin
		mkdir -p "$B/.claude"
		for f in $UNTRACKED; do cp "$SRC/$f" "$B/$f" || exit 1; done
	fi
	if [ ! -f "$H/ezh-bench.base.toml" ]; then
		mkdir -p "$H/slots"
		sed '1s|.*|# Bench copy of ~/.wk/ezhomesteading.toml. Values unchanged.|' "$HOME/.wk/ezhomesteading.toml" > "$H/ezh-bench.base.toml"
		cp "$HOME/.wk/ezhomesteading.slots.toml" "$H/ezh-bench.slots.toml"
		sed "s|^dir = .*|dir = \"$H/slots\"|" "$HOME/.wk/machine.toml" > "$H/machine.toml"
		cp "$H/ezh-bench.base.toml" "$H/ezh-bench.toml"
	fi
	"$WK" --repo "$B" config show 2>&1 | head -1 | grep -q '(found)' || { echo "run.sh: wk config not found under $H" >&2; exit 1; }
	WKV=$(wkv); WKC=$(wkc); EZHC=$(git -C "$B" rev-parse --short=12 HEAD)
	if [ -f "$H/version" ]; then
		[ "$(cat "$H/version")" = "$WKV $WKC" ] || { echo "run.sh: wk changed from $(cat "$H/version") to $WKV $WKC; refusing" >&2; exit 1; }
	else
		echo "$WKV $WKC" > "$H/version"
	fi
	RUN_L1=; RUN_L5=; RUN_MEM=
}

# cfg <pool.sparse_size> <new.max_parallel> <gc.quiet_minutes>
cfg() {
	sed -e "s/^sparse_size = .*/sparse_size = $1/" -e "s/^quiet_minutes = .*/quiet_minutes = $3/" "$H/ezh-bench.base.toml" > "$H/ezh-bench.toml"
	printf '\n[new]\nmax_parallel = %s\n' "$2" >> "$H/ezh-bench.toml"
}

wait_fill() {
	i=0
	while pgrep -f -- "--repo $B pool fill" >/dev/null 2>&1 && [ $i -lt 600 ]; do sleep 1; i=$((i + 1)); done
}

worktrees() { git -C "$B" worktree list --porcelain | awk '/^worktree / {print $2}' | grep -v "^$B\$"; }

drain_pool() {
	wait_fill
	for w in $(worktrees | grep '/wk-pool-'); do git -C "$B" worktree remove --force "$w" >/dev/null 2>&1; done
	git -C "$B" worktree prune
}

clean() {
	wait_fill
	for w in $(worktrees | grep -v '/wk-template$' | grep -v '/wk-pool-'); do
		git -C "$B" worktree remove --force "$w" >/dev/null 2>&1 || rm -rf "$w"
	done
	git -C "$B" worktree prune
	for br in $(git -C "$B" for-each-ref --format='%(refname:short)' refs/heads/wk/ refs/heads/bench/); do
		git -C "$B" branch -D "$br" >/dev/null 2>&1
	done
	wait_disk
}

wait_disk() {
	i=0
	while [ $i -lt 12 ]; do
		a=$(dfree); sleep 10; b=$(dfree)
		d=$((a - b)); [ $d -lt 0 ] && d=$((0 - d))
		[ $d -lt 51200 ] && return 0
		i=$((i + 1))
	done
}

# Record the start-of-run state; wait up to 5 minutes while the 1-minute load is above 4.
start_run() {
	RUN_FLAG=""
	i=0
	while [ "$(echo "$(load1) > 4" | bc -l)" = 1 ] && [ $i -lt 20 ]; do sleep 15; i=$((i + 1)); done
	[ "$(echo "$(load1) > 4" | bc -l)" = 1 ] && RUN_FLAG="load_high"
	RUN_L1=$(load1); RUN_L5=$(load5); RUN_MEM=$(memfree)
	RUN_AT=$(iso)
}

NEXTN=9001
next_n() { NEXTN=$((NEXTN + 1)); N=$NEXTN; }

# make_ws <way> <n>: prints the path; sets MK_EXIT
make_ws() {
	case "$1" in
	baseline)
		W="$B/.claude/worktrees/bench-$2"
		(
			git -C "$B" worktree add -q -b "bench/$2" "$W" main &&
				(cd "$W" && bun install --frozen-lockfile) &&
				for f in $BASELINE_COPIES; do cp "$B/$f" "$W/$f" || exit 1; done
		) >"$LOGS/make-$2.log" 2>&1
		MK_EXIT=$?
		;;
	wk_full)
		W=$("$WK" --repo "$B" new "$2" --full 2>"$LOGS/make-$2.log"); MK_EXIT=$?
		;;
	*)
		W=$("$WK" --repo "$B" new "$2" 2>"$LOGS/make-$2.log"); MK_EXIT=$?
		;;
	esac
}

prepare_way() {
	case "$1" in
	baseline | wk_template | wk_sparse | wk_full) drain_pool; cfg 0 2 60 ;;
	wk_mp10) drain_pool; cfg 0 10 60 ;;
	wk_pool | wk_pool_mp2)
		cfg 4 2 60; wait_fill
		"$WK" --repo "$B" pool fill >"$LOGS/pool-fill.log" 2>&1
		;;
	wk_cold)
		drain_pool; cfg 0 2 60
		git -C "$B" worktree remove --force "$B/.claude/worktrees/wk-template" >/dev/null 2>&1
		git -C "$B" worktree prune
		;;
	esac
}

ensure_template() {
	if ! worktrees | grep -q '/wk-template$'; then
		"$WK" --repo "$B" template refresh >"$LOGS/template.log" 2>&1
	fi
}

# timed <way> <run> <step> <dir> <cmd...>
timed() {
	tw=$1; tr=$2; ts=$3; td=$4; shift 4
	dfb=$(dfree); s=$(now); at=$(iso)
	(cd "$td" && "$@") >"$LOGS/$SCENARIO-$tw-$tr-$ts.log" 2>&1
	e=$?
	t=$(now); dfa=$(dfree)
	row "$tw" "$tr" "$ts" "$(sub "$t" "$s")" "$dfb" "$dfa" "$e" "$at" "$RUN_FLAG"
}

scenario_A() {
	ensure_template
	for r in $(runs 5); do
		for way in baseline wk_template wk_pool; do
			prepare_way "$way"; start_run; next_n
			dfb=$(dfree); s=$(now)
			make_ws "$way" "$N"
			t=$(now); dfa=$(dfree)
			fl=$RUN_FLAG; [ "$r" = 1 ] && fl="${fl:+$fl;}first_run"
			row "$way" "$r" ready "$(sub "$t" "$s")" "$dfb" "$dfa" "$MK_EXIT" "$RUN_AT" "$fl"
			[ "$way" = wk_pool ] && wait_fill
			RUN_FLAG=$fl
			timed "$way" "$r" tsc "$W/web" bunx tsc --noEmit
			timed "$way" "$r" build "$W/web" bun run build
			clean
		done
	done
	for r in $(runs 3); do
		prepare_way wk_cold; start_run; next_n
		dfb=$(dfree); s=$(now)
		make_ws wk_cold "$N"
		t=$(now); dfa=$(dfree)
		row wk_cold "$r" ready "$(sub "$t" "$s")" "$dfb" "$dfa" "$MK_EXIT" "$RUN_AT" "$RUN_FLAG"
		timed wk_cold "$r" tsc "$W/web" bunx tsc --noEmit
		timed wk_cold "$r" build "$W/web" bun run build
		clean
	done
}

scenario_B() {
	ensure_template
	for r in $(runs 3); do
		for way in baseline wk_sparse wk_full; do
			attempt=1
			while [ $attempt -le 3 ]; do
				prepare_way "$way"; clean; start_run
				d0=$(dfree); sleep 60; d1=$(dfree)
				s=$(now); fails=0
				for k in 1 2 3 4 5 6 7 8 9 10; do
					next_n; make_ws "$way" "$N"; [ "$MK_EXIT" = 0 ] || fails=$((fails + 1))
				done
				t=$(now); sleep 10; d2=$(dfree)
				drift=$((d0 - d1)); [ $drift -lt 0 ] && drift=$((0 - drift))
				used=$((d1 - d2))
				fl=$RUN_FLAG
				if [ $drift -gt $used ] && [ $attempt -lt 3 ]; then fl="${fl:+$fl;}drift_rerun"; fi
				row "$way" "$r" drift 60 "$d0" "$d1" 0 "$RUN_AT" "attempt=$attempt${fl:+;$fl}"
				row "$way" "$r" make10 "$(sub "$t" "$s")" "$d1" "$d2" "$fails" "$RUN_AT" "attempt=$attempt${fl:+;$fl}"
				clean
				case "$fl" in *drift_rerun*) attempt=$((attempt + 1)) ;; *) break ;; esac
			done
		done
	done
}

scenario_C() {
	ensure_template
	for r in $(runs 5); do
		for way in baseline wk_mp2 wk_pool_mp2 wk_mp10; do
			case "$way" in wk_mp2) prepare_way wk_template ;; *) prepare_way "$way" ;; esac
			start_run
			jobs_dir="$LOGS/C-$way-$r"; rm -rf "$jobs_dir"; mkdir -p "$jobs_dir"
			base=$NEXTN; NEXTN=$((NEXTN + 10))
			s=$(now); dfb=$(dfree)
			for k in 1 2 3 4 5 6 7 8 9 10; do
				n=$((base + k))
				(
					js=$(now)
					make_ws "$way" "$n"
					echo "$js $(now) $MK_EXIT" > "$jobs_dir/$k"
				) &
			done
			wait
			t=$(now); dfa=$(dfree)
			for k in 1 2 3 4 5 6 7 8 9 10; do
				read -r js je je_exit < "$jobs_dir/$k"
				row "$way" "$r" "job$k" "$(sub "$je" "$js")" "" "" "$je_exit" "$RUN_AT" "start_offset=$(sub "$js" "$s")"
			done
			row "$way" "$r" wall "$(sub "$t" "$s")" "$dfb" "$dfa" 0 "$RUN_AT" "$RUN_FLAG"
			clean
		done
	done
}

sampler() {
	out="$DATA/D-samples.csv"
	[ -f "$out" ] || echo "way,run,t,free_mb,swap_mb,mem_free_pct,load1" > "$out"
	t0=$1; way=$2; r=$3; stopf=$4; donef=$5
	ps_=$(sysctl -n hw.pagesize)
	while [ ! -f "$donef" ]; do
		fm=$(vm_stat | awk -v p="$ps_" '/Pages free/ {f=$3} /Pages inactive/ {i=$3} /Pages speculative/ {s=$3} END {gsub("\\.","",f); gsub("\\.","",i); gsub("\\.","",s); printf "%d", (f+i+s)*p/1048576}')
		sw=$(sysctl -n vm.swapusage | awk '{for (i=1;i<=NF;i++) if ($i=="used") {v=$(i+2); sub("M","",v); print v}}')
		mp=$(memfree)
		echo "$way,$r,$(sub "$(now)" "$t0"),$fm,$sw,$mp,$(load1)" >> "$out"
		if [ "$(echo "$sw > 4096" | bc -l)" = 1 ] || [ "${mp:-100}" -lt 15 ]; then touch "$stopf"; fi
		sleep 1
	done
}

scenario_D() {
	ensure_template; prepare_way wk_template; clean
	ws=""
	for k in 1 2 3 4 5 6; do next_n; make_ws wk_template "$N"; ws="$ws $W"; done
	for r in $(runs 3); do
		for way in plain slotted; do
			for w in $ws; do rm -rf "$w/web/.next"; done
			wait_disk; start_run
			jd="$LOGS/D-$way-$r"; rm -rf "$jd"; mkdir -p "$jd"
			s=$(now)
			sampler "$s" "$way" "$r" "$jd/stop" "$jd/done" &
			spid=$!
			k=0
			for w in $ws; do
				k=$((k + 1))
				perl -e 'setpgrp(0,0); exec @ARGV' sh "$HERE/run.sh" --build-job "$way" "$w/web" "$jd/$k" &
				echo $! >> "$jd/pids"
			done
			stopped=""
			while [ "$(ls "$jd" | grep -cE '^[0-9]+$')" -lt 6 ]; do
				if [ -f "$jd/stop" ]; then
					stopped="stopped"
					for p in $(cat "$jd/pids"); do kill -TERM -- "-$p" 2>/dev/null; done
					sleep 5
					for p in $(cat "$jd/pids"); do kill -KILL -- "-$p" 2>/dev/null; done
					break
				fi
				sleep 1
			done
			wait_jobs=0
			while [ "$(ls "$jd" | grep -cE '^[0-9]+$')" -lt 6 ] && [ $wait_jobs -lt 30 ]; do sleep 1; wait_jobs=$((wait_jobs + 1)); done
			t=$(now); touch "$jd/done"; wait "$spid" 2>/dev/null
			for k in 1 2 3 4 5 6; do
				if [ -f "$jd/$k" ]; then
					read -r js je je_exit < "$jd/$k"
					row "$way" "$r" "build$k" "$(sub "$je" "$js")" "" "" "$je_exit" "$RUN_AT" "start_offset=$(sub "$js" "$s")${stopped:+;$stopped}"
				else
					row "$way" "$r" "build$k" "" "" "" "killed" "$RUN_AT" "$stopped"
				fi
			done
			fl=$RUN_FLAG; [ -n "$stopped" ] && fl="${fl:+$fl;}stopped"
			row "$way" "$r" wall "$(sub "$t" "$s")" "" "" 0 "$RUN_AT" "$fl"
		done
	done
	clean
}

scenario_E() {
	ensure_template
	out="$DATA/E-verdicts.csv"
	[ -f "$out" ] || echo "rep,row,group,expected,verdict,held,reasons,dir_gone,branch_gone,kept_work_intact,match" > "$out"
	for r in $(runs 3); do
		prepare_way wk_template; cfg 0 2 1; clean; start_run
		groups=""
		for g in merged merged merged merged dirty_tracked dirty_tracked dirty_untracked dirty_untracked unmerged unmerged unmerged unmerged held held held_differs held_differs; do
			next_n; make_ws wk_template "$N"
			[ "$MK_EXIT" = 0 ] || echo "run.sh: wk new $N failed" >&2
			groups="$groups $N:$g"
		done
		hk=0
		for ng in $groups; do
			n=${ng%%:*}; g=${ng#*:}; w="$B/.claude/worktrees/wk-$n"
			case "$g" in
			merged)
				echo "bench $n" > "$w/bench-merged-$n.txt"
				git -C "$w" add "bench-merged-$n.txt" && git -C "$w" commit -q -m "bench $n"
				git -C "$B" merge -q --no-edit "wk/$n"
				;;
			dirty_tracked) echo "bench $n" >> "$w/README.md" ;;
			dirty_untracked) echo "bench $n" > "$w/bench-untracked-$n.txt" ;;
			unmerged)
				echo "bench $n" > "$w/bench-unmerged-$n.txt"
				git -C "$w" add "bench-unmerged-$n.txt" && git -C "$w" commit -q -m "bench $n"
				;;
			esac
		done
		for ng in $groups; do
			n=${ng%%:*}; g=${ng#*:}
			case "$g" in held | held_differs)
				hk=$((hk + 1)); echo "held $r $hk" > "$B/bench-held-$r-$hk.txt"
				git -C "$B" add "bench-held-$r-$hk.txt"; git -C "$B" commit -q -m "bench held $r $hk"
				;;
			esac
		done
		hk=0
		for ng in $groups; do
			n=${ng%%:*}; g=${ng#*:}; w="$B/.claude/worktrees/wk-$n"
			case "$g" in
			held) hk=$((hk + 1)); echo "held $r $hk" > "$w/bench-held-$r-$hk.txt" ;;
			held_differs) hk=$((hk + 1)); echo "other bytes $r $hk" > "$w/bench-held-$r-$hk.txt" ;;
			esac
		done
		sleep 90
		timed gc "$r" dry "$B" "$WK" --repo "$B" gc
		"$WK" --repo "$B" gc --json > "$LOGS/E-$r.json" 2>"$LOGS/E-$r.json.err"
		timed gc "$r" apply "$B" "$WK" --repo "$B" gc --apply
		"$WK" --repo "$B" gc > "$LOGS/E-$r-after.txt" 2>&1
		for ng in $groups; do
			n=${ng%%:*}; g=${ng#*:}; w="$B/.claude/worktrees/wk-$n"
			case "$g" in merged | held) exp=removable ;; *) exp=kept ;; esac
			dg=no; [ -d "$w" ] || dg=yes
			bg=no; git -C "$B" rev-parse -q --verify "refs/heads/wk/$n" >/dev/null || bg=yes
			intact=n/a
			case "$g" in
			dirty_tracked) intact=no; grep -q "bench $n" "$w/README.md" 2>/dev/null && intact=yes ;;
			dirty_untracked) intact=no; [ -f "$w/bench-untracked-$n.txt" ] && intact=yes ;;
			unmerged) intact=no; git -C "$B" cat-file -e "wk/$n:bench-unmerged-$n.txt" 2>/dev/null && intact=yes ;;
			held_differs) intact=no; grep -q "other bytes" "$w"/bench-held-*.txt 2>/dev/null && intact=yes ;;
			esac
			python3 -I - "$LOGS/E-$r.json" "$w" "$r" "$n" "$g" "$exp" "$dg" "$bg" "$intact" >> "$out" <<'PY'
import json, sys, csv
path, w, r, n, g, exp, dg, bg, intact = sys.argv[1:]
doc = json.load(open(path))
item = next((i for i in doc["worktrees"] if i["path"].rstrip("/").endswith(w.split("/.claude/")[1])), None)
verdict = item["verdict"] if item else "missing"
held = item.get("held", 0) if item else 0
reasons = "; ".join(item.get("reasons") or []) if item else ""
ok = verdict.startswith(exp) and ((exp == "removable") == (dg == "yes")) and intact in ("yes", "n/a")
csv.writer(sys.stdout, lineterminator="\n").writerow([r, n, g, exp, verdict, held, reasons, dg, bg, intact, "yes" if ok else "no"])
PY
		done
		clean
	done
}

scenario_F() {
	mkdir -p "$BENCH_DIR/logs-copy"
	cp "$HOME/.config/personal-config/completion-gate.log" "$BENCH_DIR/logs-copy/completion-gate.log"
	cp "$HOME/.wk/slots/runs.log" "$BENCH_DIR/logs-copy/runs.log"
	python3 -I - "$BENCH_DIR/logs-copy/completion-gate.log" "$DATA/F.csv" <<'PY'
import sys, re, csv, datetime, collections
src, out = sys.argv[1:]
stops = []
for line in open(src, errors="replace"):
    m = re.match(r"(\S+Z) .*\bresult=(\S+) ran=(\S+)", line)
    if not m:
        continue
    t = datetime.datetime.strptime(m.group(1), "%Y-%m-%dT%H:%M:%SZ")
    stops.append((t, m.group(3), m.group(2)))
stops.sort()
by_day = collections.OrderedDict()
for i, (t, folder, res) in enumerate(stops):
    d = by_day.setdefault(t.date().isoformat(), {"stops": 0, "folders": set(), "pairs": 0, "with_neighbor": set()})
    d["stops"] += 1
    d["folders"].add(folder)
    for j in range(i + 1, len(stops)):
        t2, f2, _ = stops[j]
        if (t2 - t).total_seconds() > 60:
            break
        if f2 != folder:
            d["pairs"] += 1
            d["with_neighbor"].update({i, j})
with open(out, "w") as f:
    w = csv.writer(f, lineterminator="\n")
    w.writerow(["scenario", "day_utc", "gate_stops", "folders", "pairs_within_60s_other_folder", "stops_with_such_a_neighbor"])
    for day, d in by_day.items():
        w.writerow(["F", day, d["stops"], len(d["folders"]), d["pairs"], len(d["with_neighbor"])])
PY
}

setup
"scenario_$SCENARIO"
