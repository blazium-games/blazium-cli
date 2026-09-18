package guard

import (
	"testing"
)

func TestProtoRoundTrip(t *testing.T) {
	raw := concat(
		encodeString(3, "R12345"),
		encodeBytes(1, []byte{1, 2, 3}),
		encodeVarintField(10, 1),
		encodeFixed64(2, 99),
	)
	m, err := decodeProto(raw)
	if err != nil {
		t.Fatal(err)
	}
	if m.str(3) != "R12345" {
		t.Fatalf("str %q", m.str(3))
	}
	if string(m.bytes(1)) != string([]byte{1, 2, 3}) {
		t.Fatalf("bytes %v", m.bytes(1))
	}
	if m.u(10) != 1 || m.u(2) != 99 {
		t.Fatalf("%v %v", m.u(10), m.u(2))
	}
}
