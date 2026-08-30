package ntpwire

import (
	"bytes"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"
)

func testdata(t *testing.T, elem ...string) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			parts := append([]string{dir, "testdata"}, elem...)
			return filepath.Join(parts...)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}

func TestMACConcatenationNotHMAC(t *testing.T) {
	header := bytes.Repeat([]byte{0x42}, PacketSize)
	key := []byte("labdev-md5-key")
	got, err := MAC("MD5", key, header)
	if err != nil {
		t.Fatal(err)
	}
	h := md5.New()
	h.Write(key)
	h.Write(header)
	want := h.Sum(nil)
	if !bytes.Equal(got, want) {
		t.Fatal("MAC must be MD5(key||header)")
	}
	// HMAC-MD5 would differ for this construction.
	if len(got) != md5.Size {
		t.Fatalf("len=%d", len(got))
	}
}

func TestMACVectors(t *testing.T) {
	cases := []struct {
		file string
		alg  string
		key  []byte
		id   uint32
	}{
		{"packets/mac-md5.raw", "MD5", []byte("labdev-md5-key"), 2},
		{"packets/mac-sha1.raw", "SHA1", mustHex(t, "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"), 3},
		{"packets/mac-sha256.raw", "SHA256", mustHex(t, "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"), 1},
	}
	for _, tc := range cases {
		t.Run(tc.alg, func(t *testing.T) {
			raw, err := os.ReadFile(testdata(t, tc.file))
			if err != nil {
				t.Fatal(err)
			}
			header, keyID, digest, ok := SplitMAC(raw)
			if !ok {
				t.Fatalf("split %s", tc.file)
			}
			if keyID != tc.id {
				t.Fatalf("keyid %d want %d", keyID, tc.id)
			}
			want, err := MAC(tc.alg, tc.key, header)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(digest, want) {
				t.Fatalf("trailer digest mismatch vs ntpd concatenation ALG(key||header)")
			}
			if !VerifyMAC(tc.alg, tc.key, raw) {
				t.Fatal("VerifyMAC")
			}
		})
	}
}

func TestSplitMACUnsigned(t *testing.T) {
	p := Encode(Packet{VN: 4, Mode: 3})
	h, id, d, ok := SplitMAC(p)
	if !ok || id != 0 || d != nil || len(h) != PacketSize {
		t.Fatalf("unsigned split h=%d id=%d d=%d ok=%v", len(h), id, len(d), ok)
	}
}

func TestDigestLens(t *testing.T) {
	if n, _ := DigestLen("md5"); n != md5.Size {
		t.Fatal("md5")
	}
	if n, _ := DigestLen("SHA1"); n != sha1.Size {
		t.Fatal("sha1")
	}
	if n, _ := DigestLen("SHA256"); n != sha256.Size {
		t.Fatal("sha256")
	}
	if _, err := DigestLen("HMAC"); err == nil {
		t.Fatal("HMAC is not a v1 algorithm")
	}
}

func mustHex(t *testing.T, s string) []byte {
	t.Helper()
	b := make([]byte, len(s)/2)
	for i := 0; i < len(b); i++ {
		var v byte
		for _, c := range []byte{s[i*2], s[i*2+1]} {
			v <<= 4
			switch {
			case c >= '0' && c <= '9':
				v |= c - '0'
			case c >= 'a' && c <= 'f':
				v |= c - 'a' + 10
			default:
				t.Fatalf("hex %q", s)
			}
		}
		b[i] = v
	}
	return b
}
