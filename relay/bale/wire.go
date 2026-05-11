package bale

import "fmt"

const (
	fieldClientPackRequest          = 1
	fieldClientPackPing             = 2
	fieldClientPackHandshakeRequest = 3

	fieldServerPackResponse          = 1
	fieldServerPackUpdate            = 2
	fieldServerPackTerminateSession  = 3
	fieldServerPackPong              = 4
	fieldServerPackHandshakeResponse = 5

	fieldRequestServiceName = 1
	fieldRequestMethod      = 2
	fieldRequestPayload     = 3
	fieldRequestIndex       = 5

	fieldResponseError    = 1
	fieldResponseResponse = 2
	fieldResponseIndex    = 3

	fieldHandshakeMkprotoVersion = 1
	fieldHandshakeAPIVersion     = 2

	fieldPingID = 1
)

const (
	wireVarint  = 0
	wireFixed64 = 1
	wireBytes   = 2
	wireFixed32 = 5
)

type pbWriter struct{ buf []byte }

func (w *pbWriter) varint(v uint64) {
	for v >= 0x80 {
		w.buf = append(w.buf, byte(v)|0x80)
		v >>= 7
	}
	w.buf = append(w.buf, byte(v))
}

func (w *pbWriter) tag(field, wire uint64) { w.varint(field<<3 | wire) }

func (w *pbWriter) string(field uint64, s string) {
	w.tag(field, wireBytes)
	w.varint(uint64(len(s)))
	w.buf = append(w.buf, s...)
}

func (w *pbWriter) bytes(field uint64, b []byte) {
	w.tag(field, wireBytes)
	w.varint(uint64(len(b)))
	w.buf = append(w.buf, b...)
}

func (w *pbWriter) message(field uint64, b []byte) { w.bytes(field, b) }

func (w *pbWriter) int32(field uint64, v int32) {
	w.tag(field, wireVarint)
	w.varint(uint64(uint32(v)))
}

func (w *pbWriter) int64(field uint64, v int64) {
	w.tag(field, wireVarint)
	w.varint(uint64(v))
}

type pbReader struct {
	buf []byte
	pos int
}

func (r *pbReader) eof() bool { return r.pos >= len(r.buf) }

func (r *pbReader) varint() (uint64, error) {
	var v uint64
	var shift uint
	for {
		if r.pos >= len(r.buf) {
			return 0, fmt.Errorf("varint: unexpected eof")
		}
		b := r.buf[r.pos]
		r.pos++
		v |= uint64(b&0x7f) << shift
		if b < 0x80 {
			return v, nil
		}
		shift += 7
		if shift >= 64 {
			return 0, fmt.Errorf("varint: overflow")
		}
	}
}

func (r *pbReader) tag() (field, wire uint64, err error) {
	t, err := r.varint()
	if err != nil {
		return 0, 0, err
	}
	return t >> 3, t & 7, nil
}

func (r *pbReader) bytes() ([]byte, error) {
	n, err := r.varint()
	if err != nil {
		return nil, err
	}
	if r.pos+int(n) > len(r.buf) {
		return nil, fmt.Errorf("bytes: short read")
	}
	out := r.buf[r.pos : r.pos+int(n)]
	r.pos += int(n)
	return out, nil
}

func (r *pbReader) skipWire(wire uint64) error {
	switch wire {
	case wireVarint:
		_, err := r.varint()
		return err
	case wireFixed64:
		if r.pos+8 > len(r.buf) {
			return fmt.Errorf("skip: short fixed64")
		}
		r.pos += 8
		return nil
	case wireBytes:
		_, err := r.bytes()
		return err
	case wireFixed32:
		if r.pos+4 > len(r.buf) {
			return fmt.Errorf("skip: short fixed32")
		}
		r.pos += 4
		return nil
	}
	return fmt.Errorf("unknown wire type %d", wire)
}

func EncodeHandshakeRequest(mkprotoVersion int32, apiVersion int64) []byte {
	w := pbWriter{}
	w.int32(fieldHandshakeMkprotoVersion, mkprotoVersion)
	w.int64(fieldHandshakeAPIVersion, apiVersion)
	return w.buf
}

func EncodePing(id int64) []byte {
	w := pbWriter{}
	w.int64(fieldPingID, id)
	return w.buf
}

