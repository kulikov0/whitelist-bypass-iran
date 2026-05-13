#!/bin/sh
# Repro the dc_research.md table: 4 configurations, with live progress.
set -u
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
BIN="$ROOT/headless/bale-dc-test/headless-bale-dc-test"
COOKIES="${1:-$ROOT/bale-cookies.json}"

[ -x "$BIN" ] || { echo "FAIL: $BIN not built"; exit 2; }
[ -r "$COOKIES" ] || { echo "FAIL: $COOKIES not readable"; exit 2; }

run_one() {
    label="$1"; mode="$2"; mb="$3"; pace="$4"
    C_LOG=$(mktemp -t dc-c.XXXXXX.log)
    J_LOG=$(mktemp -t dc-j.XXXXXX.log)
    HOLD=60
    echo
    echo "============================================================"
    echo "$label  (mode=$mode bench-mb=$mb pace-kbps=$pace hold=$HOLD)"
    echo "  creator log: $C_LOG"
    echo "  joiner log:  $J_LOG"
    echo "============================================================"

    "$BIN" --role creator --cookies "$COOKIES" \
        --bench-mb "$mb" --mode "$mode" --pace-kbps "$pace" \
        --wait-peer-sec 60 --hold-sec "$HOLD" > "$C_LOG" 2>&1 &
    C_PID=$!
    echo "[$(date +%H:%M:%S)] creator started (pid=$C_PID), waiting for join_link..."

    waited=0; JOIN_LINK=""
    while [ "$waited" -lt 60 ]; do
        JOIN_LINK=$(grep -m1 "join_link:" "$C_LOG" 2>/dev/null | awk '{print $2}')
        [ -n "$JOIN_LINK" ] && break
        if ! kill -0 "$C_PID" 2>/dev/null; then
            echo "[$(date +%H:%M:%S)] FAIL: creator died before join_link"
            tail -20 "$C_LOG"
            return
        fi
        sleep 1; waited=$((waited + 1))
    done
    [ -n "$JOIN_LINK" ] || { echo "[$(date +%H:%M:%S)] FAIL: join_link timeout"; kill $C_PID 2>/dev/null; return; }
    echo "[$(date +%H:%M:%S)] join_link=$JOIN_LINK"

    "$BIN" --role joiner --join-link "$JOIN_LINK" \
        --wait-peer-sec 60 --hold-sec "$HOLD" > "$J_LOG" 2>&1 &
    J_PID=$!
    echo "[$(date +%H:%M:%S)] joiner started (pid=$J_PID)"

    waited=0
    deadline=$((HOLD + 90))
    while [ "$waited" -lt "$deadline" ]; do
        if grep -q "^RESULT:" "$J_LOG" 2>/dev/null; then
            echo "[$(date +%H:%M:%S)] joiner emitted RESULT after ${waited}s"
            break
        fi
        if ! kill -0 "$J_PID" 2>/dev/null; then
            echo "[$(date +%H:%M:%S)] joiner exited at ${waited}s"
            break
        fi
        if [ $((waited % 5)) -eq 0 ] && [ "$waited" -gt 0 ]; then
            last_j=$(tail -1 "$J_LOG" 2>/dev/null | cut -c1-120)
            echo "[$(date +%H:%M:%S)] t=${waited}s waiting... last joiner: $last_j"
        fi
        sleep 1; waited=$((waited + 1))
    done

    kill $C_PID 2>/dev/null
    kill $J_PID 2>/dev/null
    wait $C_PID 2>/dev/null
    wait $J_PID 2>/dev/null

    echo "[$(date +%H:%M:%S)] result lines:"
    grep -E "^RESULT|^  RESULT" "$J_LOG" | sed 's/^/    /'
}

run_one "row1 reliable 10MB unpaced" reliable 10  0
run_one "row2 reliable 50MB unpaced" reliable 50  0
run_one "row3 lossy    10MB pace=3M" lossy    10  3000
run_one "row4 lossy    10MB unpaced" lossy    10  0
