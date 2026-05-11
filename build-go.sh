#!/bin/sh
set -e

ANDROID_HOME="${ANDROID_HOME:-$HOME/Library/Android/sdk}"
ANDROID_NDK_HOME="${ANDROID_NDK_HOME:-$ANDROID_HOME/ndk/29.0.14206865}"
export ANDROID_HOME ANDROID_NDK_HOME
export CGO_LDFLAGS="-Wl,-z,max-page-size=16384"
export PATH="$PATH:/opt/homebrew/bin:$HOME/go/bin"

command -v go >/dev/null || { echo "go not found"; exit 1; }
command -v gomobile >/dev/null || { echo "gomobile not found, run: go install golang.org/x/mobile/cmd/gomobile@latest && gomobile init"; exit 1; }
command -v gobind >/dev/null || { echo "gobind not found, run: go install golang.org/x/mobile/cmd/gobind@latest"; exit 1; }
[ -d "$ANDROID_NDK_HOME" ] || { echo "NDK not found at $ANDROID_NDK_HOME, set ANDROID_NDK_HOME env var to point at an installed NDK"; exit 1; }

ROOT="$(cd "$(dirname "$0")" && pwd)"
cd "$ROOT/relay"

echo "Building gomobile .aar..."
gomobile bind -v -target=android -androidapi 23 -o mobile.aar ./androidbind/

echo "Copying .aar to android-app/libs..."
mkdir -p ../android-app/app/libs
cp mobile.aar ../android-app/app/libs/mobile.aar

echo "Cross-compiling relay binary for Android..."
GOOS=linux GOARCH=arm64 go build -o ../android-app/app/src/main/jniLibs/arm64-v8a/librelay.so .
GOOS=linux GOARCH=arm   go build -o ../android-app/app/src/main/jniLibs/armeabi-v7a/librelay.so .
echo "Relay binary built for arm64-v8a and armeabi-v7a"

echo ""
echo "Building desktop relay..."
go build -o relay .

echo "Done"
ls -lh "$ROOT/relay/relay" "$ROOT/relay/mobile.aar"
