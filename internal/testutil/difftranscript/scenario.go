package difftranscript

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	protocolVersion = "2026-07-28"
	adminSecret     = "0123456789abcdef0123456789abcdef"
	operatorSecret  = "abcdef0123456789abcdef0123456789"
	viewerSecret    = "fedcba9876543210fedcba9876543210"
	normalBurst     = 10000
	rateBurst       = 3
	epiIP           = "127.0.0.2"
)

type toolSpec struct {
	name     string
	scope    string
	mutating bool
	idem     bool
}

// tools is testdata/mcp/goldens/tools.txt order.
var tools = []toolSpec{
	{"ntp_version_get", "ntp.read", false, false},
	{"ntp_capabilities_get", "ntp.read", false, false},
	{"ntp_status_get", "ntp.read", false, false},
	{"ntp_schema_get", "ntp.read", false, false},
	{"ntp_features_list", "ntp.read", false, false},
	{"ntp_state_get", "ntp.read", false, false},
	{"ntp_state_validate", "ntp.admin", false, false},
	{"ntp_state_export", "ntp.admin", false, false},
	{"ntp_state_reset", "ntp.admin", true, false},
	{"ntp_change_plan", "ntp.admin", false, false},
	{"ntp_change_apply", "ntp.admin", true, true},
	{"ntp_filters_list", "ntp.read", false, false},
	{"ntp_filters_get", "ntp.read", false, false},
	{"ntp_filters_put", "ntp.write", true, true},
	{"ntp_filters_delete", "ntp.write", true, true},
	{"ntp_views_preview", "ntp.read", false, false},
	{"ntp_queries_list", "ntp.read", false, false},
	{"ntp_audit_list", "ntp.audit.read", false, false},
	{"ntp_audit_get", "ntp.audit.read", false, false},
}

type lockedBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedBuf) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *lockedBuf) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

type proc struct {
	cmd     *exec.Cmd
	stdout  *lockedBuf
	stderr  *lockedBuf
	stopped bool
}

func (p *proc) stop() {
	if p == nil || p.cmd == nil || p.cmd.Process == nil || p.stopped {
		return
	}
	p.stopped = true
	_ = p.cmd.Process.Signal(syscall.SIGTERM)
	done := make(chan struct{})
	go func() {
		_ = p.cmd.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		_ = p.cmd.Process.Kill()
		<-done
	}
}

type response struct {
	status  int
	headers http.Header
	body    string
}

type harness struct {
	bin     string
	fix     string
	normal  string
	rate    string
	admin   string
	clients map[string]*http.Client
	b       strings.Builder
	addrs   []string
	metrics map[string]float64
	reqN    int
	srv     *proc
	baseURL string
}

func runScenario(binary, fixture, outPath string) error {
	if binary == "" || fixture == "" {
		return fmt.Errorf("binary and fixture are required")
	}
	if err := os.MkdirAll(fixture, 0o755); err != nil {
		return err
	}
	h := &harness{
		bin:     binary,
		fix:     fixture,
		normal:  filepath.Join(fixture, "normal"),
		rate:    filepath.Join(fixture, "rate"),
		clients: map[string]*http.Client{},
		metrics: map[string]float64{},
	}
	var restore func()
	defer func() {
		if h.srv != nil {
			h.srv.stop()
		}
		if restore != nil {
			restore()
		}
	}()
	if err := h.writeTree(h.normal, 32, normalBurst, "administrator"); err != nil {
		return err
	}
	if err := h.writeTree(h.rate, 1, rateBurst, "administrator"); err != nil {
		return err
	}
	h.admin = filepath.Join(h.normal, "admin.secret")
	orig, err := os.ReadFile(filepath.Join(h.normal, "labntp.yaml"))
	if err != nil {
		return err
	}
	restore = func() {
		_ = os.Chmod(h.admin, 0o600)
		_ = os.WriteFile(filepath.Join(h.normal, "labntp.yaml"), orig, 0o644)
	}

	if err := h.boot(h.normal, "normal", normalBurst); err != nil {
		return err
	}
	if err := h.block1(); err != nil {
		return err
	}
	if err := h.block2(); err != nil {
		return err
	}
	if err := h.block3(); err != nil {
		return err
	}
	stdio, err := h.block4()
	if stdio != nil {
		defer stdio.stop()
	}
	if err != nil {
		return err
	}
	stdio.stop()
	if err := os.WriteFile(filepath.Join(h.normal, "labntp.yaml"), orig, 0o644); err != nil {
		return err
	}
	if err := os.Chmod(h.admin, 0o600); err != nil {
		return err
	}
	h.srv.stop()
	h.srv = nil

	if err := h.boot(h.rate, "rate", rateBurst); err != nil {
		return err
	}
	if err := h.block5(); err != nil {
		return err
	}
	h.srv.stop()
	h.srv = nil
	h.metrics = map[string]float64{}

	if err := h.boot(h.normal, "normal-restart", normalBurst); err != nil {
		return err
	}
	if err := h.block6(); err != nil {
		return err
	}
	if err := h.block7(); err != nil {
		return err
	}
	h.b.WriteString("block 8 epilogue recorded after each block\n")

	out := Normalize([]byte(h.b.String()), h.addrs)
	if outPath == "" {
		_, err = os.Stdout.Write(out)
		return err
	}
	return os.WriteFile(outPath, out, 0o644)
}

