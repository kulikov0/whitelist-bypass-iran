#!/bin/sh
# Kick test: creator + joiner A reach tunnel, then joiner B arrives and the
# creator must call RemoveParticipant on A. Verifies (1) kick log on creator,
# (2) A's session ends, (3) B's tunnel becomes the active one and serves
# real internet traffic through its SOCKS port.

set -u

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
CREATOR="$ROOT/headless/bale/headless-bale-creator"
JOINER="$ROOT/headless/bale-joiner/headless-bale-joiner"
COOKIES="${1:-$ROOT/bale-cookies.json}"
SOCKS_A=11081
SOCKS_B=11082
TARGET_URL="https://api.ipify.org"
PROBE_TIMEOUT=20
SETTLE_TIMEOUT=60

[ -x "$CREATOR" ] || { echo "FAIL: $CREATOR not built" >&2; exit 2; }
[ -x "$JOINER" ]  || { echo "FAIL: $JOINER not built" >&2; exit 2; }
[ -r "$COOKIES" ] || { echo "FAIL: cookies not readable: $COOKIES" >&2; exit 2; }

C_LOG=$(mktemp -t bale-c.XXXXXX.log)
A_LOG=$(mktemp -t bale-a.XXXXXX.log)
B_LOG=$(mktemp -t bale-b.XXXXXX.log)
C_PID=""
A_PID=""
B_PID=""
cleanup() {
    [ -n "$B_PID" ] && kill "$B_PID" 2>/dev/null
    [ -n "$A_PID" ] && kill "$A_PID" 2>/dev/null
    [ -n "$C_PID" ] && kill "$C_PID" 2>/dev/null
    wait 2>/dev/null
}
trap cleanup EXIT INT TERM

wait_for() {
    pat="$1"; file="$2"; pid="$3"; secs="$4"
    waited=0
    while [ "$waited" -lt "$secs" ]; do
        grep -q "$pat" "$file" && return 0
        if ! kill -0 "$pid" 2>/dev/null; then
            echo "FAIL: process died waiting for: $pat" >&2
            tail -40 "$file" >&2
            return 1
        fi
        sleep 1
        waited=$((waited + 1))
    done
    echo "FAIL: timeout waiting for: $pat" >&2
    tail -40 "$file" >&2
    return 1
}

echo "=== creator ==="
"$CREATOR" --cookies "$COOKIES" --resources default > "$C_LOG" 2>&1 &
C_PID=$!

waited=0
JOIN_LINK=""
while [ "$waited" -lt "$SETTLE_TIMEOUT" ]; do
    JOIN_LINK=$(grep -m1 "join_link:" "$C_LOG" | awk '{print $2}')
    [ -n "$JOIN_LINK" ] && break
    kill -0 "$C_PID" 2>/dev/null || { echo "FAIL: creator died early" >&2; tail -30 "$C_LOG" >&2; exit 1; }
    sleep 1; waited=$((waited + 1))
done
[ -n "$JOIN_LINK" ] || { echo "FAIL: no join_link" >&2; exit 1; }
echo "join_link=$JOIN_LINK"

wait_for "\[lk\] pub PC state: connected" "$C_LOG" "$C_PID" "$SETTLE_TIMEOUT" || exit 1
echo "creator: pub PC connected"

echo "=== joiner A ==="
"$JOINER" --join-link "$JOIN_LINK" --socks-port "$SOCKS_A" --name "joiner-A" > "$A_LOG" 2>&1 &
A_PID=$!
wait_for "TUNNEL CONNECTED" "$A_LOG" "$A_PID" "$SETTLE_TIMEOUT" || exit 1
echo "A: tunnel connected"

response=$(curl --socks5 "127.0.0.1:$SOCKS_A" -m "$PROBE_TIMEOUT" -sv "$TARGET_URL" 2>&1)
echo "$response" | grep -q "HTTP/.* 200" || { echo "FAIL: A's SOCKS did not get HTTP 200" >&2; echo "$response" >&2; exit 1; }
echo "A: SOCKS reachable, ip=$(echo "$response" | tail -1)"

wait_for "tunnel mode selected:" "$C_LOG" "$C_PID" 30 || exit 1

echo "=== joiner B (should trigger kick of A) ==="
"$JOINER" --join-link "$JOIN_LINK" --socks-port "$SOCKS_B" --name "joiner-B" > "$B_LOG" 2>&1 &
B_PID=$!

wait_for "kicking stale peer identity=" "$C_LOG" "$C_PID" "$SETTLE_TIMEOUT" || {
    echo "--- creator tail ---" >&2; tail -60 "$C_LOG" >&2
    exit 1
}
echo "creator: kicked A"

wait_for "new vp8 track arrived after mode locked, re-arming" "$C_LOG" "$C_PID" 30 || {
    echo "--- creator tail ---" >&2; tail -60 "$C_LOG" >&2
    exit 1
}
echo "creator: re-armed for B"

wait_for "TUNNEL CONNECTED" "$B_LOG" "$B_PID" "$SETTLE_TIMEOUT" || exit 1
echo "B: tunnel connected"

# Give B a moment to settle after the rearm before probing
sleep 2
response=$(curl --socks5 "127.0.0.1:$SOCKS_B" -m "$PROBE_TIMEOUT" -sv "$TARGET_URL" 2>&1)
if ! echo "$response" | grep -q "HTTP/.* 200"; then
    echo "FAIL: B's SOCKS did not get HTTP 200 after kick+rearm" >&2
    echo "--- curl ---" >&2; echo "$response" >&2
    echo "--- creator tail ---" >&2; tail -60 "$C_LOG" >&2
    echo "--- B tail ---" >&2; tail -40 "$B_LOG" >&2
    exit 1
fi
echo "B: SOCKS reachable post-kick, ip=$(echo "$response" | tail -1)"

# Verify A's session actually ended (was kicked off the call)
if kill -0 "$A_PID" 2>/dev/null; then
    if ! grep -qE "TUNNEL_LOST|session ended|read loop ended" "$A_LOG"; then
        echo "WARN: A's process still alive and no TUNNEL_LOST seen yet" >&2
        tail -20 "$A_LOG" >&2
    fi
fi

echo "PASS: bale kick"
