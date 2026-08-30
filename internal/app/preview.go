package app

import (
	"context"
	"net/netip"
	"time"

	"github.com/hilather/go-lab-ntp/internal/config"
	"github.com/hilather/go-lab-ntp/internal/domainerr"
	"github.com/hilather/go-lab-ntp/internal/querylog"
)

func (s *App) Preview(ctx context.Context, actor Actor, ip string) (*Preview, error) {
	if err := s.requireCtx(ctx); err != nil {
		return nil, err
	}
	_ = actor
	if ip == "" {
		return nil, domainerr.ValidationFailed("ip is required",
			domainerr.FieldViolation{Path: "ip", Code: "required", Message: "ip is required"})
	}
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return nil, domainerr.ValidationFailed("unparseable ip",
			domainerr.FieldViolation{Path: "ip", Code: "invalid_value", Message: "ip is not a valid address"})
	}
	addr = addr.Unmap()
	snap, err := s.active()
	if err != nil {
		return nil, err
	}
	host := s.clock.Now()
	out := &Preview{
		IP:       addr.String(),
		HostTime: host.UTC(),
	}
	if !snap.Allowed(addr) {
		out.Reason = "allowlist"
		return out, nil
	}
	f := snap.Match(addr)
	if f == nil {
		out.Reason = "unmatched"
		return out, nil
	}
	served := f.View.Served(host)
	st := served.UTC()
	out.Filter = f.Name
	out.ServedTime = &st
	out.Mode = f.View.Mode
	out.Leap = f.View.Leap
	out.Stratum = f.View.Stratum
	out.RefID = f.View.RefID
	out.OffsetFromHost = config.FormatDuration(st.Sub(out.HostTime))
	return out, nil
}

func (s *App) ListQueries(ctx context.Context, actor Actor, page Page) (*QueryList, error) {
	if err := s.requireCtx(ctx); err != nil {
		return nil, err
	}
	_ = actor
	if s.queryLog == nil {
		return &QueryList{}, nil
	}
	all := s.queryLog.List()
	limit := page.Limit
	if limit <= 0 || limit > 256 {
		limit = 64
	}
	start := 0
	if page.Cursor != "" {
		if n, err := time.ParseDuration(page.Cursor); err == nil {
			_ = n
		}
		for i, e := range all {
			if e.WhenHost.UTC().Format(time.RFC3339Nano) == page.Cursor {
				start = i + 1
				break
			}
		}
	}
	if start > len(all) {
		start = len(all)
	}
	end := start + limit
	if end > len(all) {
		end = len(all)
	}
	items := append([]querylog.Entry(nil), all[start:end]...)
	next := ""
	if end < len(all) && len(items) > 0 {
		next = items[len(items)-1].WhenHost.UTC().Format(time.RFC3339Nano)
	}
	return &QueryList{Items: items, NextCursor: next}, nil
}