func (h *harness) writeTree(dir string, rps, burst int, adminRole string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	secrets := map[string]string{
		"admin.secret":    adminSecret,
		"operator.secret": operatorSecret,
		"viewer.secret":   viewerSecret,
	}
	for name, secret := range secrets {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(secret+"\n"), 0o600); err != nil {
			return err
		}
	}
	yaml := fmt.Sprintf(`apiVersion: labntp.dev/v1alpha1
kind: LabNTP
metadata:
  name: difftranscript
spec:
  listeners:
    ntp:
      address: "127.0.0.1:9"
    management:
      address: "127.0.0.1:9"
      restPath: /v1
      mcpPath: /mcp
  auth:
    mode: bearer
    tokens:
      - id: admin
        role: %s
        secretFile: %s
      - id: operator
        role: operator
        secretFile: %s
      - id: viewer
        role: viewer
        secretFile: %s
  ui:
    enabled: false
  management:
    allowedOrigins: ["https://lab.example"]
    mcp:
      allowLegacyClients: false
    bodyLimit: 1MiB
    requestsPerSecond: %d
    burst: %d
    maxConcurrent: 256
  observability:
    metrics:
      publicPath: true
  ntp:
    allowClientCidrs: ["127.0.0.0/8", "::1/128"]
  filters:
    - name: default
      enabled: true
      match:
        cidrs: ["0.0.0.0/0", "::/0"]
      view:
        mode: follow-real
`, adminRole,
		filepath.Join(dir, "admin.secret"),
		filepath.Join(dir, "operator.secret"),
		filepath.Join(dir, "viewer.secret"),
		rps, burst)
	return os.WriteFile(filepath.Join(dir, "labntp.yaml"), []byte(yaml), 0o644)
}

// boot starts one instance. A lost pre-bound port (address already in use on
// stderr, or readiness failure) reserves new ports and reruns that instance
// once. The rerun is logged to stderr, not the transcript, so a rare retry
// cannot fail the self-diff. A second failure fails the harness.
func (h *harness) boot(dir, label string, burst int) error {
	err := h.bootOnce(dir, label, burst)
	if err == nil || !lostPreboundPort(err) {
		return err
	}
	fmt.Fprintf(os.Stderr, "difftranscript: rerun %s once after lost pre-bound port: %v\n", label, err)
	if err = h.bootOnce(dir, label, burst); err != nil {
		return fmt.Errorf("boot %s: rerun failed: %w", label, err)
	}
	return nil
}

func lostPreboundPort(err error) bool {
	msg := err.Error()
	if strings.Contains(msg, "address already in use") || strings.Contains(msg, "address in use") {
		return true
	}
	return strings.Contains(msg, "readiness:")
}

