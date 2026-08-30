package ntpserver

import (
	"net"
	"time"

	"github.com/hilather/go-lab-ntp/internal/model"
	"github.com/hilather/go-lab-ntp/internal/ntpkeys"
	"github.com/hilather/go-lab-ntp/internal/ntpview"
	"github.com/hilather/go-lab-ntp/internal/ntpwire"
	"github.com/hilather/go-lab-ntp/internal/querylog"
)

func vnLabel(vn uint8) string {
	switch vn {
	case 3:
		return "3"
	case 4:
		return "4"
	default:
		return "other"
	}
}

func (s *Server) handle(pkt []byte, addr net.Addr, tRecv time.Time) {
	if len(pkt) > s.cfg.MaxUDPSize {
		s.Oversize.Add(1)
		s.observe("oversize", "other")
		return
	}
	if len(pkt) < ntpwire.PacketSize {
		s.Short.Add(1)
		s.observe("short", "other")
		return
	}
	snap := s.cfg.Store.Load()
	if snap == nil {
		return
	}
	s.syncAdmission(snap)

	header := pkt
	var key *ntpkeys.Key
	if snap.Keys != nil && len(snap.Keys.ByID) > 0 {
		h, kid, dig, ok := ntpwire.SplitMAC(pkt)
		if !ok || len(dig) == 0 {
			return
		}
		k, found := snap.Keys.ByID[kid]
		if !found {
			return
		}
		if !ntpwire.VerifyMAC(k.Alg, k.Secret, pkt) {
			return
		}
		header = h
		cp := k
		key = &cp
	} else if len(pkt) > ntpwire.PacketSize {
		header = pkt[:ntpwire.PacketSize]
	}

	p, err := ntpwire.Parse(header)
	if err != nil {
		s.Short.Add(1)
		s.observe("short", "other")
		return
	}
	vn := vnLabel(p.VN)
	if !snap.VersionOK(p.VN) {
		s.Version.Add(1)
		s.observe("version", vn)
		return
	}
	if p.Mode != ntpwire.ModeClient {
		s.Mode.Add(1)
		s.observe("mode", vn)
		return
	}
	if p.XmtTime.Zero() {
		s.ZeroXmit.Add(1)
		s.observe("zero_xmit", vn)
		return
	}

	ip := peerFromAddr(addr)
	if !snap.Allowed(ip) {
		s.Allowlist.Add(1)
		s.observe("allowlist", vn)
		return
	}

	if !s.global.allow("global") || !s.perIP.allow(ip.String()) {
		s.Admission.Add(1)
		s.observe("admission", vn)
		return
	}

	switch snap.RestrictDefault {
	case model.RestrictIgnore:
		s.Ignore.Add(1)
		s.observe("ignore", vn)
		return
	case model.RestrictLimited:
		if !s.limited.allow(ip.String()) {
			if snap.KoD {
				s.replyKoD(p, addr, key)
			} else {
				s.observe("drop", vn)
			}
			return
		}
	}

	f := snap.Match(ip)
	if f == nil {
		s.Unmatched.Add(1)
		s.observe("unmatched", vn)
		return
	}

	hostSec := tRecv.Unix()
	jit := f.View.JitterDelta(hostSec)
	tRecvVirt := f.View.Served(tRecv).Add(jit)
	tXmit := s.cfg.Clock.Now()
	tXmitVirt := f.View.Served(tXmit).Add(jit)
	ref := f.View.RefTime(tXmitVirt).Add(jit)

	reply := ntpwire.Packet{
		LI:        ntpview.LeapLI(f.View.Leap),
		VN:        p.VN,
		Mode:      ntpwire.ModeServer,
		Stratum:   uint8(f.View.Stratum),
		Poll:      f.View.ClampPoll(p.Poll),
		Precision: int8(f.View.Precision),
		RootDelay: ntpwire.ShortFromDuration(f.View.RootDelay),
		RootDisp:  ntpwire.UShortFromDuration(f.View.RootDispersion),
		RefID:     ntpview.EncodeRefID(f.View.RefID),
		RefTime:   ntpwire.FromTime(ref),
		OrgTime:   p.XmtTime,
		RecTime:   ntpwire.FromTime(tRecvVirt),
		XmtTime:   ntpwire.FromTime(tXmitVirt),
	}
	out := ntpwire.Encode(reply)
	if key != nil {
		d, err := ntpwire.MAC(key.Alg, key.Secret, out)
		if err == nil {
			out = ntpwire.AppendMAC(out, key.ID, d)
		}
	}
	pc := s.conn()
	if pc != nil {
		_, _ = pc.WriteTo(out, addr)
	}
	s.Serve.Add(1)
	s.observe("serve", vnLabel(p.VN))
	s.observeFilter(f.Name)
	if s.cfg.Log != nil {
		if !s.cfg.Log.TryInsert(querylog.Entry{
			ClientIP:   ip.String(),
			Filter:     f.Name,
			ServedTime: tXmitVirt,
			Leap:       f.View.Leap,
			Mode:       f.View.Mode,
			VN:         p.VN,
			WhenHost:   tXmit,
		}) {
			s.observeQuerylogDrop()
		}
	}
}

func (s *Server) replyKoD(req ntpwire.Packet, addr net.Addr, key *ntpkeys.Key) {
	s.KoD.Add(1)
	s.observe("kod", vnLabel(req.VN))
	out := ntpwire.Encode(ntpwire.KoD(req, ntpwire.KissRATE))
	if key != nil {
		d, err := ntpwire.MAC(key.Alg, key.Secret, out)
		if err == nil {
			out = ntpwire.AppendMAC(out, key.ID, d)
		}
	}
	pc := s.conn()
	if pc != nil {
		_, _ = pc.WriteTo(out, addr)
	}
	if s.cfg.Log != nil {
		s.cfg.Log.TryInsert(querylog.Entry{
			Filter:   "",
			Mode:     "kod",
			Leap:     model.LeapUnsync,
			VN:       req.VN,
			WhenHost: s.cfg.Clock.Now(),
		})
	}
}
