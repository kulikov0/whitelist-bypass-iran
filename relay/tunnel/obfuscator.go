package tunnel

import (
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"log"
	"os"
	"strings"
	"sync"
	"sync/atomic"

	"golang.org/x/crypto/chacha20poly1305"
)

var debugTunnel = os.Getenv("DEBUG_TUNNEL") != ""

var vp8Keepalive = []byte{
	0x30, 0x01, 0x00, 0x9d, 0x01, 0x2a, 0x10, 0x00,
	0x10, 0x00, 0x00, 0x47, 0x08, 0x85, 0x85, 0x88,
	0x99, 0x84, 0x88, 0xfc,
}

const (
	vp8KeepaliveLen = 20
	epochOff        = vp8KeepaliveLen
	epochHdrLen     = vp8KeepaliveLen + 4
)

var ErrEmptySecret = errors.New("tunnel: obfuscator requires a non-empty secret")

type DecodeResult struct {
	HasFrame    bool
	Keepalive   bool
	SelfEcho    bool
	PeerRestart bool
	Payload     []byte
	PeerEpoch   uint32
}

type TunnelObfuscator struct {
	aead       cipher.AEAD
	localEpoch uint32
	keyHash    [32]byte

	mu        sync.Mutex
	peerEpoch uint32
	hasPeer   bool

	encCount atomic.Uint32
	decCount atomic.Uint32
}

func DeriveSecretFromJoinLink(joinLink string) []byte {
	token := extractJoinToken(joinLink)
	if token == "" {
		return nil
	}
	return []byte(token)
}

func extractJoinToken(joinLink string) string {
	s := strings.TrimSpace(joinLink)
	s = strings.TrimRight(s, "/")
	if i := strings.IndexByte(s, '?'); i >= 0 {
		s = s[:i]
	}
	if i := strings.IndexByte(s, '#'); i >= 0 {
		s = s[:i]
	}
	if i := strings.LastIndexByte(s, '/'); i >= 0 {
		s = s[i+1:]
	}
	return s
}

func NewTunnelObfuscator(secret []byte) (*TunnelObfuscator, error) {
	if len(secret) == 0 {
		return nil, ErrEmptySecret
	}
	keyHash := sha256.Sum256(secret)
	aead, err := chacha20poly1305.NewX(keyHash[:])
	if err != nil {
		return nil, err
	}
	var epochBytes [4]byte
	if _, err := rand.Read(epochBytes[:]); err != nil {
		return nil, err
	}
	epoch := binary.BigEndian.Uint32(epochBytes[:])
	if epoch == 0 {
		epoch = 1
	}
	o := &TunnelObfuscator{aead: aead, localEpoch: epoch, keyHash: keyHash}
	if debugTunnel {
		dbgLog("obf: init secretLen=%d keyHash=%s localEpoch=0x%08x", len(secret), hex.EncodeToString(keyHash[:8]), epoch)
	}
	return o, nil
}

func dbgLog(format string, args ...any) {
	if !debugTunnel {
		return
	}
	log.Printf(format, args...)
}

func hexPrefix(b []byte, n int) string {
	if len(b) < n {
		n = len(b)
	}
	return hex.EncodeToString(b[:n])
}

func (o *TunnelObfuscator) LocalEpoch() uint32 { return o.localEpoch }

func (o *TunnelObfuscator) header() []byte {
	hdr := make([]byte, epochHdrLen)
	copy(hdr, vp8Keepalive)
	binary.BigEndian.PutUint32(hdr[epochOff:], o.localEpoch)
	return hdr
}

func (o *TunnelObfuscator) EncodeKeepalive() []byte {
	return o.header()
}

func (o *TunnelObfuscator) EncodeData(payload []byte) []byte {
	hdr := o.header()
	nonce := make([]byte, o.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil
	}
	out := make([]byte, 0, len(hdr)+len(nonce)+len(payload)+o.aead.Overhead())
	out = append(out, hdr...)
	out = append(out, nonce...)
	out = o.aead.Seal(out, nonce, payload, nil)
	if debugTunnel {
		n := o.encCount.Add(1)
		if n <= 8 {
			dbgLog("obf: enc #%d payloadLen=%d outLen=%d payloadHex=%s", n, len(payload), len(out), hexPrefix(payload, 48))
		}
	}
	return out
}

func (o *TunnelObfuscator) EncryptPayload(plaintext []byte) []byte {
	if o == nil {
		return plaintext
	}
	nonce := make([]byte, o.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil
	}
	out := make([]byte, 0, len(nonce)+len(plaintext)+o.aead.Overhead())
	out = append(out, nonce...)
	return o.aead.Seal(out, nonce, plaintext, nil)
}

func (o *TunnelObfuscator) DecryptPayload(data []byte) ([]byte, bool) {
	if o == nil {
		return data, true
	}
	nonceSize := o.aead.NonceSize()
	if len(data) < nonceSize+o.aead.Overhead() {
		return nil, false
	}
	nonce := data[:nonceSize]
	ciphertext := data[nonceSize:]
	plaintext, err := o.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, false
	}
	return plaintext, true
}

func (o *TunnelObfuscator) Decode(frame []byte) DecodeResult {
	if len(frame) < epochHdrLen {
		return DecodeResult{}
	}
	peerEpoch := binary.BigEndian.Uint32(frame[epochOff:epochHdrLen])
	if peerEpoch == o.localEpoch {
		return DecodeResult{HasFrame: true, SelfEcho: true, PeerEpoch: peerEpoch}
	}

	res := DecodeResult{HasFrame: true, PeerEpoch: peerEpoch}
	o.mu.Lock()
	if !o.hasPeer {
		o.peerEpoch = peerEpoch
		o.hasPeer = true
	} else if o.peerEpoch != peerEpoch {
		o.peerEpoch = peerEpoch
		res.PeerRestart = true
	}
	o.mu.Unlock()

	if len(frame) == epochHdrLen {
		res.Keepalive = true
		return res
	}

	body := frame[epochHdrLen:]
	nonceSize := o.aead.NonceSize()
	if len(body) < nonceSize+o.aead.Overhead() {
		return DecodeResult{}
	}
	nonce := body[:nonceSize]
	ciphertext := body[nonceSize:]
	plaintext, err := o.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		if debugTunnel {
			dbgLog("obf: dec AEAD-fail frameLen=%d peerEpoch=0x%08x", len(frame), peerEpoch)
		}
		return DecodeResult{}
	}
	res.Payload = plaintext
	if debugTunnel {
		n := o.decCount.Add(1)
		if n <= 8 {
			dbgLog("obf: dec #%d frameLen=%d peerEpoch=0x%08x payloadLen=%d payloadHex=%s", n, len(frame), peerEpoch, len(plaintext), hexPrefix(plaintext, 48))
		}
	}
	return res
}