func (h *harness) bootOnce(dir, label string, burst int) error {
	if h.srv != nil {
		h.srv.stop()
		h.srv = nil
	}
	ntp, err := reserve("udp")
	if err != nil {
		return err
	}
	mgmt, err := reserve("tcp")
	if err != nil {
		return err
	}
	cfg := filepath.Join(dir, "labntp.yaml")
	cmd := exec.Command(h.bin, "serve", "--config", cfg, "--ntp-listen", ntp, "--management-listen", mgmt, "--shutdown-timeout", "2s")
	p := &proc{cmd: cmd, stdout: &lockedBuf{}, stderr: &lockedBuf{}}
	cmd.Stdout = p.stdout
	cmd.Stderr = p.stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	h.srv = p
	deadline := time.Now().Add(15 * time.Second)
	var ntpBound, mgmtEcho string
	for time.Now().Before(deadline) {
		text := p.stdout.String()
		ntpBound = listenValue(text, "labntp ntp listen=")
		mgmtEcho = listenValue(text, "labntp management listen=")
		if ntpBound != "" && mgmtEcho != "" {
			break
		}
		if p.cmd.ProcessState != nil && p.cmd.ProcessState.Exited() {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if mgmtEcho == "" || ntpBound == "" {
		return fmt.Errorf("boot %s: ntp=%q management=%q stderr=%s", label, ntpBound, mgmtEcho, p.stderr.String())
	}
	if mgmtEcho != mgmt {
		return fmt.Errorf("boot %s: management log %q, flag %q stderr=%s", label, mgmtEcho, mgmt, p.stderr.String())
	}
	h.baseURL = "http://" + mgmt
	h.metrics = map[string]float64{}
	if err := h.ready(); err != nil {
		return fmt.Errorf("boot %s: %w stderr=%s", label, err, p.stderr.String())
	}
	h.addrs = append(h.addrs, ntpBound, mgmtEcho)
	if _, err := h.scrape(true); err != nil {
		return err
	}
	fmt.Fprintf(&h.b, "startup fixture=%s burst=%d management=%s ntp=%s\n", label, burst, mgmtEcho, ntpBound)
	h.b.WriteString("baseline scrape recorded\n")
	return nil
}

func reserve(network string) (string, error) {
	if network == "udp" {
		ln, err := net.ListenPacket("udp", "127.0.0.1:0")
		if err != nil {
			return "", err
		}
		addr := ln.LocalAddr().String()
		return addr, ln.Close()
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	addr := ln.Addr().String()
	return addr, ln.Close()
}

func listenValue(text, prefix string) string {
	for _, line := range strings.Split(text, "\n") {
		if rest, ok := strings.CutPrefix(line, prefix); ok {
			return strings.TrimSpace(rest)
		}
	}
	return ""
}

func (h *harness) ready() error {
	deadline := time.Now().Add(10 * time.Second)
	var last string
	for time.Now().Before(deadline) {
		live, err := h.do(epiIP, http.MethodGet, "/v1/health/live", "", nil)
		if err == nil && live.status == 200 {
			ready, err := h.do(epiIP, http.MethodGet, "/v1/health/ready", "", nil)
			if err == nil && ready.status == 200 {
				return nil
			}
			if err != nil {
				last = err.Error()
			} else {
				last = ready.body
			}
		} else if err != nil {
			last = err.Error()
		} else {
			last = live.body
		}
		time.Sleep(30 * time.Millisecond)
	}
	return fmt.Errorf("readiness: %s", last)
}

func (h *harness) client(ip string) *http.Client {
	if c, ok := h.clients[ip]; ok {
		return c
	}
	dialer := &net.Dialer{Timeout: 5 * time.Second, LocalAddr: &net.TCPAddr{IP: net.ParseIP(ip)}}
	c := &http.Client{
		Timeout: 8 * time.Second,
		Transport: &http.Transport{
			DialContext:       dialer.DialContext,
			ForceAttemptHTTP2: false,
			TLSNextProto:      map[string]func(string, *tls.Conn) http.RoundTripper{},
		},
	}
	h.clients[ip] = c
	return c
}

func (h *harness) do(ip, method, path, body string, hdr map[string]string) (response, error) {
	req, err := http.NewRequest(method, h.baseURL+path, strings.NewReader(body))
	if err != nil {
		return response{}, err
	}
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	res, err := h.client(ip).Do(req)
	if err != nil {
		return response{}, err
	}
	defer func() { _ = res.Body.Close() }()
	b, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if err != nil {
		return response{}, err
	}
	return response{status: res.StatusCode, headers: res.Header.Clone(), body: string(b)}, nil
}

func (h *harness) record(block int, ip, method, path, body string, hdr map[string]string) (response, error) {
	h.reqN++
	hdrCopy := map[string]string{}
	for k, v := range hdr {
		hdrCopy[k] = v
	}
	hdrCopy["X-Request-ID"] = fmt.Sprintf("req-%d-%d", block, h.reqN)
	if body != "" && hdrCopy["Content-Type"] == "" {
		hdrCopy["Content-Type"] = "application/json"
	}
	res, err := h.do(ip, method, path, body, hdrCopy)
	if err != nil {
		return res, err
	}
	fmt.Fprintf(&h.b, "-- req block=%d n=%d group=%s method=%s path=%s status=%d\n", block, h.reqN, ip, method, path, res.status)
	for _, key := range []string{"Content-Type", "WWW-Authenticate", "Cache-Control", "Allow", "X-LabNTP-Revision", "X-Request-ID"} {
		if v := res.headers.Get(key); v != "" {
			fmt.Fprintf(&h.b, "%s: %s\n", key, v)
		}
	}
	for _, v := range res.headers.Values("Set-Cookie") {
		fmt.Fprintf(&h.b, "Set-Cookie: %s\n", v)
	}
	h.b.WriteString(res.body)
	if !strings.HasSuffix(res.body, "\n") {
		h.b.WriteByte('\n')
	}
	h.b.WriteString("-- end req\n")
	return res, nil
}

func bearer(secret string) map[string]string {
	return map[string]string{"Authorization": "Bearer " + secret}
}

func (h *harness) with(base map[string]string, extra map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range base {
		out[k] = v
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}

func (h *harness) begin(n int) time.Time {
	fmt.Fprintf(&h.b, "block %d\n", n)
	return time.Now()
}

func (h *harness) end(n int, start time.Time) error {
	if time.Since(start) >= 60*time.Second {
		return fmt.Errorf("block %d exceeded 60s", n)
	}
	return h.epilogue(n)
}

func (h *harness) epilogue(block int) error {
	res, err := h.record(block, epiIP, http.MethodGet, "/v1/audit?limit=100", "", bearer(adminSecret))
	if err != nil {
		return err
	}
	fmt.Fprintf(&h.b, "audit block=%d status=%d\n", block, res.status)
	deltas, err := h.scrape(false)
	if err != nil {
		return err
	}
	fmt.Fprintf(&h.b, "metrics block=%d\n", block)
	for _, line := range deltas {
		h.b.WriteString(line)
		h.b.WriteByte('\n')
	}
	h.b.WriteString("-- end metrics\n")
	return nil
}

func (h *harness) scrape(baseline bool) ([]string, error) {
	res, err := h.do(epiIP, http.MethodGet, "/v1/metrics", "", bearer(adminSecret))
	if err != nil {
		return nil, err
	}
	cur := parseMetrics(res.body)
	if baseline {
		h.metrics = cur
		return nil, nil
	}
	var lines []string
	for key, val := range cur {
		delta := val - h.metrics[key]
		lines = append(lines, key+" "+strconv.FormatFloat(delta, 'f', -1, 64))
	}
	sort.Strings(lines)
	h.metrics = cur
	return lines, nil
}

func parseMetrics(text string) map[string]float64 {
	out := map[string]float64{}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		name, rest, ok := strings.Cut(line, " ")
		if !ok {
			name, rest, ok = strings.Cut(line, "\t")
			if !ok {
				continue
			}
		}
		if strings.HasPrefix(name, "labntp_") && (strings.Contains(name, "_sum") || strings.Contains(name, "_bucket") || strings.HasPrefix(name, "labntp_udp_inflight")) {
			continue
		}
		if !strings.HasPrefix(name, "labntp_") {
			continue
		}
		key := canonicalMetric(name)
		val, err := strconv.ParseFloat(strings.Fields(rest)[0], 64)
		if err != nil {
			continue
		}
		out[key] = val
	}
	return out
}

func canonicalMetric(spec string) string {
	name, labels, ok := strings.Cut(spec, "{")
	if !ok {
		return name
	}
	labels = strings.TrimSuffix(labels, "}")
	if labels == "" {
		return name
	}
	parts := strings.Split(labels, ",")
	sort.Strings(parts)
	return name + "{" + strings.Join(parts, ",") + "}"
}

func (h *harness) revision(block int, ip string) (string, error) {
	res, err := h.record(block, ip, http.MethodGet, "/v1/state", "", bearer(adminSecret))
	if err != nil {
		return "", err
	}
	return jsonString(res.body, "runtimeRevision"), nil
}

func jsonString(body, key string) string {
	var doc map[string]any
	if err := json.Unmarshal([]byte(extractJSON(body)), &doc); err != nil {
		return ""
	}
	s, _ := doc[key].(string)
	return s
}

func extractJSON(body string) string {
	trim := strings.TrimSpace(body)
	if strings.HasPrefix(trim, "{") || strings.HasPrefix(trim, "[") {
		return trim
	}
	var b strings.Builder
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimRight(line, "\r")
		if rest, ok := strings.CutPrefix(line, "data:"); ok {
			b.WriteString(strings.TrimPrefix(rest, " "))
		}
	}
	return b.String()
}

func applyBody(rev, key, reason, def string) string {
	return fmt.Sprintf(`{"expectedRevision":%q,"idempotencyKey":%q,"reason":%q,"operations":[{"op":"replaceRestrict","restrict":{"default":%q,"kod":true}}]}`, rev, key, reason, def)
}

func (h *harness) block1() error {
	start := h.begin(1)
	ip := "127.0.0.10"
	rev, err := h.revision(1, ip)
	if err != nil {
		return fmt.Errorf("block 1: %w", err)
	}
	if _, err := h.record(1, ip, http.MethodPost, "/v1/changes:apply", applyBody(rev, "idem-block1", "harness", "limited"), bearer(adminSecret)); err != nil {
		return err
	}
	if _, err := h.record(1, ip, http.MethodGet, "/v1/state", "", bearer("not-a-token")); err != nil {
		return err
	}
	if _, err := h.record(1, ip, http.MethodGet, "/v1/state", "", nil); err != nil {
		return err
	}
	if _, err := h.record(1, ip, http.MethodPost, "/v1/changes:apply", applyBody(rev, "idem-none", "harness", "limited"), nil); err != nil {
		return err
	}
	basic := map[string]string{"Authorization": "Basic dXNlcjpwYXNz"}
	if _, err := h.record(1, ip, http.MethodGet, "/v1/state", "", basic); err != nil {
		return err
	}
	if _, err := h.record(1, ip, http.MethodGet, "/v1/state", "", map[string]string{"Authorization": "Token abc"}); err != nil {
		return err
	}
	if _, err := h.mcp(1, ip, "ntp_version_get", map[string]any{}, basic); err != nil {
		return err
	}
	if _, err := h.mcp(1, ip, "ntp_version_get", map[string]any{}, map[string]string{"Authorization": "Token abc"}); err != nil {
		return err
	}
	return h.end(1, start)
}

func (h *harness) mcp(block int, ip, tool string, args any, hdr map[string]string) (response, error) {
	params := map[string]any{
		"name":      tool,
		"arguments": args,
		"_meta": map[string]any{
			"io.modelcontextprotocol/protocolVersion":    protocolVersion,
			"io.modelcontextprotocol/clientInfo":         map[string]any{"name": "difftranscript", "version": "dev"},
			"io.modelcontextprotocol/clientCapabilities": map[string]any{},
		},
	}
	raw, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": params})
	if err != nil {
		return response{}, err
	}
	hdr = h.with(hdr, map[string]string{
		"Content-Type":         "application/json",
		"Accept":               "application/json, text/event-stream",
		"Mcp-Protocol-Version": protocolVersion,
		"Mcp-Method":           "tools/call",
		"Mcp-Name":             tool,
	})
	return h.record(block, ip, http.MethodPost, "/mcp", string(raw), hdr)
}

func (h *harness) block2() error {
	start := h.begin(2)
	ip := "127.0.0.11"
	res, err := h.record(2, ip, http.MethodPost, "/v1/session", "", bearer(adminSecret))
	if err != nil {
		return err
	}
	cookie := cookieValue(res.headers.Values("Set-Cookie"))
	csrf := jsonString(res.body, "csrf")
	ck := map[string]string{"Cookie": "labntp_session=" + cookie}
	if _, err = h.record(2, ip, http.MethodGet, "/v1/session", "", ck); err != nil {
		return err
	}
	rev, err := h.revision(2, ip)
	if err != nil {
		return err
	}
	body := applyBody(rev, "idem-cookie", "harness", "serve")
	if _, err = h.record(2, ip, http.MethodPost, "/v1/changes:apply", body, ck); err != nil {
		return err
	}
	if _, err = h.record(2, ip, http.MethodPost, "/v1/changes:apply", body, h.with(ck, map[string]string{"X-LabNTP-CSRF": "deadbeef"})); err != nil {
		return err
	}
	if _, err = h.record(2, ip, http.MethodPost, "/v1/changes:apply", body, h.with(ck, map[string]string{"X-LabNTP-CSRF": csrf})); err != nil {
		return err
	}
	if _, err = h.record(2, ip, http.MethodDelete, "/v1/session", "", h.with(ck, map[string]string{"X-LabNTP-CSRF": csrf})); err != nil {
		return err
	}
	if _, err = h.record(2, ip, http.MethodPost, "/v1/changes:apply", body, ck); err != nil {
		return err
	}
	return h.end(2, start)
}

func cookieValue(set []string) string {
	for _, line := range set {
		parts := strings.Split(line, ";")
		nv := strings.SplitN(strings.TrimSpace(parts[0]), "=", 2)
		if len(nv) == 2 && nv[0] == "labntp_session" {
			return nv[1]
		}
	}
	return ""
}

func (h *harness) block3() error {
	start := h.begin(3)
	ip := "127.0.0.12"
	// Empty host ("http://") hits the parse-error branch. Every detail is recorded.
	for _, origin := range []string{
		"https://evil.example",
		"https://lab.example",
		"",
		"file:///tmp/x",
		"http://127.0.0.1:8080",
		"http://10.1.2.3",
		"http://",
	} {
		hdr := bearer(adminSecret)
		if origin != "" {
			hdr = h.with(hdr, map[string]string{"Origin": origin})
		}
		if _, err := h.record(3, ip, http.MethodGet, "/v1/state", "", hdr); err != nil {
			return err
		}
		if _, err := h.mcp(3, ip, "ntp_version_get", map[string]any{}, hdr); err != nil {
			return err
		}
	}
	return h.end(3, start)
}

func (h *harness) block4() (*stdioProc, error) {
	start := h.begin(4)
	ip := "127.0.0.13"
	res, err := h.record(4, ip, http.MethodPost, "/v1/session", "", bearer(adminSecret))
	if err != nil {
		return nil, err
	}
	cookie := cookieValue(res.headers.Values("Set-Cookie"))
	sp, err := h.startStdio()
	if err != nil {
		return sp, err
	}
	// While the live verifier is still administrator, an unreadable secret
	// reaches preflightAuth. After demotion it does not (see below).
	if _, err := h.revision(4, ip); err != nil {
		return sp, err
	}
	if err := os.Chmod(h.admin, 0); err != nil {
		return sp, err
	}
	if _, err := h.record(4, ip, http.MethodPost, "/v1/state:reset", `{"reason":"unreadable-admin"}`, bearer(adminSecret)); err != nil {
		return sp, err
	}
	if _, err := h.revision(4, ip); err != nil {
		return sp, err
	}
	stdioFail, err := sp.call("ntp_state_reset", map[string]any{"reason": "unreadable-admin"})
	if err != nil {
		return sp, err
	}
	h.writeStdio(4, "ntp_state_reset-unreadable-admin", stdioFail)
	stdioRead, err := sp.call("ntp_version_get", map[string]any{})
	if err != nil {
		return sp, err
	}
	h.writeStdio(4, "ntp_version_get-after-unreadable-admin", stdioRead)
	if err := os.Chmod(h.admin, 0o600); err != nil {
		return sp, err
	}
	yamlPath := filepath.Join(h.normal, "labntp.yaml")
	raw, err := os.ReadFile(yamlPath)
	if err != nil {
		return sp, err
	}
	swapped := strings.Replace(string(raw), "role: administrator", "role: viewer", 1)
	if swapped == string(raw) {
		return sp, fmt.Errorf("block 4: administrator role not found")
	}
	if err := os.WriteFile(yamlPath, []byte(swapped), 0o644); err != nil {
		return sp, err
	}
	if _, err := h.record(4, ip, http.MethodPost, "/v1/state:reset", `{"reason":"demote"}`, bearer(adminSecret)); err != nil {
		return sp, err
	}
	if _, err := h.record(4, ip, http.MethodGet, "/v1/state", "", map[string]string{"Cookie": "labntp_session=" + cookie}); err != nil {
		return sp, err
	}
	if _, err := h.record(4, ip, http.MethodGet, "/v1/session", "", bearer(adminSecret)); err != nil {
		return sp, err
	}
	rev, err := h.revision(4, ip)
	if err != nil {
		return sp, err
	}
	if _, err := h.record(4, ip, http.MethodPost, "/v1/changes:apply", applyBody(rev, "idem-demote", "harness", "limited"), bearer(adminSecret)); err != nil {
		return sp, err
	}
	if err := h.recordStdioDemotion(sp); err != nil {
		return sp, err
	}
	if _, err := h.revision(4, ip); err != nil {
		return sp, err
	}
	if err := os.Chmod(h.admin, 0); err != nil {
		return sp, err
	}
	if _, err := h.record(4, ip, http.MethodPost, "/v1/state:reset", `{"reason":"unreadable"}`, bearer(adminSecret)); err != nil {
		return sp, err
	}
	// The live verifier is already viewer, so authorize rejects the reset
	// before preflightAuth reads the secret file. The response is recorded.
	if _, err := h.revision(4, ip); err != nil {
		return sp, err
	}
	failRes, err := sp.call("ntp_state_reset", map[string]any{"reason": "unreadable"})
	if err != nil {
		return sp, err
	}
	h.writeStdio(4, "ntp_state_reset-unreadable", failRes)
	readRes, err := sp.call("ntp_version_get", map[string]any{})
	if err != nil {
		return sp, err
	}
	h.writeStdio(4, "ntp_version_get-after-unreadable", readRes)
	if err := os.Chmod(h.admin, 0o600); err != nil {
		return sp, err
	}
	return sp, h.end(4, start)
}

func (h *harness) recordStdioDemotion(sp *stdioProc) error {
	res, err := sp.call("ntp_state_reset", map[string]any{"reason": "stdio-demote"})
	if err != nil {
		return err
	}
	h.writeStdio(4, "ntp_state_reset", res)
	res, err = sp.call("ntp_version_get", map[string]any{})
	if err != nil {
		return err
	}
	h.writeStdio(4, "ntp_version_get", res)
	res, err = sp.call("ntp_state_export", map[string]any{"format": "yaml"})
	if err != nil {
		return err
	}
	h.writeStdio(4, "ntp_state_export", res)
	res, err = sp.call("ntp_filters_put", toolArgs("ntp_filters_put", "sha256:0000000000000000000000000000000000000000000000000000000000000000", "idem-stdio", ""))
	if err != nil {
		return err
	}
	h.writeStdio(4, "ntp_filters_put", res)
	res, err = sp.call("ntp_not_a_tool", map[string]any{})
	if err != nil {
		return err
	}
	h.writeStdio(4, "ntp_not_a_tool", res)
	return nil
}

func (h *harness) writeStdio(block int, name, body string) {
	fmt.Fprintf(&h.b, "-- stdio block=%d tool=%s\n%s\n-- end stdio\n", block, name, body)
}

func (h *harness) block5() error {
	start := h.begin(5)
	// REST series, one source, burst+1. /mcp is mounted on the REST server.
	// dispatchMount calls the REST limiter before the MCP handler, so this
	// fourth call is that limiter's problem+json 429 (rate_limited, "too many
	// management requests"). The MCP limiter is not consulted.
	restIP := "127.0.0.14"
	series := time.Now()
	for i := 0; i < rateBurst+1; i++ {
		if _, err := h.record(5, restIP, http.MethodGet, "/v1/version", "", bearer(adminSecret)); err != nil {
			return err
		}
	}
	if time.Since(series) >= time.Second {
		return fmt.Errorf("block 5 REST series took %s", time.Since(series))
	}

	// The MCP limiter is built once (mcp.New → newLimiter) and has no setRate.
	// reloadAuth only replaces the verifier. Reset rereads a bootstrap whose
	// burst is high enough that ApplyLimits/setRate raises the live REST
	// limiter and it stops denying. setRate does not refill an existing
	// bucket, so the reset uses a source that still has REST tokens.
	// The MCP limiter keeps the startup burst of 3.
	if err := h.writeTree(h.rate, 1, normalBurst, "administrator"); err != nil {
		return err
	}
	if _, err := h.record(5, "127.0.0.18", http.MethodPost, "/v1/state:reset", `{"reason":"raise-rest-burst"}`, bearer(adminSecret)); err != nil {
		return err
	}

	// MCP series from a fresh source, burst+1, own 1s budget. REST admits
	// each call: the source's bucket is created after setRate, at the raised
	// burst. The fourth call is the MCP limiter's JSON-RPC -32005.
	mcpIP := "127.0.0.19"
	series = time.Now()
	for i := 0; i < rateBurst+1; i++ {
		if _, err := h.mcp(5, mcpIP, "ntp_version_get", map[string]any{}, bearer(adminSecret)); err != nil {
			return err
		}
	}
	if time.Since(series) >= time.Second {
		return fmt.Errorf("block 5 MCP series took %s", time.Since(series))
	}
	return h.end(5, start)
}

func (h *harness) block6() error {
	start := h.begin(6)
	ip := "127.0.0.15"
	rev, err := h.revision(6, ip)
	if err != nil {
		return err
	}
	body := applyBody(rev, "idem-replay", "same", "limited")
	if _, err := h.record(6, ip, http.MethodPost, "/v1/changes:apply", body, bearer(adminSecret)); err != nil {
		return err
	}
	replayBody := applyBody("sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "idem-replay", "same", "limited")
	if _, err := h.record(6, ip, http.MethodPost, "/v1/changes:apply", replayBody, bearer(adminSecret)); err != nil {
		return err
	}
	conflict := applyBody(rev, "idem-replay", "same", "serve")
	if _, err := h.record(6, ip, http.MethodPost, "/v1/changes:apply", conflict, bearer(adminSecret)); err != nil {
		return err
	}
	return h.end(6, start)
}

func (h *harness) block7() error {
	start := h.begin(7)
	ip := "127.0.0.16"
	var auditID string
	for _, tool := range tools {
		rev, err := h.revision(7, ip)
		if err != nil {
			return err
		}
		args := toolArgs(tool.name, rev, "idem-"+tool.name, auditID)
		view, err := h.mcp(7, ip, tool.name, args, bearer(viewerSecret))
		if err != nil {
			return err
		}
		if tool.name == "ntp_audit_list" {
			if id := firstAudit(view.body); id != "" {
				auditID = id
			}
		}
		if tool.mutating {
			if _, err := h.record(7, ip, http.MethodPost, "/v1/state:reset", `{"reason":"before-`+tool.name+`"}`, bearer(adminSecret)); err != nil {
				return err
			}
			rev, err = h.revision(7, ip)
			if err != nil {
				return err
			}
			args = toolArgs(tool.name, rev, "idem-admin-"+tool.name, auditID)
		}
		admin, err := h.mcp(7, ip, tool.name, args, bearer(adminSecret))
		if err != nil {
			return err
		}
		if tool.name == "ntp_audit_list" {
			if id := firstAudit(admin.body); id != "" {
				auditID = id
			}
		}
	}
	return h.end(7, start)
}

func firstAudit(body string) string {
	i := strings.Index(body, "aud-")
	if i < 0 {
		return ""
	}
	j := i + 4
	for j < len(body) && body[j] >= '0' && body[j] <= '9' {
		j++
	}
	return body[i:j]
}

func toolArgs(name, rev, idem, auditID string) any {
	switch name {
	case "ntp_filters_get":
		return map[string]any{"name": "default"}
	case "ntp_filters_put":
		return map[string]any{
			"name":             "extra",
			"expectedRevision": rev,
			"idempotencyKey":   idem,
			"reason":           "harness",
			"filter": map[string]any{
				"name":    "extra",
				"enabled": true,
				"match":   map[string]any{"cidrs": []string{"10.9.9.9/32"}},
				"view":    map[string]any{"mode": "follow-real", "leap": "none", "stratum": 2, "refid": "LOCL"},
			},
		}
	case "ntp_filters_delete":
		return map[string]any{"name": "extra", "expectedRevision": rev, "idempotencyKey": idem, "reason": "harness"}
	case "ntp_views_preview":
		return map[string]any{"ip": "203.0.113.5"}
	case "ntp_state_export":
		return map[string]any{"format": "yaml"}
	case "ntp_state_reset":
		return map[string]any{"reason": "harness"}
	case "ntp_change_plan":
		return map[string]any{
			"expectedRevision": rev,
			"reason":           "harness",
			"operations":       []any{map[string]any{"op": "replaceRestrict", "restrict": map[string]any{"default": "limited", "kod": true}}},
		}
	case "ntp_change_apply":
		return map[string]any{
			"expectedRevision": rev,
			"idempotencyKey":   idem,
			"reason":           "harness",
			"operations":       []any{map[string]any{"op": "replaceRestrict", "restrict": map[string]any{"default": "limited", "kod": true}}},
		}
	case "ntp_audit_get":
		if auditID == "" {
			auditID = "aud-1"
		}
		return map[string]any{"id": auditID}
	default:
		return map[string]any{}
	}
}

type stdioProc struct {
	proc   *proc
	in     io.WriteCloser
	lines  chan string
	nextID int
}

func (h *harness) startStdio() (*stdioProc, error) {
	cmd := exec.Command(h.bin, "mcp-stdio", "--config", filepath.Join(h.normal, "labntp.yaml"), "--token-file", h.admin)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	p := &proc{cmd: cmd, stdout: &lockedBuf{}, stderr: &lockedBuf{}}
	cmd.Stderr = p.stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	sp := &stdioProc{proc: p, in: stdin, lines: make(chan string, 32), nextID: 1}
	go func() {
		sc := bufio.NewScanner(stdout)
		sc.Buffer(make([]byte, 64*1024), 8<<20)
		for sc.Scan() {
			sp.lines <- sc.Text()
		}
		close(sp.lines)
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := sp.round(ctx, "initialize", map[string]any{
		"protocolVersion": protocolVersion,
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "difftranscript", "version": "dev"},
	}); err != nil {
		p.stop()
		return nil, fmt.Errorf("stdio initialize: %w stderr=%s", err, p.stderr.String())
	}
	note, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized"})
	if err != nil {
		p.stop()
		return nil, err
	}
	if _, err := stdin.Write(append(note, '\n')); err != nil {
		p.stop()
		return nil, err
	}
	return sp, nil
}

func (s *stdioProc) stop() {
	if s == nil {
		return
	}
	if s.in != nil {
		_ = s.in.Close()
	}
	s.proc.stop()
}

func (s *stdioProc) call(name string, args any) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	return s.round(ctx, "tools/call", map[string]any{"name": name, "arguments": args})
}

func (s *stdioProc) round(ctx context.Context, method string, params any) (string, error) {
	s.nextID++
	id := s.nextID
	raw, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params})
	if err != nil {
		return "", err
	}
	if _, err := s.in.Write(append(raw, '\n')); err != nil {
		return "", err
	}
	for {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("%s id %d: %w stderr=%s", method, id, ctx.Err(), s.proc.stderr.String())
		case line, ok := <-s.lines:
			if !ok {
				return "", fmt.Errorf("%s: stdio closed stderr=%s", method, s.proc.stderr.String())
			}
			var doc struct {
				ID any `json:"id"`
			}
			if err := json.Unmarshal([]byte(line), &doc); err != nil || doc.ID == nil {
				continue
			}
			if fmt.Sprint(doc.ID) == strconv.Itoa(id) {
				return line, nil
			}
		}
	}
}
