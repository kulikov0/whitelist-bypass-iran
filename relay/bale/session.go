package bale

import (
	"context"
	"net"
	"sync"

	"github.com/google/uuid"
	"github.com/pion/datachannel"
	"github.com/pion/rtp"
	"github.com/pion/rtp/codecs"
	"github.com/pion/webrtc/v4"
	"whitelist-bypass-iran/relay/common"
	"whitelist-bypass-iran/relay/livekit"
	"whitelist-bypass-iran/relay/tunnel"
)

const (
	RoleCreator = "creator"
	RoleJoiner  = "joiner"

	ModeVP8 = "vp8"
	ModeDC  = "dc"

	dcLabelReliable = "_reliable"
)

type SessionConfig struct {
	Role           string
	TunnelMode     string
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
	KickFn         func(identity string)
}

type peerInfo struct {
	identity string
	state    int32
}

type Session struct {
	cfg SessionConfig

	lk          *livekit.Client
	sampleTrack *webrtc.TrackLocalStaticSample

	mu          sync.Mutex
	vp8tun      *tunnel.VP8DataTunnel
	vp8Selected tunnel.DataTunnel
	dcTun       *tunnel.DCTunnel
	dcWrite     datachannel.ReadWriteCloser
	dcRead      datachannel.ReadWriteCloser
	outDC       *webrtc.DataChannel
	tunFired    bool
	peers       map[string]peerInfo

	done chan struct{}

	OnConnected       func(tunnel.DataTunnel)
	OnPeerRestart     func()
	OnRemoteCandidate func(target int, candidate string)
}

