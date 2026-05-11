package joiner

import (
	"github.com/pion/webrtc/v4"
)

type ResolveFunc func(hostname string) (string, error)

type StatusEmitter interface {
	EmitStatus(status string)
	EmitStatusError(msg string)
}

type PeerConnectionConfigurer interface {
	ConfigureSettingEngine(settingEngine *webrtc.SettingEngine)
}
