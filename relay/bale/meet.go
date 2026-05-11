package bale

import "strings"

const (
	fieldCallEnvelope = 1

	fieldCallID        = 1
	fieldCallToken     = 2
	fieldCallLivekit   = 3
	fieldCallURL       = 4
	fieldCallShareLink = 12
)

type Call struct {
	ID         int64
	Token      string
	ShareLink  string
	LivekitJWT string
	URL        string
}

func EncodeGenerateCallLinkRequest(isPublic bool) []byte {
	w := pbWriter{}
	if isPublic {
		w.tag(1, wireVarint)
		w.varint(1)
	}
	return w.buf
}

func EncodeGetCallLinkDetailsRequest(session string) []byte {
	w := pbWriter{}
	w.string(1, session)
	return w.buf
}

func EncodeJoinGroupCallRequest(callID int64, name string) []byte {
	w := pbWriter{}
	w.int64(1, callID)
	if name != "" {
		nameMsg := pbWriter{}
		nameMsg.string(1, name)
		w.bytes(2, nameMsg.buf)
	}
	return w.buf
}

func DecodeCallEnvelope(data []byte) (Call, error) {
	var out Call
	r := pbReader{buf: data}
	for !r.eof() {
		field, wire, err := r.tag()
		if err != nil {
			return out, err
		}
		if field == fieldCallEnvelope && wire == wireBytes {
			inner, err := r.bytes()
			if err != nil {
				return out, err
			}
			return decodeCall(inner)
		}
		if err := r.skipWire(wire); err != nil {
			return out, err
		}
	}
	return out, nil
}

func decodeCall(data []byte) (Call, error) {
	var out Call
	r := pbReader{buf: data}
	for !r.eof() {
		field, wire, err := r.tag()
		if err != nil {
			return out, err
		}
		switch {
		case field == fieldCallID && wire == wireVarint:
			v, err := r.varint()
			if err != nil {
				return out, err
			}
			out.ID = int64(v)
		case field == fieldCallToken && wire == wireBytes:
			b, err := r.bytes()
			if err != nil {
				return out, err
			}
			out.Token = string(b)
		case field == fieldCallLivekit && wire == wireBytes:
			b, err := r.bytes()
			if err != nil {
				return out, err
			}
			out.LivekitJWT = string(b)
		case field == fieldCallURL && wire == wireBytes:
			b, err := r.bytes()
			if err != nil {
				return out, err
			}
			s, err := decodeStringValue(b)
			if err != nil {
				return out, err
			}
			out.URL = s
		case field == fieldCallShareLink && wire == wireBytes:
			b, err := r.bytes()
			if err != nil {
				return out, err
			}
			out.ShareLink = string(b)
		default:
			if err := r.skipWire(wire); err != nil {
				return out, err
			}
		}
	}
	return out, nil
}

func decodeStringValue(data []byte) (string, error) {
	r := pbReader{buf: data}
	for !r.eof() {
		field, wire, err := r.tag()
		if err != nil {
			return "", err
		}
		if field == 1 && wire == wireBytes {
			b, err := r.bytes()
			if err != nil {
				return "", err
			}
			return string(b), nil
		}
		if err := r.skipWire(wire); err != nil {
			return "", err
		}
	}
	return "", nil
}

func ExtractShareCode(joinLink string) string {
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
