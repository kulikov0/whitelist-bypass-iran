#!/bin/sh
# Smoke test for headless-bale-creator: spawn the binary, verify it creates a
# call link via Bale next-ws, joins LiveKit and reaches publisher PC connected.
#
# Usage:
#   ./test-bale-creator.sh [path/to/bale-cookies.json]
# Default cookies path: ../../bale-cookies.json (repo root).

set -u

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
CREATOR="$ROOT/headless/bale/headless-bale-creator"
COOKIES="${1:-$ROOT/bale-cookies.json}"
SETTLE_TIMEOUT=60

[ -x "$CREATOR" ] || { echo "FAIL: $CREATOR not built (cd headless/bale && go build -o headless-bale-creator)" >&2; exit 2; }
[ -r "$COOKIES" ] || { echo "FAIL: cookies file not readable: $COOKIES" >&2; exit 2; }

LOG=$(mktemp -t bale-c.XXXXXX.log)
PID=""
cleanup() {
    [ -n "$PID" ] && kill "$PID" 2>/dev/null
    wait 2>/dev/null
}
trap cleanup EXIT INT TERM

echo "=== headless-bale-creator ==="
"$CREATOR" --cookies "$COOKIES" --resources default > "$LOG" 2>&1 &
PID=$!

waited=0
JOIN_LINK=""
while [ "$waited" -lt "$SETTLE_TIMEOUT" ]; do
    JOIN_LINK=$(grep -m1 "join_link:" "$LOG" | awk '{print $2}')
    [ -n "$JOIN_LINK" ] && break
    if ! kill -0 "$PID" 2>/dev/null; then
        echo "FAIL: creator died before printing join_link" >&2
        tail -30 "$LOG" >&2
        exit 1
    fi
    sleep 1
    waited=$((waited + 1))
done

if [ -z "$JOIN_LINK" ]; then
    echo "FAIL: no join_link within ${SETTLE_TIMEOUT}s" >&2
    tail -30 "$LOG" >&2
    exit 1
fi
echo "join_link=$JOIN_LINK"

waited=0
while [ "$waited" -lt "$SETTLE_TIMEOUT" ]; do
    if grep -q "\[lk\] pub PC state: connected" "$LOG"; then
        break
    fi
    if ! kill -0 "$PID" 2>/dev/null; then
        echo "FAIL: creator died before reaching pub PC connected" >&2
        tail -30 "$LOG" >&2
        exit 1
    fi
    sleep 1
    waited=$((waited + 1))
done

if ! grep -q "\[lk\] pub PC state: connected" "$LOG"; then
    echo "FAIL: pub PC never connected within ${SETTLE_TIMEOUT}s" >&2
    tail -30 "$LOG" >&2
    exit 1
fi

ROOM=$(grep -m1 "\[lk\] join: room=" "$LOG" | sed -E 's/.*room=([a-zA-Z0-9-]+).*/\1/')
echo "room=$ROOM"
echo "PASS: bale creator end-to-end up to publisher connected"
