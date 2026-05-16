# Running the Creator on a VPS

> فارسی: [docs/fa/VPS.md](../fa/VPS.md)

This guide covers two ways to run the Creator on a Linux VPS without a local display.

- **Headless (recommended)** - pure-Go binary, no Electron, no display server. The simplest path; what production deployments should use.
- **GUI over XPRA** - run the regular Electron Creator on the server and forward its window to your laptop via XPRA. Use this only if you specifically need the desktop UI on the VPS (e.g. to debug Bale sign-in interactively).

## Requirements

Tested on:
- 1 vCPU
- 1 GB RAM
- Ubuntu 22.04 / Debian 12

## Contents

- [Option A - Headless Creator](#option-a--headless-creator)
- [Option B - GUI over XPRA](#option-b--gui-over-xpra)

## Option A - Headless Creator

Pure-Go binary. No browser, no Electron, no graphical environment needed.

### 1. Prepare cookies

The headless Creator authenticates with cookies exported from a desktop Creator instance:

1. On a desktop machine (Windows / macOS / Linux), download the Creator app from [Releases](https://github.com/kulikov0/whitelist-bypass-iran/releases) and sign in to Bale once.
2. Click **Bale Cookies** to export `bale-cookies.json`.
3. Copy the file to the VPS, e.g. `/etc/whitelist-bypass/bale-cookies.json`.

### 2. Download the binary

From [GitHub Releases](https://github.com/kulikov0/whitelist-bypass-iran/releases), download the matching `headless-bale-creator-linux-*` binary for your server architecture:

```sh
# Example: x64 server
wget -O /usr/local/bin/headless-bale-creator \
  https://github.com/kulikov0/whitelist-bypass-iran/releases/latest/download/headless-bale-creator-linux-x64
sudo chmod +x /usr/local/bin/headless-bale-creator
```

### 3. Run

```sh
/usr/local/bin/headless-bale-creator \
  --cookies /etc/whitelist-bypass/bale-cookies.json \
  --write-file /var/run/whitelist-bypass/call.txt
```

On startup the binary prints something like:

```
CALL CREATED
  join_link: https://meet.bale.ai/i/<code>
  protocol:  api 1 mkproto 1
```

Send the `join_link` to the Joiner. The same link is also appended to `--write-file`, one line per call, which is handy for tooling.

### 4. systemd service (autostart)

`/etc/systemd/system/wlb-bale-creator.service`:

```ini
[Unit]
Description=Whitelist Bypass Bale Creator (headless)
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/headless-bale-creator \
  --cookies /etc/whitelist-bypass/bale-cookies.json \
  --write-file /var/run/whitelist-bypass/call.txt \
  --resources default
Restart=always
RestartSec=5
User=wlb
RuntimeDirectory=whitelist-bypass

[Install]
WantedBy=multi-user.target
```

Enable and watch the log:

```sh
sudo useradd -r -s /usr/sbin/nologin wlb 2>/dev/null || true
sudo systemctl daemon-reload
sudo systemctl enable --now wlb-bale-creator
sudo journalctl -u wlb-bale-creator -f
```

The active join link will be at `/var/run/whitelist-bypass/call.txt` whenever the service is running. If Bale cookies expire, the service restarts in `WAITING_FOR_COOKIES` state - re-export `bale-cookies.json` from the desktop Creator and replace the file on the server.

### 5. Resource modes

Match the binary's `--resources` flag to the VPS size:

| VPS RAM | Recommended mode |
|---|---|
| <= 512 MB | `moderate` |
| 1 GB | `default` |
| >= 2 GB or dedicated host | `unlimited` |

See the [main SETUP guide](SETUP.md#resource-modes) for what each mode actually changes.

## Option B - GUI over XPRA

Use this only when you specifically need the desktop Creator UI on the server. For everything else, Option A is simpler.

### 1. Install XPRA

XPRA forwards a single application window from the VPS to your laptop's browser. See [XPRA download instructions](https://github.com/Xpra-org/xpra/wiki/Download#-linux), or the one-liner:

```sh
curl https://xpra.org/get-xpra.sh | bash
```

### 2. Find the Creator AppImage URL

- Go to the [Releases page](https://github.com/kulikov0/whitelist-bypass-iran/releases) and pick the latest tag (marked `latest`).
- Expand **Assets** and find the file ending in `.AppImage` - that's the Linux Creator build.

  ![Release assets](../res/vps/open-latest-release-assets.jpg)

- Right-click the `.AppImage` link and choose **Copy link address**.

  ![Copy direct link](../res/vps/copy-latest-creator-direct-link.jpg)

### 3. Install AppImage dependencies

```sh
sudo add-apt-repository universe
sudo apt install libfuse2
```

### 4. Download and install the Creator

```sh
wget https://github.com/kulikov0/whitelist-bypass-iran/releases/download/v0.1.0/Whitelist.Bypass.Creator.Iran-0.1.0.AppImage
mv *.AppImage creator.AppImage
chmod +x creator.AppImage
sudo mv creator.AppImage /usr/bin/whitelist-bypass-creator
```

### 5. Helper scripts

Stop script:

```sh
sudo tee /usr/bin/whitelist-bypass-stop > /dev/null << 'EOF'
#!/usr/bin/env bash
xpra stop 100
EOF
sudo chmod +x /usr/bin/whitelist-bypass-stop
```

Start script:

```sh
sudo tee /usr/bin/whitelist-bypass-start > /dev/null << 'EOF'
#!/usr/bin/env bash
xpra start :100 --pulseaudio=no --webcam=no --mdns=no --resize-display=1200x900 \
  --attach=yes --daemon=no --html=on --bind-tcp=127.0.0.1:10000 \
  --start='xterm -e whitelist-bypass-creator --no-sandbox'
EOF
sudo chmod +x /usr/bin/whitelist-bypass-start
```

### 6. systemd autostart

> Skip this if you don't need the Creator to auto-launch after VPS reboots.

```sh
sudo tee /etc/systemd/system/whitelist-bypass-start.service > /dev/null << 'EOF'
[Unit]
Description=Whitelist Bypass Creator Service
Documentation=https://github.com/kulikov0/whitelist-bypass-iran/
After=xpra-server.service

[Service]
Type=simple
ExecStart=bash /usr/bin/whitelist-bypass-start

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl daemon-reload
sudo systemctl enable whitelist-bypass-start.service
sudo systemctl start whitelist-bypass-start.service
sudo reboot
```

> VPS reboots can take a while - be patient.

### 7. Connecting from your laptop

> If you skipped the autostart step, SSH in and run `whitelist-bypass-start` first.

Forward the XPRA port over SSH:

```sh
ssh vps-username@vps-ip -NL 10000:localhost:10000
```

Keep this terminal open for the duration of your Creator session.

Open `http://localhost:10000/connect.html` in any browser.

In the connection dialog, open **Advanced Options**:

![Advanced options](../res/vps/open-xpra-advanced-settings.jpg)

Set **Keyboard Layout** to **English USA**:

![Keyboard layout](../res/vps/choose-xpra-keyboard-layout.jpg)

Click the green **Connect** button. Once connected, the Creator window appears inside the browser - drive it just like a normal desktop Creator.
