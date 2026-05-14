// Command desktop-joiner is the engine behind the desktop joiner GUI
// for Bale Meet. On Windows it brings up a wintun adapter so every IP
// packet on the host is steered through the resulting SOCKS5 proxy.
// On Linux/macOS it does the same via a native TUN device. Pass
// --no-tun to expose only the SOCKS5 proxy.
//
// On Windows it must run with administrator rights (the embedded
// manifest asks for them); creating wintun adapters and editing the
// route table both require elevation.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/pion/webrtc/v4"
	"whitelist-bypass-iran/relay/common"
	"whitelist-bypass-iran/relay/desktoptun"
	joiner "whitelist-bypass-iran/relay/pion/headless-joiner-common"
	"whitelist-bypass-iran/relay/tunnel"
)

type statusEmitter struct{}

var tunnelLostCh = make(chan struct{}, 1)

func (statusEmitter) EmitStatus(status string) {
	log.Printf("[status] %s", status)
	if status == common.StatusTunnelLost {
		select {
		case tunnelLostCh <- struct{}{}:
		default:
		}
	}
}

func (statusEmitter) EmitStatusError(msg string) {
	log.Printf("[status] ERROR: %s", msg)
	select {
	case tunnelLostCh <- struct{}{}:
	default:
	}
}

type noopPCConfigurer struct{}

func (noopPCConfigurer) ConfigureSettingEngine(*webrtc.SettingEngine) {}

const (
	tunAdapter = "WhitelistBypass"
	tunIP      = "10.99.0.2"
	tunMask    = "255.255.255.0"
	tunPeer    = "10.99.0.1"
	tunMTU     = 1500
)

