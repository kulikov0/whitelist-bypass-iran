package bale

import (
	"context"
	"net"
	"sync"

	"github.com/google/uuid"
	"github.com/pion/rtp"
	"github.com/pion/rtp/codecs"
	"github.com/pion/webrtc/v4"
	"whitelist-bypass-iran/relay/common"
	"whitelist-bypass-iran/relay/livekit"
	"whitelist-bypass-iran/relay/tunnel"
)

type SessionConfig struct {
	WSURL          string
	RoomToken      string
	Origin         string
	Obfuscator     *tunnel.TunnelObfuscator
	LogFn          func(string, ...any)
	VP8FPS         int
	VP8Batch       int
	SettingEngine  *webrtc.SettingEngine
	NetDialContext func(ctx context.Context, network, addr string) (net.Conn, error)
	ResolveICEHost func(host string) (string, error)
}

type Session struct {
	cfg SessionConfig

	lk          *livekit.Client
	sampleTrack *webrtc.TrackLocalStaticSample

	vp8tun *tunnel.VP8DataTunnel
	mu     sync.Mutex
	done   chan struct{}

	OnConnected func(tunnel.DataTunnel)
}

func NewSession(cfg SessionConfig) *Session {
	return &Session{cfg: cfg, done: make(chan struct{})}
}

func (s *Session) Done() <-chan struct{} { return s.done }

func (s *Session) Start() error {
	s.lk = livekit.NewClient(livekit.Config{
		ServerURL:      s.cfg.WSURL,
		Token:          s.cfg.RoomToken,
		Origin:         s.cfg.Origin,
		UserAgent:      common.UserAgent,
		LogFn:          s.cfg.LogFn,
		SettingEngine:  s.cfg.SettingEngine,
		NetDialContext: s.cfg.NetDialContext,
		ResolveICEHost: s.cfg.ResolveICEHost,
	})
	s.lk.OnReady = s.onLKReady
	s.lk.OnTrack = s.onRemoteTrack
	s.lk.OnPubConnected = s.startTunnel

	if err := s.lk.Connect(); err != nil {
		return err
	}
	go s.lk.PingLoop()
	go func() {
		if err := s.lk.ReadLoop(); err != nil {
			s.cfg.LogFn("[lk] read loop ended: %v", err)
		}
		close(s.done)
	}()
	return nil
}

func (s *Session) onLKReady() {
	pubPC := s.lk.PubPC()
	if pubPC == nil {
		return
	}

	trackID := "videochannel-" + uuid.New().String()
	track, err := webrtc.NewTrackLocalStaticSample(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeVP8, ClockRate: 90000},
		trackID, "tunnel-video-"+uuid.New().String(),
	)
	if err != nil {
		s.cfg.LogFn("[lk] create local track: %v", err)
		return
	}
	s.mu.Lock()
	s.sampleTrack = track
	s.mu.Unlock()

	if _, err := pubPC.AddTransceiverFromTrack(track,
		webrtc.RTPTransceiverInit{Direction: webrtc.RTPTransceiverDirectionSendonly}); err != nil {
		s.cfg.LogFn("[lk] add transceiver: %v", err)
		return
	}

	if err := s.lk.SendAddTrack(track.ID(), "videochannel",
		livekit.TrackTypeVideo, livekit.TrackSourceCamera, 1280, 720); err != nil {
		s.cfg.LogFn("[lk] send add-track: %v", err)
		return
	}

	offer, err := pubPC.CreateOffer(nil)
	if err != nil {
		s.cfg.LogFn("[lk] create offer: %v", err)
		return
	}
	if err := pubPC.SetLocalDescription(offer); err != nil {
		s.cfg.LogFn("[lk] set local offer: %v", err)
		return
	}
	if err := s.lk.SendOffer(offer.SDP); err != nil {
		s.cfg.LogFn("[lk] send offer: %v", err)
		return
	}
	s.cfg.LogFn("[lk] sent publisher offer %d bytes", len(offer.SDP))
}

func (s *Session) startTunnel() {
	s.mu.Lock()
	if s.vp8tun != nil || s.sampleTrack == nil {
		s.mu.Unlock()
		return
	}
	s.vp8tun = tunnel.NewVP8DataTunnel(s.sampleTrack, s.cfg.Obfuscator, s.cfg.LogFn)
	s.vp8tun.Start(s.cfg.VP8FPS, s.cfg.VP8Batch)
	seqTun := tunnel.NewSequencedTunnel(s.vp8tun, tunnel.DefaultSeqWindow)
	s.mu.Unlock()
	s.cfg.LogFn("[lk] vp8 tunnel writer started, seq window=%d", tunnel.DefaultSeqWindow)
	if s.OnConnected != nil {
		s.OnConnected(seqTun)
	}
}

func (s *Session) currentVP8Tun() *tunnel.VP8DataTunnel {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.vp8tun
}

func (s *Session) onRemoteTrack(track *webrtc.TrackRemote, _ *webrtc.RTPReceiver) {
	if track.Codec().MimeType != webrtc.MimeTypeVP8 {
		go func() {
			buf := make([]byte, common.UDPBufSize)
			for {
				if _, _, err := track.Read(buf); err != nil {
					return
				}
			}
		}()
		return
	}
	go s.readVP8Track(track)
}

func (s *Session) readVP8Track(track *webrtc.TrackRemote) {
	var vp8Pkt codecs.VP8Packet
	var frameBuf []byte
	var lastSeq uint16
	var haveLastSeq bool
	frameValid := false
	var recvCount int
	buf := make([]byte, common.RTPBufSize)
	for {
		n, _, err := track.Read(buf)
		if err != nil {
			return
		}
		pkt := &rtp.Packet{}
		if pkt.Unmarshal(buf[:n]) != nil {
			continue
		}
		if haveLastSeq && pkt.SequenceNumber != lastSeq+1 {
			frameValid = false
			frameBuf = frameBuf[:0]
		}
		lastSeq = pkt.SequenceNumber
		haveLastSeq = true

		vp8Payload, err := vp8Pkt.Unmarshal(pkt.Payload)
		if err != nil {
			frameValid = false
			frameBuf = frameBuf[:0]
			continue
		}
		if vp8Pkt.S == 1 {
			frameBuf = frameBuf[:0]
			frameValid = true
		}
		if !frameValid {
			continue
		}
		frameBuf = append(frameBuf, vp8Payload...)
		if !pkt.Marker {
			continue
		}
		recvCount++
		if recvCount <= 3 || recvCount%200 == 0 {
			s.cfg.LogFn("[lk-video] recv vp8 frame #%d %d bytes", recvCount, len(frameBuf))
		}

		tun := s.currentVP8Tun()
		if tun != nil {
			tun.HandleFrame(frameBuf)
		}
		frameBuf = frameBuf[:0]
		frameValid = false
	}
}

func (s *Session) Close() {
	if s.lk != nil {
		s.lk.Close()
	}
}
