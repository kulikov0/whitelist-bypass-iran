package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"whitelist-bypass-iran/relay/common"
	"whitelist-bypass-iran/relay/pion/android"
	"whitelist-bypass-iran/relay/tunnel"
)

func main() {
	mode := flag.String("mode", "", "joiner mode (bale-headless-joiner)")
	socksPort := flag.Int("socks-port", 1080, "SOCKS5 proxy port")
	socksUser := flag.String("socks-user", "", "SOCKS5 proxy username")
	socksPass := flag.String("socks-pass", "", "SOCKS5 proxy password")
	flag.Parse()

	if *mode == "" {
		fmt.Fprintf(os.Stderr, "Usage: relay --mode bale-headless-joiner [--socks-port N] [--socks-user U] [--socks-pass P]\n")
		os.Exit(1)
	}

	startJoinerBridge := func(tun tunnel.DataTunnel, readBuf int) {
		rb := tunnel.NewRelayBridgeWithAuth(tun, "joiner", readBuf, log.Printf, *socksUser, *socksPass)
		rb.MarkReady()
		addr := fmt.Sprintf("127.0.0.1:%d", *socksPort)
		go func() {
			if err := rb.ListenSOCKS(addr); err != nil {
				log.Printf("socks listen: %v", err)
			}
		}()
		fmt.Printf("\n  TUNNEL CONNECTED\n  socks5 -> %s\n\n", addr)
	}

	switch *mode {
	case "bale-headless-joiner":
		c := android.NewBaleHeadlessJoiner(log.Printf)
		c.OnConnected = func(tun tunnel.DataTunnel) {
			readBuf := common.VP8BufSize
			if _, ok := tun.(*tunnel.DCTunnel); ok {
				readBuf = common.DCSocksReadBuf
			}
			startJoinerBridge(tun, readBuf)
		}
		c.Run()
	default:
		fmt.Fprintf(os.Stderr, "Unknown mode: %s\n", *mode)
		os.Exit(1)
	}
}
