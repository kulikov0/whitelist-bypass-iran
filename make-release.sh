#!/bin/sh
set -e

ROOT="$(cd "$(dirname "$0")" && pwd)"
PREBUILTS="$ROOT/prebuilts"

export RELEASE_PLATFORM=all

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
echo "=== Building desktop joiner Electron app ==="
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
echo "=== Desktop installer coverage ==="
for pat in "*.dmg:macOS" "*.exe:Windows" "*.AppImage:Linux"; do
    glob=${pat%%:*}
    label=${pat##*:}
    if ls "$PREBUILTS"/$glob >/dev/null 2>&1; then
        echo "  [ok]   $label"
    else
        echo "  [MISS] $label"
    fi
done

echo ""
echo "=== Release complete ==="
ls -lh "$PREBUILTS/"
