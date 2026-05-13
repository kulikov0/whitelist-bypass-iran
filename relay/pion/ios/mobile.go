package ios

import (
	"fmt"
	"net"
	"sync"

	"whitelist-bypass-iran/relay/common"
	joiner "whitelist-bypass-iran/relay/pion/headless-joiner-common"
	"whitelist-bypass-iran/relay/tunnel"

	"github.com/pion/webrtc/v4"
)

type HeadlessCallback interface {
	OnLog(msg string)
	OnStatus(status string)
	ResolveHost(hostname string) string
}

type joinerHandle interface {
	Close()
}

var activeHeadless struct {
	sync.Mutex
	joiner   joinerHandle
	callback HeadlessCallback
	socksLn  net.Listener
	stopped  bool
}

type iosStatusEmitter struct {
	statusFn func(string)
}

func (e *iosStatusEmitter) EmitStatus(status string)   { e.statusFn(status) }
func (e *iosStatusEmitter) EmitStatusError(msg string) { e.statusFn("ERROR:" + msg) }

type iosPCConfigurer struct{}

func (iosPCConfigurer) ConfigureSettingEngine(_ *webrtc.SettingEngine) {}

func makeOnConnected(socksPort int, socksUser, socksPass string, logFn func(string, ...any), callback HeadlessCallback) func(tunnel.DataTunnel) {
	return func(tun tunnel.DataTunnel) {
		activeHeadless.Lock()
		if activeHeadless.stopped {
			activeHeadless.Unlock()
			return
		}
		activeHeadless.Unlock()

		readBuf := common.VP8BufSize
		if _, ok := tun.(*tunnel.DCTunnel); ok {
			readBuf = common.DCSocksReadBuf
		}
		bridge := tunnel.NewRelayBridgeWithAuth(tun, "joiner", readBuf, logFn, socksUser, socksPass)
		bridge.MarkReady()

		socksAddr := fmt.Sprintf("127.0.0.1:%d", socksPort)
		logFn("ios: SOCKS5 proxy starting on %s", socksAddr)
		go func() {
			if err := bridge.ListenSOCKS(socksAddr); err != nil {
				logFn("ios: SOCKS5 listen error: %v", err)
				callback.OnStatus("ERROR:socks listen: " + err.Error())
			}
		}()
	}
}

func makeHelpers(callback HeadlessCallback) (func(string, ...any), joiner.ResolveFunc, *iosStatusEmitter) {
	logFn := func(format string, args ...any) {
		msg := fmt.Sprintf(format, args...)
		callback.OnLog(msg)
	}
	resolveFn := func(hostname string) (string, error) {
		result := callback.ResolveHost(hostname)
		if result == "" {
			return "", fmt.Errorf("empty resolve for %s", hostname)
		}
		return result, nil
	}
	statusEmitter := &iosStatusEmitter{
		statusFn: func(status string) {
			callback.OnStatus(status)
		},
	}
	return logFn, resolveFn, statusEmitter
}

func init() {
	common.MaskingEnabled = true
}

func StartBaleHeadless(socksPort int, socksUser, socksPass string, callback HeadlessCallback) {
	StopHeadless()

	activeHeadless.Lock()
	activeHeadless.callback = callback
	activeHeadless.stopped = false
	activeHeadless.Unlock()

	logFn, resolveFn, statusEmitter := makeHelpers(callback)
	baleJoiner := joiner.NewBaleHeadlessJoiner(logFn, resolveFn, statusEmitter, iosPCConfigurer{})
	baleJoiner.OnConnected = makeOnConnected(socksPort, socksUser, socksPass, logFn, callback)

	activeHeadless.Lock()
	activeHeadless.joiner = baleJoiner
	activeHeadless.Unlock()

	callback.OnStatus(common.StatusReady)
}

func SendJoinParams(jsonParams string) {
	activeHeadless.Lock()
	currentJoiner := activeHeadless.joiner
	activeHeadless.Unlock()

	if currentJoiner == nil {
		return
	}
	if baleJoiner, ok := currentJoiner.(*joiner.BaleHeadlessJoiner); ok {
		go baleJoiner.RunWithParams(jsonParams)
	}
}

func StopHeadless() {
	activeHeadless.Lock()
	activeHeadless.stopped = true
	currentJoiner := activeHeadless.joiner
	socksLn := activeHeadless.socksLn
	activeHeadless.joiner = nil
	activeHeadless.socksLn = nil
	activeHeadless.callback = nil
	activeHeadless.Unlock()

	if currentJoiner != nil {
		currentJoiner.Close()
	}
	if socksLn != nil {
		socksLn.Close()
	}
}
