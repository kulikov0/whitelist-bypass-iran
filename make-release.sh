#!/bin/sh
set -e

ROOT="$(cd "$(dirname "$0")" && pwd)"
PREBUILTS="$ROOT/prebuilts"

mkdir -p "$PREBUILTS"

echo "=== Building Go side ==="
"$ROOT/build-go.sh"

echo ""
echo "=== Building Android APK ==="
"$ROOT/build-app.sh"

echo ""
echo "=== Building creator-app ==="
"$ROOT/build-creator.sh"

echo ""
echo "=== Release complete ==="
ls -lh "$PREBUILTS/"
