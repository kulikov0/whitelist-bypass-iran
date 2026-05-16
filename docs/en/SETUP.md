# Quick start

Download the files you need from [GitHub Releases](https://github.com/kulikov0/whitelist-bypass-iran/releases).

> فارسی: [docs/fa/SETUP.md](../fa/SETUP.md)

## Contents

- [What you need](#what-you-need)
- [Creator (desktop)](#creator-desktop)
- [Creator (headless, server)](#creator-headless-server)
- [Joiner (Android)](#joiner-android)
- [Joiner (iOS)](#joiner-ios)
- [Joiner (Desktop: Windows / macOS / Linux)](#joiner-desktop-windows--macos--linux)
- [Joiner (Linux, headless)](#joiner-linux-headless)

## What you need

- **Creator** (free-internet side) - desktop or headless on a server
- **Joiner** (censored side) - Android, iOS, or desktop

Two tunnel modes: **Video** (VP8) and **DC** (DataChannel). The headless creator adapts to whichever mode the joiner picks at connect time, on every new session. Video is the default; DC is lower throughput but a bit lighter on CPU.

> The Creator must run somewhere with unrestricted internet (outside Iran or on a working VPN), and must be online before any joiner tries to connect. Both Creator and Joiner are headless under the hood - Pion talks to the Bale SFU directly, no browser is involved.

## Creator (desktop)

![Interface](../res/desktop_interface.png)

1. Download and launch the Creator app
2. Click **+** to open a new tab
3. Click **Bale**
4. Sign in to your Bale account in the embedded window
5. The app automatically creates a call and shows the join link
6. Copy the join link and send it to the Joiner

To run the Creator without a GUI (on a server), export cookies with the **Bale Cookies** button and use them with the headless binary (see below).

> Running the desktop Creator on a VPS without a graphical environment via XPRA - see the upstream guide [vps/SETUP.md](https://github.com/kulikov0/whitelist-bypass/blob/main/docs/vps/SETUP.md). The procedure is identical, just substitute the Iran-fork AppImage.

## Creator (headless, server)

For running the Creator on a server without a GUI.

### Preparing cookies

Cookies are required to authenticate against Bale. Export them from the desktop Creator:

1. Open the Creator on a desktop machine
2. Sign in to Bale (regular mode)
3. Click **Bale Cookies**
4. Copy the resulting `bale-cookies.json` file to your server

### Running

```sh
./headless-bale-creator --cookies bale-cookies.json
```

The Creator creates a call and prints the join link to the log. Send that link to the Joiner.

If `bale-cookies.json` is missing or expired, the binary prints `WAITING_FOR_COOKIES` and reads a fresh raw cookie string from stdin. Useful for automated re-auth pipelines.

### Flags

| Flag | Description |
|---|---|
| `--cookies <path>` | Path to cookies JSON |
| `--cookie-string <str>` | Cookies as a raw string (`name=val; name=val`) |
| `--resources <mode>` | `default` / `moderate` / `unlimited` |
| `--write-file <path>` | Append the active join link to this file (one line per session) |
| `--vp8-fps <n>` | VP8 frame rate (default `24`) |
| `--vp8-batch <n>` | VP8 batch multiplier (default `30`) |

### Resource modes

| Mode | `read-buf` | `mem-limit` | When to use |
|---|---|---|---|
| `moderate`  | 16 KB | 64 MB  | Low-RAM VPS |
| `default`   | 32 KB | 128 MB | Normal use |
| `unlimited` | 64 KB | 256 MB | Max throughput (may throttle from congestion control) |

- `read-buf` - TCP read buffer. Smaller = more frequent backpressure checks, less bursty memory.
- `mem-limit` - Go runtime soft memory limit; makes GC more aggressive near the cap.

## Joiner (Android)

![Interface](../res/android_interface.png)

1. Download and install `whitelist-bypass.apk`
2. On first launch, allow the VPN prompt
3. Open settings (icon to the right of GO) and pick the tunnel mode (**Video** or **DC**). The headless Creator adapts automatically
4. Paste the call link into the input field
5. Tap **GO**
6. Wait for the **Tunnel active** status - all device traffic now flows through the call

### Settings

- **Tunnel mode** - VP8 (Video) or DC
- **Name in call** - display name shown in the meeting (random by default)
- **Split tunneling** - pick which apps go through the tunnel
- **Proxy settings** - SOCKS5 port and auth. "Proxy only" mode runs the SOCKS5 listener without enabling the VPN.
- **DNS settings** - system DNS or custom
- **VP8 pacing** - override VP8 pacing parameters (see below)
- **Reconnect on app start** - auto-reconnect to the last call link on launch
- **Show logs** - show the log panel for debugging

### VP8 pacing

Controls how often the joiner publishes VP8 frames to the SFU. Configured on the joiner only; the creator picks up the values at session start.

- **Override VP8 pacing** - off by default. With the checkbox off, defaults are `fps=24 batch=30` (~6.5 Mbps theoretical ceiling). Turning it on exposes two fields.
- **FPS** - nominal VP8 frame rate. Range 1..240. Typical 24-30.
- **Batch** - tick density multiplier. Actual send rate ~ `fps x batch` frames/sec. Range 1..256.

Throughput ~ `fps x batch x 1126 bytes/frame`. Examples:

| fps | batch | ceiling |
|----:|------:|--------:|
| 24  | 1     | ~27 KB/s |
| 24  | 8     | ~216 KB/s |
| 24  | 30    | ~810 KB/s (~6.5 Mbps) |

Higher batch = more load on the phone CPU and the SFU. If logs show packet drops or the connection is unstable, lower the batch.

### If it doesn't work

Try changing DNS: in app settings open **DNS settings** and switch to Custom (defaults to `8.8.8.8` / `8.8.4.4`). Also check that Android's system DNS is set to "Automatic" (Private DNS off).

## Joiner (iOS)

iOS only supports SOCKS5 proxy mode (no system VPN). To proxy the whole device, use a SOCKS5-capable VPN app (Happ, Shadowrocket, Streisand, etc.).

1. Download `whitelist-bypass-proxy.ipa` (unsigned) from [GitHub Releases](https://github.com/kulikov0/whitelist-bypass-iran/releases) and install it. For installing unsigned IPAs on iOS, AltStore, Sideloadly, or your own developer account all work. Or build from source (see README).
2. Install any SOCKS5-capable VPN app (Happ is free on the App Store).
3. Open whitelist-bypass, pick the tunnel mode (**VP8** or **DC**), paste the call link, tap **Go**. The headless Creator adapts to whichever mode you pick.
4. Wait for **Tunnel Active**. The app displays a local SOCKS5 endpoint (e.g. `socks5://user:pass@127.0.0.1:1081`).
5. Copy the SOCKS5 parameters out of whitelist-bypass.
6. Paste them into the VPN app and connect - all device traffic now flows through the tunnel.

### Connecting via Happ

Happ is a free App Store app that supports SOCKS5 in the v2ray URL format.

1. In whitelist-bypass, wait for **Tunnel Active** and tap **Copy v2ray URL** - a `socks://...@127.0.0.1:1081#WLB-1081` link goes to the clipboard.

   ![whitelist-bypass: Copy v2ray URL](../res/ios_interface.jpeg)

2. Open Happ, tap **+** in the top-right and pick **Import from clipboard**.

   ![Happ: Import from clipboard](../res/ios_happ_s1.jpeg)

3. A `WLB-1081` server entry appears in the list - turn the tunnel on.

   ![Happ: tunnel running](../res/ios_happ_s2.jpeg)

> The app intentionally avoids NetworkExtension, so it installs through a free Apple Developer account (no $99/year). The downside: the SOCKS5 port may change between launches - if the previous port is busy, whitelist-bypass picks a free one. If your Happ tunnel stops working after a restart, delete the `WLB-...` entry and re-tap **Copy v2ray URL** + **Import from clipboard**.

Alternatively, set the SOCKS5 proxy directly in individual apps:
- **Telegram**: Settings -> Data and Storage -> Proxy -> Add proxy -> SOCKS5
- Or tap **Open in Telegram** in whitelist-bypass for automatic setup

### Settings

- **Tunnel** -> **Mode** - VP8 or DataChannel
- **Auth Mode** - Auto (random credentials) or Manual (your own)
- **Display Name** - name shown in the call
- **VP8 Pacing** - FPS and Batch overrides (see VP8 pacing above)
- **Show Logs** - log panel for debugging

> SOCKS5-only is dictated by Apple's rules: NetworkExtension (true VPN) requires a paid Apple Developer account and doesn't work through free sideloading. If someone in the community ships a full VPN build on top of these sources - great.

## Joiner (Desktop: Windows / macOS / Linux)

GUI joiner for the desktop. Unlike the Linux headless joiner below, the desktop joiner can bring up a system VPN tunnel: a TUN adapter plus a split-default route through which all host traffic flows (just like Android VPN mode). Windows uses wintun, Linux uses `/dev/net/tun` + `iproute2`, macOS uses utun + `route(8)`/`ifconfig`. If you don't need TUN, there's a **SOCKS5 only** checkbox that brings up only the local proxy.

![Interface](../res/joiner_desktop_interface.jpeg)

### Download

From [GitHub Releases](https://github.com/kulikov0/whitelist-bypass-iran/releases):

- `WhitelistBypass Joiner-<version>-x64.exe` / `-ia32.exe` - Windows (portable, no installer)
- `WhitelistBypass Joiner-<version>-x86_64.AppImage` - Linux
- `WhitelistBypass Joiner-<version>-arm64.dmg` / `-x64.dmg` - macOS

### Running

1. Paste the call link (`https://meet.bale.ai/i/<code>`)
2. Optionally change: display name, SOCKS5 port, SOCKS5 user/password, tunnel mode (**Video (VP8)** or **DataChannel**), VP8 FPS / batch, DNS for the adapter, resources mode
3. Tick or untick **SOCKS5 only (no system-wide routing)**:
   - Off (default) - TUN is brought up, all traffic goes through the call (needs root/admin)
   - On - local SOCKS5 only, like on iOS
4. Click **Start**
5. Wait for `TUNNEL CONNECTED` in the log - after that SOCKS5 is live on `127.0.0.1:<port>`, and with TUN enabled all host traffic flows through the call

### Privileges

| OS | Required | How to get it |
|---|---|---|
| Windows | Administrator | The `.exe` manifest asks for UAC automatically; accept the prompt |
| Linux | root for TUN mode | Run `xhost +SI:localuser:root`, then `sudo -E ./WhitelistBypass\ Joiner-*.AppImage --no-sandbox`. Without root, only **SOCKS5 only** mode works |
| macOS | root for TUN mode | `sudo "/Applications/WhitelistBypass Joiner.app/Contents/MacOS/WhitelistBypass Joiner"`. Without sudo, only **SOCKS5 only** mode works |

### When to pick desktop over Android/iOS

- You need a system VPN on a laptop or PC
- You want a GUI and the headless CLI below is inconvenient
- You're on a desktop OS without a working Android/iOS device

## Joiner (Linux, headless)

Headless joiner for Linux servers and desktops. Brings up a local SOCKS5 proxy. Point any SOCKS5-capable client at it (`curl --socks5`, Telegram, or system-wide via `redsocks` / `tun2socks`).

Build from source (no prebuilt is shipped today - see README's `./build-headless.sh`).

### Running

```sh
./headless-bale-joiner --join-link https://meet.bale.ai/i/<code> --socks-port 1080
```

After `TUNNEL CONNECTED` the SOCKS5 listener is live on `127.0.0.1:<socks-port>`. Quick check:

```sh
curl --socks5 127.0.0.1:1080 https://api.ipify.org
```

### Flags

| Flag | Description |
|---|---|
| `--join-link <link>` | `https://meet.bale.ai/i/<code>` (required) |
| `--name <str>` | Display name in the call (default `Joiner`) |
| `--socks-port <port>` | SOCKS5 port (default `1080`) |
| `--socks-user <user>` | SOCKS5 username (optional) |
| `--socks-pass <pass>` | SOCKS5 password (optional) |
| `--resources <mode>` | `default` / `moderate` / `unlimited` |
| `--tunnel-mode <mode>` | `vp8` or `dc` (default `vp8`) |
| `--vp8-fps <fps>` | VP8 frame rate (default `24`) |
| `--vp8-batch <n>` | VP8 batch multiplier (default `30`) |

If `--socks-user`/`--socks-pass` are set, SOCKS5 requires auth. Without them the proxy is open on `127.0.0.1`.

### System-wide tunnel (Android-VPN-style)

To force all host traffic through the proxy, run `tun2socks` or `redsocks` on top of the local SOCKS5. Example with `tun2socks`:

```sh
sudo tun2socks -device tun://bale0 -proxy socks5://127.0.0.1:1080
```
