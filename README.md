# Whitelist Bypass (Bale)

Tunnels internet traffic through the Bale Meet video calling platform to bypass government whitelist censorship.

Fork of [whitelist-bypass](https://github.com/eduarddeisling/whitelist-bypass) (VK / Telemost / WB Stream) adapted to Bale Meet.

## Setup

Step-by-step setup guides:

- English: [docs/en/SETUP.md](docs/en/SETUP.md), VPS deployment: [docs/en/VPS.md](docs/en/VPS.md)
- فارسی: [docs/fa/SETUP.md](docs/fa/SETUP.md), استقرار VPS: [docs/fa/VPS.md](docs/fa/VPS.md)

## How it works

Two tunnel modes are available: **Video** (VP8 data encoding) and **DC** (DataChannel).
The headless creator adapts to whichever mode the joiner picks at connect time, on every new session.

### Video mode

The tunnel rides on a published VP8 video track. Tunnel framing and the multiplex layer above it are the same as in the upstream project's video mode.

```
Joiner (censored, Android/iOS/Desktop)              Creator (free internet)

All apps
  |
VpnService (captures all traffic)
  |
tun2socks (IP -> TCP)
  |
SOCKS5 proxy (Go, :1080)
  |
Headless joiner (Pion)                    Headless creator (Pion)
  |                                         |
VP8 video track  <----- SFU ------>  VP8 video track
                                            |
                                        Relay bridge
                                            |
                                        Internet
```

### DC mode

Same pipeline, but the tunnel rides on a SCTP DataChannel instead of a VP8 video track. Lower peak throughput than VP8 but a bit lighter on CPU. Tunnel framing and multiplex are the same as Video mode.

Traffic goes through the Bale SFU, which is on the government whitelist. To DPI it looks like a normal video call.

## Components

- `relay/` - Go relay shared by both ends: SOCKS5 proxy, tun2socks plumbing, VP8 and DC tunnels, ChaCha20 obfuscator, connection multiplexer, gomobile and iOS bindings
- `relay/bale/` - shared Bale wire format and LiveKit session glue, used by both creator and joiner
- `relay/tunnel/sequenced.go` - 4-byte sequence number and 16-frame reorder buffer wrapping the VP8 tunnel (Bale's SFU reorders frequently enough that a non-sequenced tunnel collapses under TCP)
- `relay/tunnel/dctunnel.go` - DataChannel tunnel wrapper with LiveKit DataPacket framing
- `headless/bale/` - Headless Bale creator: creates a call via the Bale HTTP/WS API, Pion VP8/DC tunnel, no browser
- `headless/bale-joiner/` - Desktop Bale joiner (counterpart to the creator, used for tests and Linux clients)
- `headless/tests/` - End-to-end smoke tests
- `android-app/` - Android joiner: VpnService + tun2socks + headless Pion
- `ios-proxy-app/` - iOS joiner: SOCKS5 + headless Pion via the gomobile xcframework
- `creator-app/` - Electron desktop creator app: GUI front-end that spawns the headless Go binary; suitable for both interactive use and deployments
- `joiner-desktop-app/` - Electron desktop joiner app: GUI front-end for Windows/macOS/Linux with system VPN (TUN) and SOCKS5-only modes

## Download

Prebuilt binaries are available on [GitHub Releases](../../releases).

### Creator side (free internet, desktop)

Download and run the Electron Creator app from [GitHub Releases](../../releases). It bundles the Go relay automatically.

1. Open the app
2. Click **+** to open a new tab
3. Click **Bale**
4. Log in to Bale and the app creates a call automatically
5. Copy the join link, send it to the joiner

For running the Creator headless on a server, see [docs/en/VPS.md](docs/en/VPS.md).

### Joiner side (censored)

Four forms are available; pick whichever fits the device:

- **Android** - install `whitelist-bypass.apk` from [Releases](../../releases). Allow the VPN prompt on first launch. Paste the join link and tap GO; system-wide traffic flows through the call.
- **iOS** - install `whitelist-bypass-proxy.ipa` from [Releases](../../releases) (sideload via AltStore, Sideloadly, or your developer account). Exposes a local SOCKS5 proxy only, no system VPN. To proxy the whole device, point any SOCKS5-capable VPN app (Happ, Shadowrocket, Streisand, ...) at the SOCKS5 endpoint the app shows; or set the proxy per app (Telegram has built-in support).
- **Desktop (Windows / macOS / Linux)** - install `WhitelistBypass Joiner` (`.exe` / `.dmg` / `.AppImage`) from [Releases](../../releases). GUI joiner that can either bring up a system VPN (TUN) tunnel or run as a local SOCKS5 proxy. TUN mode needs admin/root.
- **Linux headless** - run the headless joiner; it exposes a SOCKS5 proxy on the given port for whatever you point at it. Useful for servers and Linux clients. Optional `--socks-user` / `--socks-pass` enable SOCKS5 username/password auth.
  - `headless-bale-joiner --join-link <link> --socks-port 1080 [--socks-user u --socks-pass p]`

The full step-by-step covers each platform in detail: see [docs/en/SETUP.md](docs/en/SETUP.md) (or [docs/fa/SETUP.md](docs/fa/SETUP.md) in Persian).

## Building from source

### Requirements

- Go 1.26+
- gomobile (`go install golang.org/x/mobile/cmd/gomobile@latest`)
- gobind (`go install golang.org/x/mobile/cmd/gobind@latest`)
- Android SDK + NDK 29
- Java 11+
- Node.js 18+

### Build scripts

```sh
# Full release build (Android APK + Creator app + Desktop Joiner app + Headless binaries + iOS IPA on macOS)
./make-release.sh

# Individual builds
./build-go.sh              # Go .aar, librelay.so, desktop relay binary
./build-app.sh             # Android APK
./build-headless.sh        # Headless binaries only (current platform)
./build-creator.sh         # Creator Electron app (all platforms)
./build-joiner-app.sh      # Desktop Joiner Electron app (all platforms)
./build-desktop-joiner.sh  # Headless Bale desktop joiner binary (all platforms)
./build-ios.sh             # Go .xcframework for iOS
```

Output in `prebuilts/`:

| File | Platform |
|---|---|
| `Whitelist Bypass Creator-*-arm64.dmg` | macOS |
| `Whitelist Bypass Creator-*-x64.exe` | Windows x64 |
| `Whitelist Bypass Creator-*-ia32.exe` | Windows x86 |
| `Whitelist Bypass Creator-*.AppImage` | Linux x64 |
| `WhitelistBypass Joiner-*-arm64.dmg` | macOS |
| `WhitelistBypass Joiner-*-x64.exe` | Windows x64 |
| `WhitelistBypass Joiner-*-ia32.exe` | Windows x86 |
| `WhitelistBypass Joiner-*-x86_64.AppImage` | Linux x64 |
| `whitelist-bypass.apk` | Android |
| `whitelist-bypass-proxy.ipa` | iOS, unsigned |
| `headless-bale-creator-linux-x64` | Linux x64 |
| `headless-bale-creator-linux-ia32` | Linux x86 |

### iOS

Requires Xcode and macOS.

```sh
./build-ios.sh
```

This builds `Mobile.xcframework` into `ios-proxy-app/`. Then open `ios-proxy-app/whitelist-bypass-proxy.xcodeproj` in Xcode, select your signing team in Signing & Capabilities, and build to device.

### Docker build

To build the project using Docker, execute:

```sh
docker compose -f docker-build/docker-compose.yml up
```

This will build all components (creator-app, headless, android app) into the `prebuild` folder (except the macOS creator).

### Headless creator and joiner

Pure Go binaries that talk to Bale's API/SFU directly. No Electron, no JS hooks.

```sh
./build-headless.sh
```

Two binaries are produced - the creator and the desktop joiner:

```sh
./headless/bale/headless-bale-creator        --cookies bale-cookies.json
./headless/bale-joiner/headless-bale-joiner  --join-link <link> --socks-port 1080
```

The creator expects cookies exported from the desktop Creator app (`Bale Cookies` button) as JSON `[{"name":"..","value":".."},...]`. The joiner is anonymous and uses Bale's public `/token` endpoint, no cookies required.

#### Common flags

| Flag | Creator | Joiner | Description |
|---|---|---|---|
| `--cookies <path>` | yes | - | path to cookies JSON |
| `--cookie-string <str>` | yes | - | raw cookie string `name=val; name=val` |
| `--join-link <link>` | - | yes | `https://meet.bale.ai/i/<code>` |
| `--name <str>` | - | yes | display name shown in the meeting |
| `--socks-port <n>` | - | yes | local SOCKS5 port |
| `--socks-user <u>` | - | yes | SOCKS5 username (optional) |
| `--socks-pass <p>` | - | yes | SOCKS5 password (optional) |
| `--tunnel-mode <m>` | - | yes | `vp8` or `dc` (default `vp8`) |
| `--write-file <path>` | yes | - | append the active join link to this file (one link per line) |
| `--resources <mode>` | yes | yes | `default` / `moderate` / `unlimited` |
| `--vp8-fps <n>` | yes | yes | VP8 frame rate (default 24) |
| `--vp8-batch <n>` | yes | yes | VP8 batch multiplier (default 30) |

#### Resource modes

| Mode | `read-buf` | `mem-limit` |
|---|---|---|
| `moderate`  | 16 KB | 64 MB |
| `default`   | 32 KB | 128 MB |
| `unlimited` | 64 KB | 256 MB |

- `read-buf` - TCP read buffer size. Smaller = more frequent backpressure checks, less bursty memory
- `mem-limit` - Go runtime soft memory limit; makes GC more aggressive near the cap

## License

[MIT](LICENSE)
