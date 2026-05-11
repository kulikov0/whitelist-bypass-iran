# Whitelist Bypass (Bale)

Tunnels internet traffic through the Bale Meet video calling platform to bypass government whitelist censorship.

Fork of [whitelist-bypass](https://github.com/eduarddeisling/whitelist-bypass) (VK / Telemost / WB Stream) adapted to Bale Meet.

## Setup

Step-by-step setup guide: [docs/SETUP.md](docs/SETUP.md)

## How it works

One tunnel mode is available: **Video** (VP8 data encoding). Bale's SFU does not allow useful SCTP DataChannels through, so the upstream DC mode is not available in this fork. The setup is **headless on both ends** - pure Go (Pion) talks to Bale's SFU directly, no browser. ChaCha20 obfuscation, configurable VP8 pacing, and the LiveKit backend are headless-only features.

### Video mode

The tunnel rides on a published VP8 video track. Tunnel framing and the multiplex layer above it are the same as in the upstream project's video mode.

```
Joiner (censored, Android)                          Creator (free internet)

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

Traffic goes through the Bale SFU, which is on the government whitelist. To DPI it looks like a normal video call.

## Components

- `relay/` - Go relay shared by both ends: SOCKS5 proxy, tun2socks plumbing, VP8 tunnel, ChaCha20 obfuscator, connection multiplexer, gomobile bindings
- `relay/bale/` - shared Bale wire format + LiveKit session glue, used by both creator and joiner
- `relay/tunnel/sequenced.go` - 4-byte sequence number + 16-frame reorder buffer wrapping the VP8 tunnel (Bale's SFU reorders frequently enough that a non-sequenced tunnel collapses under TCP)
- `headless/bale/` - Headless Bale creator: creates a call via the Bale HTTP/WS API, Pion VP8 tunnel, no browser
- `headless/bale-joiner/` - Desktop Bale joiner (counterpart to the creator, used for tests and Linux clients)
- `headless/tests/` - End-to-end smoke tests
- `android-app/` - Android joiner: VpnService + tun2socks + headless Pion
- `creator-app/` - Electron desktop creator app: GUI front-end that spawns the headless Go binary; suitable for both interactive use and deployments

## Download

Prebuilt binaries are available on [GitHub Releases](../../releases).

### Creator side (free internet, desktop)

Download and run the Electron app from [GitHub Releases](../../releases). It bundles the Go relay automatically.

1. Open the app
2. Click "Bale"
3. Log in, **create a new call** from the app
4. Copy the join link, send it to the joiner

**Important:** The call must be created from within the Creator app.

### Joiner side (censored, Android)

- **Android** - install `bale-bypass.apk` from [Releases](../../releases). Allow the VPN prompt on first launch. Paste the join link and tap GO; system-wide traffic flows through the call.
- **Linux desktop** - run the headless joiner; it exposes a SOCKS5 proxy on the given port for whatever you point at it. Useful for servers and Linux clients. Optional `--socks-user` / `--socks-pass` enable SOCKS5 username/password auth.
  - Bale: `headless-bale-joiner --join-link <link> --socks-port 1080 [--socks-user u --socks-pass p]`

The full step-by-step covers each platform in detail: see [docs/SETUP.md](docs/SETUP.md).

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
# Full release build (Android APK + Creator app + Headless binaries)
./make-release.sh

# Individual builds
./build-go.sh          # Go .aar, librelay.so, desktop relay binary
./build-app.sh         # Android APK
./build-headless.sh    # Headless binaries only (current platform)
./build-creator.sh     # Creator Electron app (all platforms)
```

Output in `prebuilts/`:

| File | Platform |
|---|---|
| `BaleBypass Creator-*-arm64.dmg` | macOS |
| `BaleBypass Creator-*-x64.exe` | Windows x64 |
| `BaleBypass Creator-*-ia32.exe` | Windows x86 |
| `BaleBypass Creator-*.AppImage` | Linux x64 |
| `bale-bypass.apk` | Android |

### Docker build

To build the project using Docker, execute:

```sh
docker compose -f docker-build/docker-compose.yml up
```

This will build all components (creator-app, headless, android app) into the `prebuild` folder (except the macOS creator).

### Headless creator

Pure Go creator that creates calls via the Bale API without a browser. No Electron, no JS hooks - Go Pion PeerConnection handles the VP8 tunnel directly.

```sh
./build-headless.sh
```

Two binaries are produced - the creator and a desktop joiner:

```sh
./headless/bale/headless-bale-creator        --cookies bale-cookies.json
./headless/bale-joiner/headless-bale-joiner  --join-link <link> --socks-port 1080
```

The creator expects cookies exported from the desktop Creator app (`Bale Cookies` button) as JSON `[{"name":"..","value":".."},...]`. The joiner is anonymous and uses Bale's public `/token` endpoint - no cookies required.

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