func NewSession(cfg SessionConfig) *Session {
	if cfg.Role == "" {
		cfg.Role = RoleCreator
	}
	if cfg.TunnelMode == "" {
		cfg.TunnelMode = ModeVP8
	}
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
	s.lk.OnDataChannel = s.onRemoteDC
	s.lk.OnPubConnected = s.onPubConnected
	if s.cfg.Role == RoleCreator {
		s.lk.OnParticipantUpdate = s.onParticipantUpdate
	}
	if s.OnRemoteCandidate != nil {
		s.lk.OnRemoteCandidate = s.OnRemoteCandidate
	}

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

func (s *Session) publishesVP8() bool {
	return s.cfg.Role == RoleCreator || s.cfg.TunnelMode == ModeVP8
}

func (s *Session) publishesDC() bool {
	return s.cfg.Role == RoleCreator || s.cfg.TunnelMode == ModeDC
}

func (s *Session) onLKReady() {
	pubPC := s.lk.PubPC()
	if pubPC == nil {
		return
	}

	if s.publishesVP8() {
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
	}

	if s.publishesDC() {
		ordered := true
		dc, err := pubPC.CreateDataChannel(dcLabelReliable, &webrtc.DataChannelInit{Ordered: &ordered})
		if err != nil {
			s.cfg.LogFn("[lk] create reliable DC: %v", err)
			return
		}
		s.mu.Lock()
		s.outDC = dc
		s.mu.Unlock()
		dc.OnOpen(func() {
			raw, err := dc.Detach()
			if err != nil {
				s.cfg.LogFn("[lk] outgoing DC detach: %v", err)
				return
			}
			s.cfg.LogFn("[lk] outgoing DC %q open and detached", dc.Label())
			s.mu.Lock()
			s.dcWrite = raw
			s.mu.Unlock()
			s.tryAssembleDC()
		})
		dc.OnError(func(err error) { s.cfg.LogFn("[lk] outgoing DC error: %v", err) })
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
	s.cfg.LogFn("[lk] role=%s sent publisher offer %d bytes (vp8=%v dc=%v)",
		s.cfg.Role, len(offer.SDP), s.publishesVP8(), s.publishesDC())
}

func (s *Session) onPubConnected() {
	if !s.publishesVP8() {
		return
	}
	s.mu.Lock()
	track := s.sampleTrack
	if s.vp8tun != nil || track == nil {
		s.mu.Unlock()
		return
	}
	s.vp8tun = tunnel.NewVP8DataTunnel(track, s.cfg.Obfuscator, s.cfg.LogFn)
	s.vp8tun.Start(s.cfg.VP8FPS, s.cfg.VP8Batch)
	s.vp8Selected = tunnel.NewSequencedTunnel(s.vp8tun, tunnel.DefaultSeqWindow)
	s.mu.Unlock()
	s.cfg.LogFn("[lk] vp8 tunnel writer started")

	if s.cfg.Role == RoleJoiner && s.cfg.TunnelMode == ModeVP8 {
		s.selectMode(ModeVP8)
	}
}

func (s *Session) onRemoteDC(dc *webrtc.DataChannel) {
	if dc.Label() != dcLabelReliable {
		s.cfg.LogFn("[lk] ignoring incoming DC label=%q", dc.Label())
		return
	}
	if !s.publishesDC() {
		// joiner mode=vp8: leave peer's DC open and undetached, ignore it.
		return
	}
	dc.OnOpen(func() {
		raw, err := dc.Detach()
		if err != nil {
			s.cfg.LogFn("[lk] incoming DC detach: %v", err)
			return
		}
		s.cfg.LogFn("[lk] incoming DC %q open and detached", dc.Label())
		s.mu.Lock()
		s.dcRead = raw
		s.mu.Unlock()
		s.tryAssembleDC()
	})
	dc.OnError(func(err error) { s.cfg.LogFn("[lk] incoming DC error: %v", err) })
}

func (s *Session) tryAssembleDC() {
	s.mu.Lock()
	if s.dcTun != nil || s.dcWrite == nil || s.dcRead == nil {
		s.mu.Unlock()
		return
	}
	read := livekit.NewDataPacketWrapper(s.dcRead, livekit.DataPacketKindReliable)
	write := livekit.NewDataPacketWrapper(s.dcWrite, livekit.DataPacketKindReliable)
	tun := tunnel.NewDCTunnelFromRaw(read, write, s.cfg.Obfuscator, common.DCBufSize, s.cfg.LogFn)
	s.dcTun = tun
	s.mu.Unlock()
	s.cfg.LogFn("[lk] dc tunnel assembled")

	if s.cfg.Role == RoleJoiner {
		s.selectMode(ModeDC)
		return
	}
	// Bale SFU pre-opens _reliable on every subPC, so wait for first real
	// payload before committing creator to dc mode.
	tun.OnFirstMessage = func() { s.selectMode(ModeDC) }
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
	if !s.publishesVP8() {
		// joiner mode=dc: drain and discard creator's VP8 track.
		go func() {
			buf := make([]byte, common.RTPBufSize)
			for {
				if _, _, err := track.Read(buf); err != nil {
					return
				}
			}
		}()
		return
	}
	if s.cfg.Role == RoleCreator {
		s.mu.Lock()
		wasFired := s.tunFired
		s.mu.Unlock()
		if wasFired {
			s.cfg.LogFn("[lk] new vp8 track arrived after mode locked, re-arming")
			s.rearmAutoDetect()
		}
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

		s.mu.Lock()
		t := s.vp8tun
		s.mu.Unlock()
		if t != nil {
			if recvCount == 1 && s.cfg.Role == RoleCreator {
				s.selectMode(ModeVP8)
			}
			t.HandleFrame(frameBuf)
		}
		frameBuf = frameBuf[:0]
		frameValid = false
	}
}

func (s *Session) selectMode(mode string) {
	s.mu.Lock()
	if s.tunFired {
		s.mu.Unlock()
		return
	}
	var chosen tunnel.DataTunnel
	switch mode {
	case ModeVP8:
		chosen = s.vp8Selected
	case ModeDC:
		chosen = s.dcTun
	}
	if chosen == nil {
		s.mu.Unlock()
		return
	}
	s.tunFired = true
	s.mu.Unlock()
	s.cfg.LogFn("[lk] tunnel mode selected: %s (role=%s)", mode, s.cfg.Role)
	if s.OnConnected != nil {
		s.OnConnected(chosen)
	}
}

func (s *Session) rearmAutoDetect() {
	s.mu.Lock()
	s.tunFired = false
	seq, _ := s.vp8Selected.(*tunnel.SequencedTunnel)
	s.mu.Unlock()
	if seq != nil {
		seq.ResetRecv()
	}
	if s.OnPeerRestart != nil {
		s.OnPeerRestart()
	}
}

func (s *Session) onParticipantUpdate(updates []livekit.ParticipantInfo) {
	selfSID := s.lk.Join().ParticipantSID
	s.mu.Lock()
	if s.peers == nil {
		s.peers = make(map[string]peerInfo)
	}
	newcomers := make(map[string]bool)
	for _, p := range updates {
		if p.SID == "" || p.SID == selfSID {
			continue
		}
		if p.State == livekit.ParticipantStateDisconnected {
			if pi, ok := s.peers[p.SID]; ok {
				delete(s.peers, p.SID)
				s.cfg.LogFn("[lk] participant left sid=%s identity=%s (remaining=%d)", p.SID, pi.identity, len(s.peers))
			}
			continue
		}
		pi, ok := s.peers[p.SID]
		if !ok {
			pi = peerInfo{identity: p.Identity, state: p.State}
			s.peers[p.SID] = pi
			s.cfg.LogFn("[lk] participant joined sid=%s identity=%s state=%d (total=%d)", p.SID, p.Identity, p.State, len(s.peers))
			newcomers[p.SID] = true
			continue
		}
		if p.Identity != "" && pi.identity != p.Identity {
			pi.identity = p.Identity
		}
		pi.state = p.State
		s.peers[p.SID] = pi
	}
	var stale []string
	if len(newcomers) > 0 {
		for sid, pi := range s.peers {
			if newcomers[sid] {
				continue
			}
			if pi.state == livekit.ParticipantStateActive && pi.identity != "" {
				stale = append(stale, pi.identity)
			}
		}
	}
	s.mu.Unlock()
	if len(stale) > 0 && s.cfg.KickFn != nil {
		for _, id := range stale {
			s.cfg.LogFn("[lk] kicking stale peer identity=%s", id)
			s.cfg.KickFn(id)
		}
	}
}

func (s *Session) Close() {
	if s.lk != nil {
		s.lk.Close()
	}
}
