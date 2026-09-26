package locale

import (
	"encoding/binary"
	"errors"
	"unicode/utf16"
)

// A deliberately small reader for Apple binary property lists ("bplist00"), just enough to read the
// AppleLanguages array from ~/Library/Preferences/.GlobalPreferences.plist without cgo or exec'ing
// `defaults` (which costs ~10 ms per CLI invocation, paid again on every shell TAB completion).

var errBadPlist = errors.New("locale: malformed binary plist")

type bplist struct {
	data    []byte
	offsets []uint64
	refSize int
	objEnd  uint64 // objects live in data[8:objEnd]
}

// appleLanguages returns the AppleLanguages array of a binary plist whose top object is a dictionary.
func appleLanguages(data []byte) ([]string, error) {
	p, top, err := openBplist(data)
	if err != nil {
		return nil, err
	}
	v, err := p.dictValue(top, "AppleLanguages")
	if err != nil || v == noRef {
		return nil, err
	}
	refs, err := p.arrayRefs(v)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, r := range refs {
		if s, ok := p.str(r); ok {
			out = append(out, s)
		}
	}
	return out, nil
}

const noRef = ^uint64(0)

func openBplist(data []byte) (*bplist, uint64, error) {
	if len(data) < 8+32 || string(data[:8]) != "bplist00" {
		return nil, 0, errBadPlist
	}
	t := data[len(data)-32:]
	offSize, refSize := int(t[6]), int(t[7])
	numObjects := binary.BigEndian.Uint64(t[8:16])
	top := binary.BigEndian.Uint64(t[16:24])
	tableOff := binary.BigEndian.Uint64(t[24:32])
	if offSize < 1 || offSize > 8 || refSize < 1 || refSize > 8 {
		return nil, 0, errBadPlist
	}
	limit := uint64(len(data) - 32)
	if numObjects == 0 || top >= numObjects || tableOff < 8 || tableOff > limit ||
		numObjects > (limit-tableOff)/uint64(offSize) {
		return nil, 0, errBadPlist
	}
	offsets := make([]uint64, numObjects)
	for i := range offsets {
		pos := tableOff + uint64(i)*uint64(offSize)
		offsets[i] = readUint(data[pos : pos+uint64(offSize)])
		if offsets[i] < 8 || offsets[i] >= tableOff {
			return nil, 0, errBadPlist
		}
	}
	return &bplist{data: data, offsets: offsets, refSize: refSize, objEnd: tableOff}, top, nil
}

func readUint(b []byte) uint64 {
	var v uint64
	for _, c := range b {
		v = v<<8 | uint64(c)
	}
	return v
}

// header decodes an object's marker and length, returning the type nibble, the length, and the offset
// of the object's payload.
func (p *bplist) header(ref uint64) (kind byte, length uint64, payload uint64, err error) {
	if ref >= uint64(len(p.offsets)) {
		return 0, 0, 0, errBadPlist
	}
	off := p.offsets[ref]
	marker := p.data[off]
	kind, info := marker>>4, uint64(marker&0x0f)
	payload = off + 1
	if info == 0x0f && kind != 0x0 && kind != 0x1 && kind != 0x2 && kind != 0x3 {
		if payload >= p.objEnd {
			return 0, 0, 0, errBadPlist
		}
		im := p.data[payload]
		if im>>4 != 0x1 {
			return 0, 0, 0, errBadPlist
		}
		n := uint64(1) << (im & 0x0f)
		if n > 8 || payload+1+n > p.objEnd {
			return 0, 0, 0, errBadPlist
		}
		info = readUint(p.data[payload+1 : payload+1+n])
		payload += 1 + n
	}
	return kind, info, payload, nil
}

func (p *bplist) refs(payload, count uint64) ([]uint64, error) {
	size := uint64(p.refSize)
	if count > (p.objEnd-payload)/size {
		return nil, errBadPlist
	}
	out := make([]uint64, count)
	for i := range out {
		pos := payload + uint64(i)*size
		out[i] = readUint(p.data[pos : pos+size])
	}
	return out, nil
}

func (p *bplist) arrayRefs(ref uint64) ([]uint64, error) {
	kind, n, payload, err := p.header(ref)
	if err != nil {
		return nil, err
	}
	if kind != 0xA {
		return nil, errBadPlist
	}
	return p.refs(payload, n)
}

func (p *bplist) dictValue(ref uint64, key string) (uint64, error) {
	kind, n, payload, err := p.header(ref)
	if err != nil {
		return noRef, err
	}
	if kind != 0xD {
		return noRef, errBadPlist
	}
	all, err := p.refs(payload, 2*n)
	if err != nil {
		return noRef, err
	}
	for i := uint64(0); i < n; i++ {
		if k, ok := p.str(all[i]); ok && k == key {
			return all[n+i], nil
		}
	}
	return noRef, nil
}

// str decodes an ASCII (0x5) or UTF-16BE (0x6) string object.
func (p *bplist) str(ref uint64) (string, bool) {
	kind, n, payload, err := p.header(ref)
	if err != nil {
		return "", false
	}
	switch kind {
	case 0x5:
		if n > p.objEnd-payload {
			return "", false
		}
		return string(p.data[payload : payload+n]), true
	case 0x6:
		if n > (p.objEnd-payload)/2 {
			return "", false
		}
		u := make([]uint16, n)
		for i := range u {
			u[i] = binary.BigEndian.Uint16(p.data[payload+2*uint64(i):])
		}
		return string(utf16.Decode(u)), true
	}
	return "", false
}
