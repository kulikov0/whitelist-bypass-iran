#!/bin/sh
# Full e2e: spawn headless-bale-creator, feed its join_link to anonymous
# headless-bale-joiner, then verify SOCKS5 reaches the public internet and
# measure throughput.
#
# Usage:
#   ./test-bale-pair-e2e.sh [path/to/bale-cookies.json]

set -u

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
CREATOR="$ROOT/headless/bale/headless-bale-creator"
JOINER="$ROOT/headless/bale-joiner/headless-bale-joiner"
COOKIES="${1:-$ROOT/bale-cookies.json}"
SOCKS_PORT=11080
TARGET_URL="https://api.ipify.org"
PROBE_TIMEOUT=20
SETTLE_TIMEOUT=60

[ -x "$CREATOR" ] || { echo "FAIL: $CREATOR not built" >&2; exit 2; }
[ -x "$JOINER" ]  || { echo "FAIL: $JOINER not built" >&2; exit 2; }
[ -r "$COOKIES" ] || { echo "FAIL: cookies not readable: $COOKIES" >&2; exit 2; }

C_LOG=$(mktemp -t bale-c.XXXXXX.log)
J_LOG=$(mktemp -t bale-j.XXXXXX.log)
C_PID=""
J_PID=""
cleanup() {
    [ -n "$J_PID" ] && kill "$J_PID" 2>/dev/null
    [ -n "$C_PID" ] && kill "$C_PID" 2>/dev/null
    wait 2>/dev/null
}
trap cleanup EXIT INT TERM

echo "=== creator ==="
"$CREATOR" --cookies "$COOKIES" --resources default > "$C_LOG" 2>&1 &
C_PID=$!

waited=0
JOIN_LINK=""
while [ "$waited" -lt "$SETTLE_TIMEOUT" ]; do
    JOIN_LINK=$(grep -m1 "join_link:" "$C_LOG" | awk '{print $2}')
    [ -n "$JOIN_LINK" ] && break
    if ! kill -0 "$C_PID" 2>/dev/null; then
        echo "FAIL: creator died early" >&2
        tail -30 "$C_LOG" >&2
        exit 1
    fi
    sleep 1
    waited=$((waited + 1))
done
[ -n "$JOIN_LINK" ] || { echo "FAIL: creator did not print join_link in ${SETTLE_TIMEOUT}s" >&2; tail -30 "$C_LOG" >&2; exit 1; }
echo "join_link=$JOIN_LINK"

waited=0
while [ "$waited" -lt "$SETTLE_TIMEOUT" ]; do
    grep -q "\[lk\] pub PC state: connected" "$C_LOG" && break
    if ! kill -0 "$C_PID" 2>/dev/null; then
        echo "FAIL: creator died before pub PC connected" >&2
        tail -30 "$C_LOG" >&2
        exit 1
    fi
    sleep 1
    waited=$((waited + 1))
done
grep -q "\[lk\] pub PC state: connected" "$C_LOG" || { echo "FAIL: creator never reached pub PC connected" >&2; tail -30 "$C_LOG" >&2; exit 1; }
echo "creator: pub PC connected"

echo "=== joiner ==="
"$JOINER" --join-link "$JOIN_LINK" --socks-port "$SOCKS_PORT" > "$J_LOG" 2>&1 &
J_PID=$!

waited=0
while [ "$waited" -lt "$SETTLE_TIMEOUT" ]; do
    if grep -q "TUNNEL CONNECTED" "$J_LOG"; then
        break
    fi
    if ! kill -0 "$J_PID" 2>/dev/null; then
        echo "FAIL: joiner died early" >&2
        tail -40 "$J_LOG" >&2
        exit 1
    fi
    sleep 1
    waited=$((waited + 1))
done

if ! grep -q "TUNNEL CONNECTED" "$J_LOG"; then
    echo "FAIL: joiner did not reach TUNNEL CONNECTED in ${SETTLE_TIMEOUT}s" >&2
    tail -40 "$J_LOG" >&2
    exit 1
fi

waited=0
while [ "$waited" -lt 10 ]; do
    nc -z 127.0.0.1 "$SOCKS_PORT" 2>/dev/null && break
    sleep 1
    waited=$((waited + 1))
done
nc -z 127.0.0.1 "$SOCKS_PORT" 2>/dev/null || { echo "FAIL: SOCKS5 port $SOCKS_PORT never opened" >&2; exit 1; }

response=$(curl --socks5 "127.0.0.1:$SOCKS_PORT" -m "$PROBE_TIMEOUT" -sv "$TARGET_URL" 2>&1)
if ! echo "$response" | grep -q "HTTP/.* 200"; then
    echo "FAIL: SOCKS5 request did not return HTTP 200" >&2
    echo "--- curl ---" >&2; echo "$response" >&2
    echo "--- joiner tail ---" >&2; tail -30 "$J_LOG" >&2
    echo "--- creator tail ---" >&2; tail -30 "$C_LOG" >&2
    exit 1
fi
remote_ip=$(echo "$response" | tail -1)
echo "remote_ip=$remote_ip"

speed=$(curl --socks5 "127.0.0.1:$SOCKS_PORT" -m 30 -s -o /dev/null \
    -w "%{speed_download}" \
    "https://speed.cloudflare.com/__down?bytes=10485760")
if [ -z "$speed" ] || [ "$speed" = "0" ]; then
    echo "FAIL: throughput probe returned 0 B/s" >&2
    echo "--- joiner tail ---" >&2; tail -30 "$J_LOG" >&2
    echo "--- creator tail ---" >&2; tail -30 "$C_LOG" >&2
    exit 1
fi
mbps=$(awk -v b="$speed" 'BEGIN { printf "%.2f", b * 8 / 1000000 }')
echo "throughput=${mbps} Mbps"

# Require a sensible floor: with the seq+reorder layer the tunnel
# sustains 3-6 Mbps on a healthy path. Anything below 1 Mbps means the
# data plane is degraded.
floor_kbps=125000
if awk -v s="$speed" -v f="$floor_kbps" 'BEGIN { exit !(s < f) }'; then
    echo "FAIL: throughput ${mbps} Mbps below 1 Mbps floor" >&2
    exit 1
fi

echo "PASS: bale e2e"
