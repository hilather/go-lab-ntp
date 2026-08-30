package ntpwire

import "testing"

func FuzzParse(f *testing.F) {
	f.Add(Encode(Packet{VN: 4, Mode: ModeClient, XmtTime: Timestamp{Seconds: 1}}))
	f.Add(make([]byte, 0))
	f.Add(make([]byte, 47))
	f.Add(make([]byte, 48))
	f.Add(make([]byte, 577))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 2048 {
			data = data[:2048]
		}
		p, err := Parse(data)
		if err != nil {
			return
		}
		b := Encode(p)
		if len(b) != PacketSize {
			t.Fatalf("encode len %d", len(b))
		}
		_, _, _, _ = SplitMAC(data)
		_, _ = MAC("MD5", []byte("k"), b)
	})
}
