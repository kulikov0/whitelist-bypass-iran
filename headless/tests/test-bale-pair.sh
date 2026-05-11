#!/bin/sh
# Two headless-bale-creator instances against the same call: first creates,
# second joins by re-running with the same call_id (when the binary supports
# --call-id). Until then this test creates two independent calls and verifies
# both reach publisher connected; the SOCKS5 e2e probe is skipped because the
# joiner binary does not exist yet.
#
# Usage:
#   ./test-bale-pair.sh [path/to/bale-cookies.json]

set -u

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
CREATOR="$ROOT/headless/bale/headless-bale-creator"
COOKIES="${1:-$ROOT/bale-cookies.json}"
SETTLE_TIMEOUT=60

[ -x "$CREATOR" ] || { echo "FAIL: $CREATOR not built" >&2; exit 2; }
[ -r "$COOKIES" ] || { echo "FAIL: cookies file not readable: $COOKIES" >&2; exit 2; }

LOG_A=$(mktemp -t bale-a.XXXXXX.log)
LOG_B=$(mktemp -t bale-b.XXXXXX.log)
PID_A=""
PID_B=""
cleanup() {
    [ -n "$PID_B" ] && kill "$PID_B" 2>/dev/null
    [ -n "$PID_A" ] && kill "$PID_A" 2>/dev/null
    wait 2>/dev/null
}
trap cleanup EXIT INT TERM

run_one() {
    log=$1
    label=$2
    "$CREATOR" --cookies "$COOKIES" --resources default > "$log" 2>&1 &
    pid=$!
    waited=0
    while [ "$waited" -lt "$SETTLE_TIMEOUT" ]; do
        if grep -q "\[lk\] pub PC state: connected" "$log"; then
            link=$(grep -m1 "join_link:" "$log" | awk '{print $2}')
            echo "${label}_join_link=$link"
            echo "$pid"
            return 0
        fi
        if ! kill -0 "$pid" 2>/dev/null; then
            echo "FAIL: $label died" >&2
            tail -30 "$log" >&2
            return 1
        fi
        sleep 1
        waited=$((waited + 1))
    done
    echo "FAIL: $label did not reach pub PC connected within ${SETTLE_TIMEOUT}s" >&2
    tail -30 "$log" >&2
    return 1
}

echo "=== peer A ==="
PID_A=$(run_one "$LOG_A" A) || exit 1

echo "=== peer B ==="
PID_B=$(run_one "$LOG_B" B) || exit 1

echo "PASS: two independent bale creators reached pub PC connected"
