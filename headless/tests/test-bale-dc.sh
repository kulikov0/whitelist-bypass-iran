#!/bin/sh
# Headless DC research: spawn creator role to make a Bale call, joiner role
# to anon-join the same call. Both open _reliable and _lossy DataChannels.
# Creator sends a benchmark through each; joiner reports throughput + loss.
#
# This tells us, end-to-end and without a browser, whether the Bale SFU
# actually forwards DataChannels to other participants and at what rate.
#
# Usage:
#   ./test-bale-dc.sh [path/to/bale-cookies.json] [bench_mb]

set -u

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
BIN="$ROOT/headless/bale-dc-test/headless-bale-dc-test"
COOKIES="${1:-$ROOT/bale-cookies.json}"
BENCH_MB="${2:-10}"
WAIT_LINK=60
WAIT_PEER=60
HOLD=30

[ -x "$BIN" ] || { echo "FAIL: $BIN not built" >&2; exit 2; }
[ -r "$COOKIES" ] || { echo "FAIL: cookies not readable: $COOKIES" >&2; exit 2; }

C_LOG=$(mktemp -t bale-dc-c.XXXXXX.log)
J_LOG=$(mktemp -t bale-dc-j.XXXXXX.log)
C_PID=""
J_PID=""
cleanup() {
    [ -n "$J_PID" ] && kill "$J_PID" 2>/dev/null
    [ -n "$C_PID" ] && kill "$C_PID" 2>/dev/null
    wait 2>/dev/null
}
trap cleanup EXIT INT TERM

echo "=== creator ==="
"$BIN" --role creator --cookies "$COOKIES" \
    --bench-mb "$BENCH_MB" --wait-peer-sec "$WAIT_PEER" --hold-sec "$HOLD" \
    > "$C_LOG" 2>&1 &
C_PID=$!

waited=0
JOIN_LINK=""
while [ "$waited" -lt "$WAIT_LINK" ]; do
    JOIN_LINK=$(grep -m1 "join_link:" "$C_LOG" | awk '{print $2}')
    [ -n "$JOIN_LINK" ] && break
    if ! kill -0 "$C_PID" 2>/dev/null; then
        echo "FAIL: creator died before printing join_link" >&2
        tail -40 "$C_LOG" >&2
        exit 1
    fi
    sleep 1
    waited=$((waited + 1))
done
[ -n "$JOIN_LINK" ] || { echo "FAIL: no join_link in ${WAIT_LINK}s" >&2; tail -40 "$C_LOG" >&2; exit 1; }
echo "join_link=$JOIN_LINK"

echo "=== joiner ==="
"$BIN" --role joiner --join-link "$JOIN_LINK" \
    --wait-peer-sec "$WAIT_PEER" --hold-sec "$HOLD" \
    > "$J_LOG" 2>&1 &
J_PID=$!

wait "$J_PID" 2>/dev/null
J_RC=$?

echo "=== joiner log tail ==="
tail -60 "$J_LOG"
echo ""
echo "=== creator log tail ==="
tail -30 "$C_LOG"

if grep -q "SFU did not forward DataChannels" "$J_LOG"; then
    echo "RESULT: SFU does NOT forward DataChannels - confirms README claim"
    exit 0
fi
if grep -q "SFU IS forwarding DCs" "$J_LOG"; then
    echo "RESULT: SFU FORWARDS DataChannels"
    grep "^  RESULT" "$J_LOG"
    exit 0
fi

echo "INCONCLUSIVE rc=$J_RC"
exit 1
