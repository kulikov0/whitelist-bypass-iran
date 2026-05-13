package tunnel

import (
	"encoding/binary"
	"io"
	"sync"
	"sync/atomic"

	"github.com/pion/datachannel"
)

const dcSendQueueDepth = 1024

type DCTunnel struct {
	readRaw  datachannel.ReadWriteCloser
	writeRaw datachannel.ReadWriteCloser
	logFn    func(string, ...any)
	obf      *TunnelObfuscator
	readBuf  int

	sendCh chan []byte
	stopCh chan struct{}

	onMu    sync.Mutex
	onData  func([]byte)
	onClose func()
	pending [][]byte

	firstOnce      sync.Once
	OnFirstMessage func()

	closed atomic.Bool

	recvBytes atomic.Uint64
	sendBytes atomic.Uint64
	recvMsgs  atomic.Uint64
	sendMsgs  atomic.Uint64
}

func NewDCTunnelFromRaw(readRaw, writeRaw datachannel.ReadWriteCloser, obf *TunnelObfuscator, readBuf int, logFn func(string, ...any)) *DCTunnel {
	t := &DCTunnel{
		readRaw:  readRaw,
		writeRaw: writeRaw,
		obf:      obf,
		logFn:    logFn,
		readBuf:  readBuf,
		sendCh:   make(chan []byte, dcSendQueueDepth),
		stopCh:   make(chan struct{}),
	}
	go t.readLoop()
	go t.writerLoop()
	return t
}

func (t *DCTunnel) SetOnData(fn func([]byte)) {
	t.onMu.Lock()
	t.onData = fn
	pending := t.pending
	t.pending = nil
	t.onMu.Unlock()
	if fn != nil {
		for _, frame := range pending {
			fn(frame)
		}
	}
}

func (t *DCTunnel) OnData() func([]byte) {
	t.onMu.Lock()
	defer t.onMu.Unlock()
	return t.onData
}

func (t *DCTunnel) SetOnClose(fn func())     { t.onClose = fn }
func (t *DCTunnel) Reconfigure(_ int, _ int) {}

func (t *DCTunnel) SendData(data []byte) {
	DecodeFrames(data, func(connID uint32, msgType byte, payload []byte) {
		buf := make([]byte, 5+len(payload))
		binary.BigEndian.PutUint32(buf[0:4], connID)
		buf[4] = msgType
		copy(buf[5:], payload)
		wire := buf
		if t.obf != nil {
			wire = t.obf.EncryptPayload(buf)
			if wire == nil {
				return
			}
		}
		select {
		case t.sendCh <- wire:
		case <-t.stopCh:
		}
	})
}

func (t *DCTunnel) writerLoop() {
	for {
		select {
		case <-t.stopCh:
			return
		case msg := <-t.sendCh:
			if t.closed.Load() {
				return
			}
			if _, err := t.writeRaw.Write(msg); err != nil {
				t.logFn("dctunnel: write error: %v", err)
				return
			}
			t.sendBytes.Add(uint64(len(msg)))
			t.sendMsgs.Add(1)
		}
	}
}

func (t *DCTunnel) readLoop() {
	bufSize := t.readBuf
	if bufSize == 0 {
		bufSize = 32768
	}
	buf := make([]byte, bufSize)
	for {
		n, isString, err := t.readRaw.ReadDataChannel(buf)
		if err != nil {
			if err != io.EOF && !t.closed.Load() {
				t.logFn("dctunnel: read error: %v", err)
			}
			if t.onClose != nil {
				t.onClose()
			}
			return
		}
		if isString || n == 0 {
			continue
		}
		t.recvBytes.Add(uint64(n))
		t.recvMsgs.Add(1)
		t.deliver(buf[:n])
	}
}

func (t *DCTunnel) deliver(wire []byte) {
	payload := wire
	if t.obf != nil {
		pt, ok := t.obf.DecryptPayload(wire)
		if !ok {
			t.logFn("dctunnel: decrypt failed (%d bytes)", len(wire))
			return
		}
		payload = pt
	}
	if len(payload) == 0 {
		return
	}
	t.firstOnce.Do(func() {
		if t.OnFirstMessage != nil {
			t.OnFirstMessage()
		}
	})
	frame := make([]byte, 4+len(payload))
	binary.BigEndian.PutUint32(frame[0:4], uint32(len(payload)))
	copy(frame[4:], payload)
	t.onMu.Lock()
	cb := t.onData
	if cb == nil {
		t.pending = append(t.pending, frame)
		t.onMu.Unlock()
		return
	}
	t.onMu.Unlock()
	cb(frame)
}

func (t *DCTunnel) Close() {
	if !t.closed.CompareAndSwap(false, true) {
		return
	}
	close(t.stopCh)
	if t.writeRaw != nil {
		_ = t.writeRaw.Close()
	}
	if t.readRaw != nil {
		_ = t.readRaw.Close()
	}
}