func EncodeRequest(serviceName, method string, payload []byte, index int64) []byte {
	w := pbWriter{}
	w.string(fieldRequestServiceName, serviceName)
	w.string(fieldRequestMethod, method)
	if len(payload) > 0 {
		w.bytes(fieldRequestPayload, payload)
	}
	if index != 0 {
		w.int64(fieldRequestIndex, index)
	}
	return w.buf
}

func EncodeClientPack(req, ping, handshake []byte) []byte {
	w := pbWriter{}
	if req != nil {
		w.message(fieldClientPackRequest, req)
	}
	if ping != nil {
		w.message(fieldClientPackPing, ping)
	}
	if handshake != nil {
		w.message(fieldClientPackHandshakeRequest, handshake)
	}
	return w.buf
}

type ServerPack struct {
	Response          *Response
	Update            []byte
	TerminateSession  bool
	Pong              *PingPong
	HandshakeResponse *HandshakePB
}

type Response struct {
	Error    []byte
	Response []byte
	Index    int64
}

type PingPong struct{ ID int64 }

type HandshakePB struct {
	MkprotoVersion int32
	APIVersion     int64
}

func DecodeServerPack(data []byte) (ServerPack, error) {
	var p ServerPack
	r := pbReader{buf: data}
	for !r.eof() {
		field, wire, err := r.tag()
		if err != nil {
			return p, err
		}
		if wire != wireBytes {
			if err := r.skipWire(wire); err != nil {
				return p, err
			}
			continue
		}
		inner, err := r.bytes()
		if err != nil {
			return p, err
		}
		switch field {
		case fieldServerPackResponse:
			resp, err := decodeResponse(inner)
			if err != nil {
				return p, err
			}
			p.Response = &resp
		case fieldServerPackUpdate:
			p.Update = append([]byte(nil), inner...)
		case fieldServerPackTerminateSession:
			p.TerminateSession = true
		case fieldServerPackPong:
			pp, err := decodePingPong(inner)
			if err != nil {
				return p, err
			}
			p.Pong = &pp
		case fieldServerPackHandshakeResponse:
			hs, err := decodeHandshake(inner)
			if err != nil {
				return p, err
			}
			p.HandshakeResponse = &hs
		}
	}
	return p, nil
}

func decodeResponse(data []byte) (Response, error) {
	r := pbReader{buf: data}
	var out Response
	for !r.eof() {
		field, wire, err := r.tag()
		if err != nil {
			return out, err
		}
		switch {
		case field == fieldResponseError && wire == wireBytes:
			b, err := r.bytes()
			if err != nil {
				return out, err
			}
			out.Error = append([]byte(nil), b...)
		case field == fieldResponseResponse && wire == wireBytes:
			b, err := r.bytes()
			if err != nil {
				return out, err
			}
			out.Response = append([]byte(nil), b...)
		case field == fieldResponseIndex && wire == wireVarint:
			v, err := r.varint()
			if err != nil {
				return out, err
			}
			out.Index = int64(v)
		default:
			if err := r.skipWire(wire); err != nil {
				return out, err
			}
		}
	}
	return out, nil
}

func decodePingPong(data []byte) (PingPong, error) {
	r := pbReader{buf: data}
	var out PingPong
	for !r.eof() {
		field, wire, err := r.tag()
		if err != nil {
			return out, err
		}
		if field == fieldPingID && wire == wireVarint {
			v, err := r.varint()
			if err != nil {
				return out, err
			}
			out.ID = int64(v)
		} else if err := r.skipWire(wire); err != nil {
			return out, err
		}
	}
	return out, nil
}

func decodeHandshake(data []byte) (HandshakePB, error) {
	r := pbReader{buf: data}
	var out HandshakePB
	for !r.eof() {
		field, wire, err := r.tag()
		if err != nil {
			return out, err
		}
		switch {
		case field == fieldHandshakeMkprotoVersion && wire == wireVarint:
			v, err := r.varint()
			if err != nil {
				return out, err
			}
			out.MkprotoVersion = int32(v)
		case field == fieldHandshakeAPIVersion && wire == wireVarint:
			v, err := r.varint()
			if err != nil {
				return out, err
			}
			out.APIVersion = int64(v)
		default:
			if err := r.skipWire(wire); err != nil {
				return out, err
			}
		}
	}
	return out, nil
}
