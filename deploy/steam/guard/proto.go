package guard

import (
	"encoding/binary"
	"fmt"
)

type protoField struct {
	num  int
	wire int
	data []byte
}

const (
	wireVarint = 0
	wire64     = 1
	wireBytes  = 2
	wire32     = 5
)

func appendVarint(b []byte, v uint64) []byte {
	for v >= 0x80 {
		b = append(b, byte(v)|0x80)
		v >>= 7
	}
	return append(b, byte(v))
}

func putTag(b []byte, num, wire int) []byte {
	return appendVarint(b, uint64(num<<3|wire))
}

func encodeString(num int, s string) []byte {
	if s == "" {
		return nil
	}
	b := putTag(nil, num, wireBytes)
	b = appendVarint(b, uint64(len(s)))
	return append(b, s...)
}

func encodeBytes(num int, v []byte) []byte {
	if len(v) == 0 {
		return nil
	}
	b := putTag(nil, num, wireBytes)
	b = appendVarint(b, uint64(len(v)))
	return append(b, v...)
}

func encodeVarintField(num int, v uint64) []byte {
	b := putTag(nil, num, wireVarint)
	return appendVarint(b, v)
}

func encodeFixed64(num int, v uint64) []byte {
	b := putTag(nil, num, wire64)
	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:], v)
	return append(b, buf[:]...)
}

func encodeBool(num int, v bool) []byte {
	if !v {
		return encodeVarintField(num, 0)
	}
	return encodeVarintField(num, 1)
}

func concat(parts ...[]byte) []byte {
	n := 0
	for _, p := range parts {
		n += len(p)
	}
	out := make([]byte, 0, n)
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

type protoMap struct {
	fields map[int][][]byte
	varint map[int][]uint64
	u64    map[int][]uint64
}

func decodeProto(b []byte) (*protoMap, error) {
	m := &protoMap{
		fields: map[int][][]byte{},
		varint: map[int][]uint64{},
		u64:    map[int][]uint64{},
	}
	i := 0
	for i < len(b) {
		tag, n := binary.Uvarint(b[i:])
		if n <= 0 {
			return nil, fmt.Errorf("bad protobuf tag")
		}
		i += n
		num := int(tag >> 3)
		wire := int(tag & 7)
		switch wire {
		case wireVarint:
			v, n := binary.Uvarint(b[i:])
			if n <= 0 {
				return nil, fmt.Errorf("bad varint field %d", num)
			}
			i += n
			m.varint[num] = append(m.varint[num], v)
		case wire64:
			if i+8 > len(b) {
				return nil, fmt.Errorf("short fixed64 field %d", num)
			}
			v := binary.LittleEndian.Uint64(b[i : i+8])
			i += 8
			m.u64[num] = append(m.u64[num], v)
		case wireBytes:
			ln, n := binary.Uvarint(b[i:])
			if n <= 0 {
				return nil, fmt.Errorf("bad bytes len field %d", num)
			}
			i += n
			if i+int(ln) > len(b) {
				return nil, fmt.Errorf("short bytes field %d", num)
			}
			m.fields[num] = append(m.fields[num], append([]byte(nil), b[i:i+int(ln)]...))
			i += int(ln)
		case wire32:
			if i+4 > len(b) {
				return nil, fmt.Errorf("short fixed32 field %d", num)
			}
			i += 4
		default:
			return nil, fmt.Errorf("unsupported wire type %d", wire)
		}
	}
	return m, nil
}

func (m *protoMap) str(num int) string {
	if vs := m.fields[num]; len(vs) > 0 {
		return string(vs[0])
	}
	return ""
}

func (m *protoMap) bytes(num int) []byte {
	if vs := m.fields[num]; len(vs) > 0 {
		return vs[0]
	}
	return nil
}

func (m *protoMap) u(num int) uint64 {
	if vs := m.varint[num]; len(vs) > 0 {
		return vs[0]
	}
	if vs := m.u64[num]; len(vs) > 0 {
		return vs[0]
	}
	return 0
}

func (m *protoMap) b(num int) bool {
	return m.u(num) != 0
}
