package livekit

import "github.com/pion/datachannel"

// DataPacketWrapper adapts a detached SCTP DC to a ReadWriteCloser that
// transparently encodes/decodes LiveKit DataPacket user-payload wrappers.
// LiveKit SFUs forward DC messages only when wrapped in this format.
type DataPacketWrapper struct {
	inner   datachannel.ReadWriteCloser
	kind    int
	readBuf []byte
}

func NewDataPacketWrapper(inner datachannel.ReadWriteCloser, kind int) *DataPacketWrapper {
	return &DataPacketWrapper{inner: inner, kind: kind}
}

func (w *DataPacketWrapper) ReadDataChannel(p []byte) (int, bool, error) {
	if len(w.readBuf) < len(p) {
		w.readBuf = make([]byte, len(p))
	}
	buf := w.readBuf[:len(p)]
	for {
		n, isString, err := w.inner.ReadDataChannel(buf)
		if err != nil {
			return 0, false, err
		}
		if n == 0 {
			continue
		}
		payload, ok := DecodeDataPacketUser(buf[:n])
		if !ok || len(payload) == 0 {
			continue
		}
		return copy(p, payload), isString, nil
	}
}

func (w *DataPacketWrapper) WriteDataChannel(p []byte, isString bool) (int, error) {
	if _, err := w.inner.WriteDataChannel(EncodeDataPacketUser(p, w.kind), isString); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (w *DataPacketWrapper) Read(p []byte) (int, error) {
	n, _, err := w.ReadDataChannel(p)
	return n, err
}

func (w *DataPacketWrapper) Write(p []byte) (int, error) {
	return w.WriteDataChannel(p, false)
}

func (w *DataPacketWrapper) Close() error { return w.inner.Close() }