func main() {
	link := flag.String("link", "", "Bale Meet join link (https://meet.bale.ai/i/<code>) (required)")
	displayName := flag.String("name", "Joiner", "display name in the call")
	socksPort := flag.Int("socks-port", 1080, "local SOCKS5 port")
	socksUser := flag.String("socks-user", "", "optional SOCKS5 username")
	socksPass := flag.String("socks-pass", "", "optional SOCKS5 password")
	resources := flag.String("resources", "default", "moderate | default | unlimited")
	tunnelMode := flag.String("tunnel-mode", "vp8", "tunnel mode: vp8 or dc")
	vp8FPS := flag.Int("vp8-fps", 24, "VP8 frame rate")
	vp8Batch := flag.Int("vp8-batch", 30, "VP8 batch multiplier")
	dns := flag.String("dns", "1.1.1.1,8.8.8.8", "comma-separated DNS servers for the tunnel adapter")
	noTun := flag.Bool("no-tun", false, "expose SOCKS5 only, do not bring up the TUN adapter")
	flag.Parse()

	if *link == "" {
		log.Fatal("--link is required")
	}

	switch *resources {
	case "moderate":
		debug.SetMemoryLimit(64 << 20)
	case "default":
		debug.SetMemoryLimit(128 << 20)
	case "unlimited":
		debug.SetMemoryLimit(256 << 20)
	default:
		log.Fatalf("[config] unknown resources mode: %s", *resources)
	}
	common.MaskingEnabled = true

	var tun *desktoptun.Tunnel
	if !*noTun {
		cfg := desktoptun.Config{
			AdapterName: tunAdapter,
			TunnelIP:    tunIP,
			TunnelMask:  tunMask,
			TunnelPeer:  tunPeer,
			MTU:         tunMTU,
			DNSServers:  splitCSV(*dns),
			SocksHost:   "127.0.0.1",
			SocksPort:   *socksPort,
			SocksUser:   *socksUser,
			SocksPass:   *socksPass,
			LogFn:       log.Printf,
		}
		var err error
		tun, err = desktoptun.New(cfg)
		if err != nil {
			log.Fatalf("[desktoptun] init: %v", err)
		}
	}

	bypassHosts := signalingHosts(*link)
	preResolved := map[string][]net.IP{}
	for _, h := range bypassHosts {
		ips, err := net.LookupIP(h)
		if err != nil {
			log.Printf("[bypass] resolve %s: %v (will rely on candidate hook)", h, err)
			continue
		}
		preResolved[h] = ips
		log.Printf("[bypass] %s -> %v (pre-tun)", h, ips)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	watchStdinQuit(sig)

	tunReady := make(chan struct{})
	var tunOnce sync.Once
	var (
		pendingMu  sync.Mutex
		pending    []string
		tunStarted bool
	)
	bringUpTun := func() {
		tunOnce.Do(func() {
			if tun == nil {
				close(tunReady)
				return
			}
			if err := tun.Start(); err != nil {
				log.Fatalf("[desktoptun] start: %v", err)
			}
			for host, ips := range preResolved {
				for _, ip := range ips {
					if err := tun.AddBypassIP(ip); err != nil {
						log.Printf("[bypass] %s ip %s: %v", host, ip, err)
					}
				}
			}
			pendingMu.Lock()
			drained := pending
			pending = nil
			tunStarted = true
			pendingMu.Unlock()
			for _, c := range drained {
				if err := tun.AddBypassFromCandidate(c); err != nil {
					log.Printf("[bypass] replay: %v", err)
				}
			}
			fmt.Printf("\n  TUNNEL ACTIVE on adapter %q (DNS=%s)\n  all traffic now egresses via Bale Meet\n\n",
				tunAdapter, *dns)
			close(tunReady)
		})
	}

	tryBypass := func(c string) {
		if err := tun.AddBypassFromCandidate(c); err != nil {
			pendingMu.Lock()
			if !tunStarted {
				pending = append(pending, c)
				pendingMu.Unlock()
				return
			}
			pendingMu.Unlock()
			log.Printf("[bypass] candidate: %v", err)
		}
	}

	addCandidate := func(_ int, candidateOrSDP string) {
		if tun == nil {
			return
		}
		tryBypass(candidateOrSDP)
		if strings.Contains(candidateOrSDP, "a=candidate:") {
			for _, line := range strings.Split(candidateOrSDP, "\n") {
				line = strings.TrimRight(line, "\r")
				if strings.HasPrefix(line, "a=candidate:") {
					tryBypass(line)
				}
			}
		}
	}

	c := joiner.NewBaleHeadlessJoiner(log.Printf, resolveHostname, statusEmitter{}, noopPCConfigurer{})
	c.OnConnected = func(t tunnel.DataTunnel) {
		readBuf := common.VP8BufSize
		if _, ok := t.(*tunnel.DCTunnel); ok {
			readBuf = common.DCSocksReadBuf
		}
		bridge := tunnel.NewRelayBridgeWithAuth(t, "joiner", readBuf, log.Printf, *socksUser, *socksPass)
		bridge.MarkReady()
		addr := fmt.Sprintf("127.0.0.1:%d", *socksPort)
		go func() {
			if err := bridge.ListenSOCKS(addr); err != nil {
				log.Printf("[socks] listen: %v", err)
			}
		}()
		log.Printf("[socks] listening on %s", addr)
		bringUpTun()
	}
	c.OnRemoteCandidate = addCandidate

	params, _ := json.Marshal(struct {
		JoinLink    string `json:"joinLink"`
		DisplayName string `json:"displayName"`
		Resources   string `json:"resources"`
		VP8FPS      int    `json:"vp8Fps"`
		VP8Batch    int    `json:"vp8Batch"`
		TunnelMode  string `json:"tunnelMode"`
	}{
		JoinLink:    strings.TrimSpace(*link),
		DisplayName: *displayName,
		Resources:   *resources,
		VP8FPS:      *vp8FPS,
		VP8Batch:    *vp8Batch,
		TunnelMode:  *tunnelMode,
	})

	done := make(chan struct{})
	go func() {
		c.RunWithParams(string(params))
		close(done)
	}()

	var lost bool
	select {
	case <-sig:
		log.Printf("[main] shutting down")
	case <-tunnelLostCh:
		log.Printf("[main] tunnel lost, exiting with code 2 to trigger auto-reconnect")
		lost = true
	case <-done:
		log.Printf("[main] joiner done")
	}
	c.Close()
	if tun != nil {
		tun.Stop()
	}
	time.Sleep(200 * time.Millisecond)
	if lost {
		os.Exit(2)
	}
}

func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func signalingHosts(link string) []string {
	hosts := []string{"meet.bale.ai", "tapi.bale.ai"}
	if u, err := url.Parse(strings.TrimSpace(link)); err == nil && u.Host != "" {
		hosts = append(hosts, u.Host)
	}
	return hosts
}

func resolveHostname(host string) (string, error) {
	if ip := net.ParseIP(host); ip != nil {
		return host, nil
	}
	ips, err := net.DefaultResolver.LookupIP(context.Background(), "ip4", host)
	if err != nil {
		return "", err
	}
	if len(ips) == 0 {
		return "", fmt.Errorf("no IPv4 for %s", host)
	}
	return ips[0].String(), nil
}
