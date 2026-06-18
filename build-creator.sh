#!/bin/sh
set -e

ROOT="$(cd "$(dirname "$0")" && pwd)"
RELAY_DIR="$ROOT/relay"
CREATOR_DIR="$ROOT/creator-app"
HEADLESS_DIR="$ROOT/headless"
HEADLESS_BALE_DIR="$HEADLESS_DIR/bale"

echo "=== Building relay binaries ==="
cd "$RELAY_DIR"

if command -v lipo >/dev/null; then
    echo "macOS universal..."
    GOOS=darwin GOARCH=amd64 go build -o relay-darwin-amd64 .
    GOOS=darwin GOARCH=arm64 go build -o relay-darwin-arm64 .
    lipo -create -output relay-darwin relay-darwin-amd64 relay-darwin-arm64
    rm relay-darwin-amd64 relay-darwin-arm64
else
    echo "lipo not found, skipping macOS universal build"
fi

echo "Windows x64..."
GOOS=windows GOARCH=amd64 go build -o relay-windows-x64.exe .
echo "Windows x86..."
GOOS=windows GOARCH=386 go build -o relay-windows-ia32.exe .

echo "Linux x64..."
GOOS=linux GOARCH=amd64 go build -o relay-linux-x64 .
echo "Linux x86..."
GOOS=linux GOARCH=386 go build -o relay-linux-ia32 .

ls -lh relay-darwin relay-windows-*.exe relay-linux-* 2>/dev/null || true

echo ""
echo "=== Building headless-bale-creator ==="
cd "$HEADLESS_BALE_DIR"

if command -v lipo >/dev/null; then
    echo "macOS universal..."
    GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o "$HEADLESS_DIR/headless-bale-darwin-amd64" .
    GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o "$HEADLESS_DIR/headless-bale-darwin-arm64" .
    lipo -create -output "$HEADLESS_DIR/headless-bale-darwin" "$HEADLESS_DIR/headless-bale-darwin-amd64" "$HEADLESS_DIR/headless-bale-darwin-arm64"
    rm "$HEADLESS_DIR/headless-bale-darwin-amd64" "$HEADLESS_DIR/headless-bale-darwin-arm64"
else
    echo "lipo not found, skipping macOS universal build for headless-bale"
fi

echo "Windows x64..."
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o "$HEADLESS_DIR/headless-bale-windows-x64.exe" .
echo "Windows x86..."
GOOS=windows GOARCH=386 go build -ldflags="-s -w" -o "$HEADLESS_DIR/headless-bale-windows-ia32.exe" .

echo "Linux x64..."
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o "$HEADLESS_DIR/headless-bale-linux-x64" .
echo "Linux x86..."
GOOS=linux GOARCH=386 go build -ldflags="-s -w" -o "$HEADLESS_DIR/headless-bale-linux-ia32" .

ls -lh "$HEADLESS_DIR"/headless-bale-darwin 2>/dev/null || true

echo ""
echo "=== Building Electron creator ==="
cd "$CREATOR_DIR"
npm install --quiet 2>&1
npm run build 2>&1

echo ""
echo "--- macOS ---"
npx electron-builder --mac || true

echo ""
echo "--- Windows x64 ---"
cp "$RELAY_DIR/relay-windows-x64.exe" "$RELAY_DIR/relay-bundle.exe"
cp "$HEADLESS_DIR/headless-bale-windows-x64.exe" "$HEADLESS_DIR/headless-bale-bundle.exe"
npx electron-builder --win --x64

echo ""
echo "--- Windows x86 ---"
cp "$RELAY_DIR/relay-windows-ia32.exe" "$RELAY_DIR/relay-bundle.exe"
cp "$HEADLESS_DIR/headless-bale-windows-ia32.exe" "$HEADLESS_DIR/headless-bale-bundle.exe"
npx electron-builder --win --ia32

echo ""
echo "--- Linux x64 ---"
cp "$RELAY_DIR/relay-linux-x64" "$RELAY_DIR/relay-bundle"
cp "$HEADLESS_DIR/headless-bale-linux-x64" "$HEADLESS_DIR/headless-bale-bundle"
npx electron-builder --linux --x64

echo ""
echo "=== Copying headless binaries to prebuilts ==="
mkdir -p "$ROOT/prebuilts"
cp "$HEADLESS_DIR/headless-bale-linux-x64" "$ROOT/prebuilts/headless-bale-creator-linux-x64"
cp "$HEADLESS_DIR/headless-bale-linux-ia32" "$ROOT/prebuilts/headless-bale-creator-linux-ia32"

rm -f "$RELAY_DIR"/relay-darwin "$RELAY_DIR"/relay-windows-*.exe "$RELAY_DIR"/relay-linux-*
rm -f "$RELAY_DIR"/relay-bundle "$RELAY_DIR"/relay-bundle.exe
rm -f "$HEADLESS_DIR"/headless-bale-darwin "$HEADLESS_DIR"/headless-bale-windows-*.exe "$HEADLESS_DIR"/headless-bale-linux-*
rm -f "$HEADLESS_DIR"/headless-bale-bundle "$HEADLESS_DIR"/headless-bale-bundle.exe

"$ROOT/clean-prebuilts.sh"

echo ""
echo "=== Done ==="
ls -lh "$ROOT/prebuilts/" 2>/dev/null || true
