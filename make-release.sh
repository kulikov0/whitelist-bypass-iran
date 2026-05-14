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
echo "=== Building desktop joiner Electron app (Windows + Linux + macOS) ==="
"$ROOT/build-joiner-app.sh"

if [ "$(uname)" = "Darwin" ]; then
    echo ""
    echo "=== Building iOS app ==="
    "$ROOT/build-ios.sh"
else
    echo ""
    echo "=== Skipping iOS build (requires macOS) ==="
fi

"$ROOT/clean-prebuilts.sh"

echo ""
echo "=== Release complete ==="
ls -lh "$PREBUILTS/"
