package tunnel

import (
	"encoding/binary"
	"sync"
	"sync/atomic"
)

const (
	seqHeaderLen     = 4
	DefaultSeqWindow = 16
)

// SequencedTunnel wraps a DataTunnel and provides in-order delivery via a
// 4-byte big-endian sequence number prepended to every payload. Out-of-order
// arrivals are buffered up to seqWindow frames; if a missing seq does not
// arrive in time, the receiver advances past the gap so the stream does not
// stall forever (the gap will still corrupt any TCP stream traversing the
// tunnel - that is a separate concern handled by duplication or retransmit
// layers, not here).
//
// The wrapper is wire-incompatible with a peer that does not also use it; both
// ends must agree.
type SequencedTunnel struct {
	inner     DataTunnel
	seqWindow uint32

	sendSeq atomic.Uint32

	mu          sync.Mutex
	expectedSeq uint32
	haveExpect  bool
	reorderBuf  map[uint32][]byte
	onData      func([]byte)
	onClose     func()

	delivered atomic.Uint64
	reordered atomic.Uint64
	skipped   atomic.Uint64
	dropped   atomic.Uint64
}

func NewSequencedTunnel(inner DataTunnel, seqWindow uint32) *SequencedTunnel {
	if seqWindow == 0 {
		seqWindow = DefaultSeqWindow
	}
	s := &SequencedTunnel{
		inner:      inner,
		seqWindow:  seqWindow,
		reorderBuf: make(map[uint32][]byte),
	}
	inner.SetOnData(s.handleInner)
	inner.SetOnClose(s.handleInnerClose)
	return s
}

func (s *SequencedTunnel) SendData(data []byte) {
	if len(data) == 0 {
		return
	}
	seq := s.sendSeq.Add(1)
	framed := make([]byte, seqHeaderLen+len(data))
	binary.BigEndian.PutUint32(framed[:seqHeaderLen], seq)
	copy(framed[seqHeaderLen:], data)
	s.inner.SendData(framed)
}

func (s *SequencedTunnel) SetOnData(fn func([]byte)) {
	s.mu.Lock()
	s.onData = fn
	s.mu.Unlock()
}

func (s *SequencedTunnel) SetOnClose(fn func()) {
	s.mu.Lock()
	s.onClose = fn
	s.mu.Unlock()
}

func (s *SequencedTunnel) Reconfigure(fps, batch int) {
	s.inner.Reconfigure(fps, batch)
}

// Stats returns counters for diagnostics: delivered = payloads passed to
// onData, reordered = payloads buffered before delivery, skipped = expectedSeq
// values declared lost (never arrived within window), dropped = duplicates or
// frames older than expectedSeq.
func (s *SequencedTunnel) Stats() (delivered, reordered, skipped, dropped uint64) {
	return s.delivered.Load(), s.reordered.Load(), s.skipped.Load(), s.dropped.Load()
}

func (s *SequencedTunnel) handleInner(data []byte) {
	if len(data) < seqHeaderLen {
		return
	}
	seq := binary.BigEndian.Uint32(data[:seqHeaderLen])
	payload := append([]byte(nil), data[seqHeaderLen:]...)

	var deliver [][]byte

	s.mu.Lock()
	if !s.haveExpect {
		s.expectedSeq = seq
		s.haveExpect = true
	}
	switch {
	case seq == s.expectedSeq:
		deliver = append(deliver, payload)
		s.expectedSeq++
		deliver = s.drainContiguousLocked(deliver)
	case seqGreater(seq, s.expectedSeq):
		diff := seq - s.expectedSeq
		if diff > s.seqWindow {
			// gap larger than window: declare the missing seqs lost, jump.
			lost := uint64(diff)
			s.skipped.Add(lost)
			s.expectedSeq = seq
			deliver = append(deliver, payload)
			s.expectedSeq++
			s.discardStaleLocked()
			deliver = s.drainContiguousLocked(deliver)
		} else {
			s.reorderBuf[seq] = payload
			s.reordered.Add(1)
			// guard against an unbounded buffer if the writer outruns the
			// expected seq for a long time (should not happen with the diff
			// check above, but defensive).
			if uint32(len(s.reorderBuf)) > s.seqWindow {
				var smallest uint32 = ^uint32(0)
				first := true
				for k := range s.reorderBuf {
					if first || seqLess(k, smallest) {
						smallest = k
						first = false
					}
				}
				if !first {
					s.skipped.Add(uint64(smallest - s.expectedSeq))
					s.expectedSeq = smallest
					deliver = s.drainContiguousLocked(deliver)
				}
			}
		}
	default:
		// seq < expectedSeq: duplicate or very late
		s.dropped.Add(1)
	}
	cb := s.onData
	s.mu.Unlock()

	if cb != nil {
		for _, p := range deliver {
			cb(p)
		}
	}
	s.delivered.Add(uint64(len(deliver)))
}

func (s *SequencedTunnel) drainContiguousLocked(deliver [][]byte) [][]byte {
	for {
		buf, ok := s.reorderBuf[s.expectedSeq]
		if !ok {
			return deliver
		}
		delete(s.reorderBuf, s.expectedSeq)
		deliver = append(deliver, buf)
		s.expectedSeq++
	}
}

func (s *SequencedTunnel) discardStaleLocked() {
	for k := range s.reorderBuf {
		if !seqGreaterOrEqual(k, s.expectedSeq) {
			delete(s.reorderBuf, k)
		}
	}
}

func (s *SequencedTunnel) handleInnerClose() {
	s.mu.Lock()
	cb := s.onClose
	s.mu.Unlock()
	if cb != nil {
		cb()
	}
}

// seqGreater returns true if a > b under 32-bit modular comparison (RFC 1982).
func seqGreater(a, b uint32) bool { return int32(a-b) > 0 }
func seqLess(a, b uint32) bool    { return int32(a-b) < 0 }
func seqGreaterOrEqual(a, b uint32) bool {
	return a == b || seqGreater(a, b)
}
