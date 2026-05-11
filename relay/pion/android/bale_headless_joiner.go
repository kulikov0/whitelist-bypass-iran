package android

import (
	"log"
	"strings"

	"whitelist-bypass-iran/relay/common"
	joiner "whitelist-bypass-iran/relay/pion/headless-joiner-common"
	"whitelist-bypass-iran/relay/tunnel"
)

type BaleHeadlessJoiner struct {
	inner       *joiner.BaleHeadlessJoiner
	OnConnected func(tunnel.DataTunnel)
}

func NewBaleHeadlessJoiner(logFn func(string, ...any)) *BaleHeadlessJoiner {
	if logFn == nil {
		logFn = log.Printf
	}
	inner := joiner.NewBaleHeadlessJoiner(logFn, RequestResolve, StatusEmitter{}, PCConfigurer{})
	wrapper := &BaleHeadlessJoiner{inner: inner}
	inner.OnConnected = func(tun tunnel.DataTunnel) {
		if wrapper.OnConnected != nil {
			wrapper.OnConnected(tun)
		}
	}
	return wrapper
}

func (j *BaleHeadlessJoiner) Run() {
	j.inner.Status.EmitStatus(common.StatusReady)
	for {
		line, err := ReadStdinLine()
		if err != nil {
			log.Printf("bale-joiner: stdin closed: %v", err)
			return
		}
		if strings.HasPrefix(line, "JOIN:") {
			j.inner.RunWithParams(strings.TrimPrefix(line, "JOIN:"))
			return
		}
	}
}
