package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"whitelist-bypass-iran/relay/bale"
	"whitelist-bypass-iran/relay/common"
	"whitelist-bypass-iran/relay/tunnel"
)

const creatorOrigin = "https://web.bale.ai"

type creator struct {
	cookieStr string
	config    BaleConfig
	vp8FPS    int
	vp8Batch  int

	bridge *bale.Bridge

	mu   sync.Mutex
	link string
}

func (c *creator) run() {
	c.bridge = bale.NewBridge(bale.BridgeConfig{LogFn: log.Printf})

	header := http.Header{}
	header.Set("User-Agent", common.UserAgent)
	header.Set("Origin", creatorOrigin)
	header.Set("Cookie", c.cookieStr)

	if err := c.bridge.Dial(c.config.WSURL, header); err != nil {
		log.Fatalf("[bale-ws] %s", common.MaskError(err))
	}
	defer c.bridge.Close()

	go c.bridge.Run()

	c.bridge.SendHandshake(c.config.APIVersion)
	<-c.bridge.Hello()

	resp, err := c.bridge.Unary("bale.meet.v1.Meet", "GenerateCallLink", bale.EncodeGenerateCallLinkRequest(true))
	if err != nil {
		log.Fatalf("[auth] %v", err)
	}
	call, err := bale.DecodeCallEnvelope(resp.Response)
	if err != nil {
		log.Fatalf("[auth] decode: %v", err)
	}
	c.mu.Lock()
	c.link = call.ShareLink
	c.mu.Unlock()
	log.Printf("[auth] call_id=%d token=%s", call.ID, call.Token)

	resp, err = c.bridge.Unary("bale.meet.v1.Meet", "JoinGroupCall", bale.EncodeJoinGroupCallRequest(call.ID, ""))
	if err != nil {
		log.Fatalf("[auth] JoinGroupCall: %v", err)
	}
	joined, err := bale.DecodeCallEnvelope(resp.Response)
	if err != nil {
		log.Fatalf("[auth] decode join: %v", err)
	}
	if joined.URL == "" || joined.LivekitJWT == "" {
		log.Fatalf("[auth] JoinGroupCall returned empty livekit creds: url=%q jwt=%dB", joined.URL, len(joined.LivekitJWT))
	}
	log.Printf("[auth] livekit url=%s jwt=%dB room=%s", joined.URL, len(joined.LivekitJWT), joined.Token)

	fmt.Println("")
	fmt.Println("  CALL CREATED")
	fmt.Println("  join_link:", call.ShareLink)
	fmt.Printf("  protocol:  api %d mkproto %d\n\n", c.config.APIVersion, bale.MkprotoVersion)

	obf, err := tunnel.NewTunnelObfuscator(tunnel.DeriveSecretFromJoinLink(call.ShareLink))
	if err != nil {
		log.Fatalf("[obf] init failed: %v", err)
	}
	log.Printf("[obf] localEpoch=0x%08x", obf.LocalEpoch())

	sess := bale.NewSession(bale.SessionConfig{
		WSURL:      joined.URL,
		RoomToken:  joined.LivekitJWT,
		Origin:     "https://meet.bale.ai",
		Obfuscator: obf,
		LogFn:      log.Printf,
		VP8FPS:     c.vp8FPS,
		VP8Batch:   c.vp8Batch,
	})
	sess.OnConnected = func(tun tunnel.DataTunnel) {
		tunnel.NewRelayBridge(tun, "creator", common.VP8BufSize, log.Printf)
		fmt.Print("\n  TUNNEL CONNECTED\n\n")
	}
	if err := sess.Start(); err != nil {
		log.Fatalf("[session] %v", err)
	}

	<-sess.Done()
}

func (c *creator) currentLink() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.link
}

func main() {
	cookiesPath := flag.String("cookies", "", "path to bale-cookies.json")
	cookieString := flag.String("cookie-string", "", "raw cookie string")
	resources := flag.String("resources", "default", "resource mode: default, moderate, unlimited")
	writeFile := flag.String("write-file", "", "path to file where active call link is appended")
	vp8FPS := flag.Int("vp8-fps", 24, "VP8 frame rate")
	vp8Batch := flag.Int("vp8-batch", 30, "VP8 batch multiplier (writer rate = fps * batch fps)")
	flag.Parse()

	var memLimit int64
	switch *resources {
	case "moderate":
		memLimit = 64 << 20
	case "default":
		memLimit = 128 << 20
	case "unlimited":
		memLimit = 256 << 20
	default:
		log.Fatalf("[config] unknown resources mode: %s", *resources)
	}
	if memLimit > 0 {
		debug.SetMemoryLimit(memLimit)
	}
	common.MaskingEnabled = true
	log.Printf("[config] resources=%s mem-limit=%d", *resources, memLimit)

	var cookieStr string
	if *cookieString != "" {
		cookieStr = *cookieString
	} else if *cookiesPath != "" {
		cookieStr = common.LoadCookies(*cookiesPath)
	} else {
		fmt.Println("WAITING_FOR_COOKIES")
		line, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil || strings.TrimSpace(line) == "" {
			log.Fatal("No cookies received on stdin")
		}
		cookieStr = strings.TrimSpace(line)
	}

	log.Println("[config] Fetching live config from Bale bundle...")
	cfg, err := fetchConfig()
	if err != nil {
		log.Fatalf("[config] %v", err)
	}

	c := &creator{
		cookieStr: cookieStr,
		config:    cfg,
		vp8FPS:    *vp8FPS,
		vp8Batch:  *vp8Batch,
	}

	if *writeFile != "" {
		go func() {
			for c.currentLink() == "" {
				time.Sleep(50 * time.Millisecond)
			}
			f, err := os.OpenFile(*writeFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				log.Fatalf("Failed to open write-file: %v", err)
			}
			fmt.Fprintln(f, c.currentLink())
			f.Close()
			log.Printf("[config] Wrote call link to %s", *writeFile)
		}()
	}

	c.run()
}
