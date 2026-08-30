package ntpkeys

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseTestdata(t *testing.T) {
	root := moduleRoot(t)
	tab, err := ParseFile(filepath.Join(root, "testdata", "keys", "ntp.keys"))
	if err != nil {
		t.Fatal(err)
	}
	if len(tab.ByID) != 3 {
		t.Fatalf("keys=%d", len(tab.ByID))
	}
	if tab.ByID[2].Alg != "MD5" {
		t.Fatalf("md5 alg %s", tab.ByID[2].Alg)
	}
	if string(tab.ByID[2].Secret) != "labdev-md5-key" {
		t.Fatal("md5 secret")
	}
}

func TestParseErrors(t *testing.T) {
	if _, err := Parse([]byte("0 MD5 ascii:x\n")); err == nil {
		t.Fatal("keyid 0")
	}
	if _, err := Parse([]byte("1 MD5 ascii:x\n1 SHA1 hex:aa\n")); err == nil {
		t.Fatal("duplicate")
	}
	if _, err := Parse([]byte("1 HMAC ascii:x\n")); err == nil {
		t.Fatal("hmac")
	}
}

func TestZero(t *testing.T) {
	b := []byte("secret")
	Zero(b)
	for _, c := range b {
		if c != 0 {
			t.Fatal("not zeroed")
		}
	}
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod")
		}
		dir = parent
	}
}
