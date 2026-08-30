package main

import (
	"flag"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/hilather/go-lab-ntp/internal/ntpwire"
)

// queryCmd is a CLI-only SNTP client. It must not be imported by
// internal/ntpserver or the serve path.
func queryCmd(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("query", flag.ContinueOnError)
	fs.SetOutput(stderr)
	server := fs.String("server", "", "NTP server host:port")
	timeout := fs.Duration("timeout", 2*time.Second, "I/O timeout")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *server == "" {
		_, _ = fmt.Fprintln(stderr, "labntp query: --server is required")
		return 2
	}
	c, err := net.DialTimeout("udp", *server, *timeout)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "labntp query: %v\n", err)
		return 1
	}
	defer func() { _ = c.Close() }()
	_ = c.SetDeadline(time.Now().Add(*timeout))
	hostTime := time.Now()
	req := ntpwire.Encode(ntpwire.Packet{
		VN:      4,
		Mode:    ntpwire.ModeClient,
		XmtTime: ntpwire.FromTime(hostTime),
	})
	if _, err := c.Write(req); err != nil {
		_, _ = fmt.Fprintf(stderr, "labntp query: write: %v\n", err)
		return 1
	}
	buf := make([]byte, ntpwire.MaxUDPSize)
	n, err := c.Read(buf)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "labntp query: read: %v\n", err)
		return 1
	}
	p, err := ntpwire.Parse(buf[:n])
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "labntp query: parse: %v\n", err)
		return 1
	}
	org := p.OrgTime.Time(ntpwire.Era(hostTime))
	rec := p.RecTime.Time(ntpwire.Era(hostTime))
	xmt := p.XmtTime.Time(ntpwire.Era(hostTime))
	_, _ = fmt.Fprintf(stdout, "hostTime=%s originate=%s receive=%s transmit=%s vn=%d stratum=%d li=%d refid=%q\n",
		hostTime.UTC().Format(time.RFC3339Nano),
		org.Format(time.RFC3339Nano),
		rec.Format(time.RFC3339Nano),
		xmt.Format(time.RFC3339Nano),
		p.VN, p.Stratum, p.LI, refidString(p.RefID))
	return 0
}

func refidString(id [4]byte) string {
	n := 4
	for n > 0 && id[n-1] == 0 {
		n--
	}
	return string(id[:n])
}
