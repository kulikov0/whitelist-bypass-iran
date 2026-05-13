package joiner

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"net/url"
	"runtime/debug"
	"sync"
	"time"

	"github.com/pion/webrtc/v4"
	"whitelist-bypass-iran/relay/bale"
	"whitelist-bypass-iran/relay/common"
	"whitelist-bypass-iran/relay/tunnel"
)

const baleOrigin = "https://meet.bale.ai"

type BaleHeadlessJoiner struct {
	logFn       func(string, ...any)
	OnConnected func(tunnel.DataTunnel)
	ResolveFn   ResolveFunc
	Status      StatusEmitter
	PCConfig    PeerConnectionConfigurer

	mu      sync.Mutex
	session *bale.Session
	closed  bool
}

func NewBaleHeadlessJoiner(logFn func(string, ...any), resolveFn ResolveFunc, status StatusEmitter, pcConfig PeerConnectionConfigurer) *BaleHeadlessJoiner {
	if logFn == nil {
		logFn = log.Printf
	}
	return &BaleHeadlessJoiner{
		logFn:     logFn,
		ResolveFn: resolveFn,
		Status:    status,
		PCConfig:  pcConfig,
	}
}

func (j *BaleHeadlessJoiner) RunWithParams(jsonParams string) {
	var params struct {
		JoinLink    string `json:"joinLink"`
		DisplayName string `json:"displayName"`
		Resources   string `json:"resources"`
		VP8FPS      int    `json:"vp8Fps"`
		VP8Batch    int    `json:"vp8Batch"`
		TunnelMode  string `json:"tunnelMode"`
	}
	if err := json.Unmarshal([]byte(jsonParams), &params); err != nil {
		j.logFn("bale-joiner: failed to parse params: %v", err)
		j.Status.EmitStatusError("bad params: " + err.Error())
		return
	}
	if params.JoinLink == "" {
		j.logFn("bale-joiner: missing joinLink")
		j.Status.EmitStatusError("missing joinLink")
		return
	}
	if params.DisplayName == "" {
		params.DisplayName = "Joiner"
	}
	if params.VP8FPS == 0 {
		params.VP8FPS = 24
	}
	if params.VP8Batch == 0 {
		params.VP8Batch = 30
	}
	switch params.TunnelMode {
	case "", bale.ModeVP8:
		params.TunnelMode = bale.ModeVP8
	case bale.ModeDC:
	default:
		j.logFn("bale-joiner: unknown tunnelMode %q, falling back to vp8", params.TunnelMode)
		params.TunnelMode = bale.ModeVP8
	}

	var memLimit int64
	switch params.Resources {
	case "moderate":
		memLimit = 64 << 20
	case "", "default":
		memLimit = 128 << 20
	case "unlimited":
		memLimit = 256 << 20
	default:
		j.logFn("bale-joiner: unknown resources mode %q, using default", params.Resources)
		memLimit = 128 << 20
	}
	if memLimit > 0 {
		debug.SetMemoryLimit(memLimit)
	}
	common.MaskingEnabled = true
	j.logFn("[config] resources=%s mem-limit=%d vp8-fps=%d vp8-batch=%d tunnel-mode=%s", params.Resources, memLimit, params.VP8FPS, params.VP8Batch, params.TunnelMode)
	j.Status.EmitStatus(common.StatusConnecting)

	code := bale.ExtractShareCode(params.JoinLink)
	if code == "" {
		j.logFn("[config] could not extract code from %q", params.JoinLink)
		j.Status.EmitStatusError("bad join link")
		return
	}
	j.logFn("[config] share-code=%s", code)

	dialCtx := j.makeDialContext()
	httpClient := &http.Client{Timeout: 60 * time.Second, Transport: &http.Transport{DialContext: dialCtx}}
	cfg, err := baleFetchAnonConfig(httpClient, j.logFn)
	if err != nil {
		j.logFn("[config] %v", err)
		j.Status.EmitStatusError("config: " + err.Error())
		return
	}

	bridge := bale.NewBridge(bale.BridgeConfig{
		LogFn:       j.logFn,
		DialContext: dialCtx,
		RPCTimeout:  15 * time.Second,
	})
	wsURL := cfg.WSURL + "?token=" + url.QueryEscape(cfg.Token)
	header := http.Header{}
	header.Set("User-Agent", common.UserAgent)
	header.Set("Origin", baleOrigin)
	if err := bridge.Dial(wsURL, header); err != nil {
		j.logFn("[bale-ws] %v", err)
		j.Status.EmitStatusError("ws: " + err.Error())
		return
	}
	defer bridge.Close()

	go bridge.Run()

	resp, err := bridge.Unary("bale.meet.v1.Meet", "GetCallLinkDetails", bale.EncodeGetCallLinkDetailsRequest(code))
	if err != nil {
		j.logFn("[auth] GetCallLinkDetails: %v", err)
		j.Status.EmitStatusError("auth: " + err.Error())
		return
	}
	details, err := bale.DecodeCallEnvelope(resp.Response)
	if err != nil {
		j.logFn("[auth] decode call: %v", err)
		j.Status.EmitStatusError("auth decode")
		return
	}
	if details.ID == 0 {
		j.logFn("[auth] GetCallLinkDetails returned no callId")
		j.Status.EmitStatusError("no callId")
		return
	}
	j.logFn("[auth] resolved code=%s -> callId=%d", code, details.ID)

	resp, err = bridge.Unary("bale.meet.v1.Meet", "JoinGroupCall", bale.EncodeJoinGroupCallRequest(details.ID, params.DisplayName))
	if err != nil {
		j.logFn("[auth] JoinGroupCall: %v", err)
		j.Status.EmitStatusError("join: " + err.Error())
		return
	}
	joined, err := bale.DecodeCallEnvelope(resp.Response)
	if err != nil {
		j.logFn("[auth] decode join: %v", err)
		j.Status.EmitStatusError("join decode")
		return
	}
	if joined.URL == "" || joined.LivekitJWT == "" {
		j.logFn("[auth] JoinGroupCall returned empty livekit creds")
		j.Status.EmitStatusError("empty creds")
		return
	}
	j.logFn("[auth] livekit url=%s jwt=%dB room=%s", joined.URL, len(joined.LivekitJWT), joined.Token)

	obf, err := tunnel.NewTunnelObfuscator(tunnel.DeriveSecretFromJoinLink(params.JoinLink))
	if err != nil {
		j.logFn("[obf] init failed: %v", err)
		j.Status.EmitStatusError("obf: " + err.Error())
		return
	}
	j.logFn("[obf] localEpoch=0x%08x", obf.LocalEpoch())

	var settingEngine *webrtc.SettingEngine
	if j.PCConfig != nil {
		se := webrtc.SettingEngine{}
		j.PCConfig.ConfigureSettingEngine(&se)
		settingEngine = &se
	}

	sess := bale.NewSession(bale.SessionConfig{
		Role:           bale.RoleJoiner,
		TunnelMode:     params.TunnelMode,
		WSURL:          joined.URL,
		RoomToken:      joined.LivekitJWT,
		Origin:         baleOrigin,
		Obfuscator:     obf,
		LogFn:          j.logFn,
		VP8FPS:         params.VP8FPS,
		VP8Batch:       params.VP8Batch,
		SettingEngine:  settingEngine,
		NetDialContext: dialCtx,
		ResolveICEHost: j.ResolveFn,
	})
	sess.OnConnected = func(tun tunnel.DataTunnel) {
		j.logFn("bale-joiner: === TUNNEL CONNECTED ===")
		j.Status.EmitStatus(common.StatusTunnelConnected)
		if j.OnConnected != nil {
			j.OnConnected(tun)
		}
	}

	j.mu.Lock()
	j.session = sess
	closed := j.closed
	j.mu.Unlock()
	if closed {
		sess.Close()
		return
	}

	if err := sess.Start(); err != nil {
		j.logFn("[session] start: %v", err)
		j.Status.EmitStatusError("session: " + err.Error())
		return
	}
	<-sess.Done()
	j.logFn("bale-joiner: session ended")
	j.Status.EmitStatus(common.StatusTunnelLost)
}

func (j *BaleHeadlessJoiner) Close() {
	j.mu.Lock()
	j.closed = true
	sess := j.session
	j.session = nil
	j.mu.Unlock()
	if sess != nil {
		sess.Close()
	}
}

func (j *BaleHeadlessJoiner) makeDialContext() func(ctx context.Context, network, addr string) (net.Conn, error) {
	if j.ResolveFn == nil {
		return nil
	}
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}
		if net.ParseIP(host) != nil {
			return (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, network, addr)
		}
		resolvedIP, err := j.ResolveFn(host)
		if err != nil {
			return nil, err
		}
		return (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, network, resolvedIP+":"+port)
	}
}
