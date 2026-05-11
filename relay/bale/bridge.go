package bale

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"whitelist-bypass-iran/relay/common"
)

const (
	MkprotoVersion    = 1
	defaultRPCTimeout = 10 * time.Second
	pingPeriod        = 10 * time.Second
)

type DialContextFunc func(ctx context.Context, network, addr string) (net.Conn, error)

type BridgeConfig struct {
	LogFn       func(string, ...any)
	DialContext DialContextFunc
	RPCTimeout  time.Duration
}

type Bridge struct {
	cfg BridgeConfig

	mu      sync.Mutex
	ws      *websocket.Conn
	rpcSeq  int64
	pingSeq int64
	pending map[int64]chan *Response
	helloCh chan struct{}
}

func NewBridge(cfg BridgeConfig) *Bridge {
	if cfg.LogFn == nil {
		cfg.LogFn = func(string, ...any) {}
	}
	if cfg.RPCTimeout == 0 {
		cfg.RPCTimeout = defaultRPCTimeout
	}
	return &Bridge{
		cfg:     cfg,
		pending: make(map[int64]chan *Response),
		helloCh: make(chan struct{}, 1),
	}
}

func (b *Bridge) Dial(wsURL string, header http.Header) error {
	dialer := *websocket.DefaultDialer
	if b.cfg.DialContext != nil {
		dialer.NetDialContext = b.cfg.DialContext
	}
	b.cfg.LogFn("[bale-ws] connecting %s", wsURL)
	ws, resp, err := dialer.Dial(wsURL, header)
	if err != nil {
		if resp != nil {
			return fmt.Errorf("ws dial: %w (status %d)", err, resp.StatusCode)
		}
		return fmt.Errorf("ws dial: %w", err)
	}
	b.mu.Lock()
	b.ws = ws
	b.mu.Unlock()
	b.cfg.LogFn("[bale-ws] connected")
	return nil
}

func (b *Bridge) Close() {
	b.mu.Lock()
	ws := b.ws
	b.ws = nil
	b.mu.Unlock()
	if ws != nil {
		_ = ws.Close()
	}
}

func (b *Bridge) wsSend(req, ping, hs []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.ws == nil {
		return
	}
	b.ws.WriteMessage(websocket.BinaryMessage, EncodeClientPack(req, ping, hs))
}

func (b *Bridge) SendHandshake(apiVersion int64) {
	b.cfg.LogFn("[bale-ws] -> handshake")
	b.wsSend(nil, nil, EncodeHandshakeRequest(MkprotoVersion, apiVersion))
}

func (b *Bridge) sendPing() {
	b.mu.Lock()
	b.pingSeq++
	id := b.pingSeq
	b.mu.Unlock()
	b.wsSend(nil, EncodePing(id), nil)
}

func (b *Bridge) Unary(serviceName, method string, payload []byte) (*Response, error) {
	b.mu.Lock()
	b.rpcSeq++
	idx := b.rpcSeq
	ch := make(chan *Response, 1)
	b.pending[idx] = ch
	b.mu.Unlock()
	b.cfg.LogFn("[bale-ws] -> %s/%s #%d", serviceName, method, idx)
	b.wsSend(EncodeRequest(serviceName, method, payload, idx), nil, nil)
	select {
	case resp := <-ch:
		b.cfg.LogFn("[bale-ws] <- %s/%s #%d resp=%dB err=%dB", serviceName, method, idx, len(resp.Response), len(resp.Error))
		if len(resp.Error) > 0 {
			return resp, fmt.Errorf("rpc error %s/%s err=%q", serviceName, method, string(resp.Error))
		}
		return resp, nil
	case <-time.After(b.cfg.RPCTimeout):
		b.mu.Lock()
		delete(b.pending, idx)
		b.mu.Unlock()
		return nil, fmt.Errorf("rpc timeout %s/%s", serviceName, method)
	}
}

func (b *Bridge) handleMessage(raw []byte) {
	pack, err := DecodeServerPack(raw)
	if err != nil {
		b.cfg.LogFn("[bale-ws] decode: %v", err)
		return
	}
	if hs := pack.HandshakeResponse; hs != nil {
		b.cfg.LogFn("[bale-ws] <- handshake mkproto=%d apiVersion=%d", hs.MkprotoVersion, hs.APIVersion)
		select {
		case b.helloCh <- struct{}{}:
		default:
		}
		return
	}
	if pack.TerminateSession {
		b.cfg.LogFn("[bale-ws] <- terminateSession")
		return
	}
	if pack.Pong != nil {
		return
	}
	if r := pack.Response; r != nil {
		b.mu.Lock()
		ch := b.pending[r.Index]
		delete(b.pending, r.Index)
		b.mu.Unlock()
		if ch != nil {
			ch <- r
		}
	}
}

func (b *Bridge) Hello() <-chan struct{} { return b.helloCh }

// Run starts the read loop and ping loop. Returns when the WS read ends.
// Call after Dial.
func (b *Bridge) Run() {
	b.mu.Lock()
	ws := b.ws
	b.mu.Unlock()
	if ws == nil {
		return
	}

	stopPing := make(chan struct{})
	go func() {
		ticker := time.NewTicker(pingPeriod)
		defer ticker.Stop()
		for {
			select {
			case <-stopPing:
				return
			case <-ticker.C:
				b.sendPing()
			}
		}
	}()

	for {
		_, raw, err := ws.ReadMessage()
		if err != nil {
			b.cfg.LogFn("[bale-ws] closed: %s", common.MaskError(err))
			close(stopPing)
			return
		}
		b.handleMessage(raw)
	}
}
