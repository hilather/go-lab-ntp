package web

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func testFS() fstest.MapFS {
	return fstest.MapFS{
		"index.html":                   {Data: []byte("<!doctype html><title>LabNTP</title>")},
		"assets/index-0123456789ab.js": {Data: []byte("console.log('ok')")},
		"favicon.ico":                  {Data: []byte("ico")},
	}
}

func TestSPAFallbackAndCache(t *testing.T) {
	t.Parallel()
	h := NewHandler(testFS())

	index := httptest.NewRecorder()
	h.ServeHTTP(index, httptest.NewRequest(http.MethodGet, "/", nil))
	if index.Code != http.StatusOK {
		t.Fatalf("GET / code=%d", index.Code)
	}
	if !strings.Contains(index.Body.String(), "LabNTP") {
		t.Fatalf("body=%s", index.Body.String())
	}
	if got := index.Header().Get("Cache-Control"); got != cacheHTML {
		t.Fatalf("index cache=%q", got)
	}

	preview := httptest.NewRecorder()
	h.ServeHTTP(preview, httptest.NewRequest(http.MethodGet, "/preview", nil))
	if preview.Code != http.StatusOK || !strings.Contains(preview.Body.String(), "LabNTP") {
		t.Fatalf("SPA fallback code=%d body=%s", preview.Code, preview.Body.String())
	}

	asset := httptest.NewRecorder()
	h.ServeHTTP(asset, httptest.NewRequest(http.MethodGet, "/assets/index-0123456789ab.js", nil))
	if asset.Code != http.StatusOK {
		t.Fatalf("asset code=%d", asset.Code)
	}
	if got := asset.Header().Get("Cache-Control"); got != cacheHashed {
		t.Fatalf("hashed cache=%q", got)
	}

	missing := httptest.NewRecorder()
	h.ServeHTTP(missing, httptest.NewRequest(http.MethodGet, "/assets/nope-0123456789ab.js", nil))
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing asset code=%d", missing.Code)
	}
}

func TestReservedPathsNotCaptured(t *testing.T) {
	t.Parallel()
	h := NewHandler(testFS())
	for _, p := range []string{
		"/v1/filters",
		"/v1/session",
		"/mcp",
		"/mcp/x",
		"/healthz",
		"/config",
		"/.well-known/oauth-protected-resource",
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, p, nil))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s code=%d", p, rec.Code)
		}
		if strings.Contains(rec.Body.String(), "<!doctype") {
			t.Fatalf("%s served HTML", p)
		}
	}
}

func TestFilesHasIndex(t *testing.T) {
	t.Parallel()
	raw, err := fs.ReadFile(Files(), "index.html")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "LabNTP") {
		t.Fatalf("embedded index=%s", raw)
	}
}

func TestStubFiles(t *testing.T) {
	h := NewHandler(stubFS(t))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET / code=%d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "LabNTP") {
		t.Fatalf("body=%s", rec.Body.String())
	}
}

func stubFS(t *testing.T) fs.FS {
	t.Helper()
	sub, err := fs.Sub(stub, "stub")
	if err != nil {
		t.Fatal(err)
	}
	return sub
}

func TestRejectsTraversalAndNonGET(t *testing.T) {
	t.Parallel()
	h := NewHandler(testFS())

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/foo/../../etc/passwd", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("traversal code=%d body=%s", rec.Code, rec.Body.String())
	}

	post := httptest.NewRecorder()
	h.ServeHTTP(post, httptest.NewRequest(http.MethodPost, "/", nil))
	if post.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST / code=%d", post.Code)
	}
}

func TestHEADIndex(t *testing.T) {
	t.Parallel()
	h := NewHandler(testFS())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodHead, "/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("HEAD / code=%d", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("HEAD / leaked body len=%d", rec.Body.Len())
	}
}
