#!/bin/sh
set -e
ROOT="$(cd "$(dirname "$0")" && pwd)"

echo "Building headless-bale-creator..."
go -C "$ROOT/headless/bale" build -trimpath -ldflags="-s -w" -o headless-bale-creator .

echo "Building headless-bale-joiner..."
go -C "$ROOT/headless/bale-joiner" build -trimpath -ldflags="-s -w" -o headless-bale-joiner .

echo "Done."
ls -lh "$ROOT/headless/bale/headless-bale-creator" "$ROOT/headless/bale-joiner/headless-bale-joiner"
