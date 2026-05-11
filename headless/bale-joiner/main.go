package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pion/webrtc/v4"
	"whitelist-bypass-iran/relay/common"
	joiner "whitelist-bypass-iran/relay/pion/headless-joiner-common"
	"whitelist-bypass-iran/relay/tunnel"
)

type stdoutStatus struct{}

func (stdoutStatus) EmitStatus(status string)   { fmt.Printf("STATUS:%s\n", status) }
func (stdoutStatus) EmitStatusError(msg string) { fmt.Printf("STATUS:ERROR:%s\n", msg) }

type noopPCConfigurer struct{}

func (noopPCConfigurer) ConfigureSettingEngine(*webrtc.SettingEngine) {}

func desktopResolve(host string) (string, error) {
	ips, err := net.DefaultResolver.LookupIP(context.Background(), "ip4", host)
	if err != nil {
		return "", err
	}
	if len(ips) == 0 {
		return "", fmt.Errorf("no ipv4 for %s", host)
	}
	return ips[0].String(), nil
}

func main() {
	joinLink := flag.String("join-link", "", "https://meet.bale.ai/i/<code> (required)")
	displayName := flag.String("name", "Joiner", "display name in the call")
	socksPort := flag.Int("socks-port", 1080, "SOCKS5 listen port")
	socksUser := flag.String("socks-user", "", "SOCKS5 username (optional)")
	socksPass := flag.String("socks-pass", "", "SOCKS5 password (optional)")
	resources := flag.String("resources", "default", "resource mode: moderate, default, unlimited")
	vp8FPS := flag.Int("vp8-fps", 24, "VP8 frame rate")
	vp8Batch := flag.Int("vp8-batch", 30, "VP8 batch multiplier")
	flag.Parse()

	if *joinLink == "" {
		log.Fatal("--join-link is required")
	}

	c := joiner.NewBaleHeadlessJoiner(log.Printf, desktopResolve, stdoutStatus{}, noopPCConfigurer{})

	c.OnConnected = func(tun tunnel.DataTunnel) {
		bridge := tunnel.NewRelayBridgeWithAuth(tun, "joiner", common.VP8BufSize, log.Printf, *socksUser, *socksPass)
		bridge.MarkReady()
		addr := fmt.Sprintf("127.0.0.1:%d", *socksPort)
		go func() {
			if err := bridge.ListenSOCKS(addr); err != nil {
				log.Printf("socks listen: %v", err)
			}
		}()
		fmt.Printf("\n  TUNNEL CONNECTED\n  socks5 -> %s\n\n", addr)
	}

	params := struct {
		JoinLink    string `json:"joinLink"`
		DisplayName string `json:"displayName"`
		Resources   string `json:"resources"`
		VP8FPS      int    `json:"vp8Fps"`
		VP8Batch    int    `json:"vp8Batch"`
	}{
		JoinLink:    *joinLink,
		DisplayName: *displayName,
		Resources:   *resources,
		VP8FPS:      *vp8FPS,
		VP8Batch:    *vp8Batch,
	}
	js, _ := json.Marshal(params)

	done := make(chan struct{})
	go func() {
		c.RunWithParams(string(js))
		close(done)
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	select {
	case <-sig:
		log.Printf("[main] signal received, shutting down")
		c.Close()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
		}
	case <-done:
	}
}
