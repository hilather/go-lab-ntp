package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/hilather/go-lab-ntp/internal/app"
	"github.com/hilather/go-lab-ntp/internal/auth"
	"github.com/hilather/go-lab-ntp/internal/compiler"
	"github.com/hilather/go-lab-ntp/internal/config"
	"github.com/hilather/go-lab-ntp/internal/control/mcp"
	"github.com/hilather/go-lab-ntp/internal/control/rest"
	"github.com/hilather/go-lab-ntp/internal/ntpserver"
	"github.com/hilather/go-lab-ntp/internal/observability"
	"github.com/hilather/go-lab-ntp/internal/querylog"
	"github.com/hilather/go-lab-ntp/internal/snapshot"
)

type serveFlags struct {
	Config           string
	NTPListen        string
	ManagementListen string
	ShutdownTimeout  time.Duration
	PIDFile          string
}

func parseServeFlags(args []string, stderr io.Writer) (serveFlags, error) {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(stderr)
	path := fs.String("config", "", "path to bootstrap YAML or JSON")
	ntpListen := fs.String("ntp-listen", "", "override NTP listen address (empty uses YAML)")
	mgmtListen := fs.String("management-listen", "off", "override management listen address; off/none/- leaves it unbound")
	shutdown := fs.Duration("shutdown-timeout", ntpserver.DefaultShutdownWait, "graceful shutdown deadline")
	pidFile := fs.String("pid-file", "", "write process id after listeners bind")
	if err := fs.Parse(args); err != nil {
		return serveFlags{}, err
	}
	if *path == "" {
		_, _ = fmt.Fprintln(stderr, "labntp serve: --config is required")
		return serveFlags{}, fmt.Errorf("missing --config")
	}
	return serveFlags{
		Config:           *path,
		NTPListen:        *ntpListen,
		ManagementListen: *mgmtListen,
		ShutdownTimeout:  *shutdown,
		PIDFile:          *pidFile,
	}, nil
}

func managementOff(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "off", "none", "-":
		return true
	default:
		return false
	}
}

func serveCmd(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags, err := parseServeFlags(args, stderr)
	if err != nil {
		return 2
	}
	st, warns, err := config.LoadFileWithWarnings(flags.Config)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "labntp serve: %v\n", err)
		return 1
	}
	for _, w := range warns {
		_, _ = fmt.Fprintf(stderr, "warning %s: %s\n", w.Path, w.Message)
	}
	snap, err := compiler.Compile(st, compiler.CompileOpts{})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "labntp serve: %v\n", err)
		return 1
	}
	store := snapshot.NewStore()
	store.InstallBootstrap(snap)
	metrics := observability.NewRegistry()
	logger := observability.NewLogger(stderr, observability.ParseLevel(snap.Canonical.Spec.Observability.LogLevel))
	logger.SetRegistry(metrics)
	qlog := querylog.New(snap.QueryLogSize)
	svc := app.New(app.Options{
		Snapshots:          store,
		BootstrapPath:      flags.Config,
		Metrics:            metrics,
		Logger:             logger,
		QueryLog:           qlog,
		NTPListenOverride:  flags.NTPListen,
		MgmtListenOverride: flags.ManagementListen,
	})

	addr := snap.NTPAddress
	if flags.NTPListen != "" {
		addr = flags.NTPListen
	}
	ntp, err := ntpserver.New(ntpserver.Config{Addr: addr, Store: store, Log: qlog, Metrics: metrics})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "labntp serve: %v\n", err)
		return 1
	}
	if err := ntp.Start(); err != nil {
		_, _ = fmt.Fprintf(stderr, "labntp serve: %v\n", err)
		return 1
	}
	_, _ = fmt.Fprintf(stdout, "labntp ntp listen=%s\n", ntp.Addr().String())
	svc.SetNTPRebind(ntp.Rebind)
	svc.SetQueryLog(qlog)

	var restSrv *rest.Server
	var mcpSrv *mcp.Server
	var metricsLn *observability.Listener
	mgmtOff := managementOff(flags.ManagementListen)

	svc.SetHealth(func() observability.Facts {
		return observability.Facts{
			NTPBound:   ntp.Bound(),
			SnapshotUp: store.Load() != nil,
			MgmtBound:  restSrv != nil && restSrv.Bound(),
			MgmtOff:    mgmtOff,
		}
	})

	if !mgmtOff {
		v, vErr := auth.FromSpec(snap.Canonical.Spec.Auth)
		if vErr != nil {
			_, _ = fmt.Fprintf(stderr, "labntp serve: auth: %v\n", vErr)
			_ = ntp.Shutdown(context.Background())
			return 1
		}
		if err := v.RequireListen(); err != nil {
			_, _ = fmt.Fprintf(stderr, "labntp serve: %v\n", err)
			_ = ntp.Shutdown(context.Background())
			return 1
		}
		mcpSrv, err = mcp.New(mcp.Config{
			Service:            svc,
			AllowedOrigins:     snap.Canonical.Spec.Management.AllowedOrigins,
			AllowLegacyClients: snap.Canonical.Spec.Management.MCP.AllowLegacyClients,
			RatePerSec:         float64(snap.Canonical.Spec.Management.RequestsPerSecond),
			RateBurst:          float64(snap.Canonical.Spec.Management.Burst),
			MaxBodyBytes:       snap.Canonical.Spec.Management.BodyLimit,
			MaxConcurrent:      snap.Canonical.Spec.Management.MaxConcurrent,
			Auth:               v,
			Metrics:            metrics,
		})
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "labntp serve: mcp: %v\n", err)
			_ = ntp.Shutdown(context.Background())
			return 1
		}
		restSrv, err = rest.New(rest.Config{
			Addr:           flags.ManagementListen,
			Service:        svc,
			AllowedOrigins: snap.Canonical.Spec.Management.AllowedOrigins,
			MaxBodyBytes:   snap.Canonical.Spec.Management.BodyLimit,
			MaxConcurrent:  snap.Canonical.Spec.Management.MaxConcurrent,
			RatePerSec:     float64(snap.Canonical.Spec.Management.RequestsPerSecond),
			RateBurst:      float64(snap.Canonical.Spec.Management.Burst),
			PublicMetrics:  snap.Canonical.Spec.Observability.Metrics.PublicPath,
			Metrics:        metrics,
			Logger:         logger,
			Auth:           v,
			Ready: func() bool {
				return ntp.Bound() && store.Load() != nil
			},
			Mounts: map[string]http.Handler{
				"/mcp": mcpSrv.Handler(),
			},
		})
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "labntp serve: rest: %v\n", err)
			_ = ntp.Shutdown(context.Background())
			return 1
		}
		svc.SetHTTPRebind(restSrv.Rebind)
		go func() {
			if err := restSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				_, _ = fmt.Fprintf(stderr, "labntp management: %v\n", err)
			}
		}()
		_, _ = fmt.Fprintf(stdout, "labntp management listen=%s\n", flags.ManagementListen)
	} else {
		_, _ = fmt.Fprintln(stdout, "labntp management: not bound")
	}

	if listen := strings.TrimSpace(snap.Canonical.Spec.Observability.Metrics.Listen); listen != "" {
		metricsLn, err = observability.Listen(listen, metrics)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "labntp metrics: %v\n", err)
		}
	}

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				metrics.Set(observability.MetricUDPInflight, nil, float64(ntp.Inflight()))
			}
		}
	}()

	if flags.PIDFile != "" {
		if err := os.WriteFile(flags.PIDFile, []byte(fmt.Sprintf("%d\n", os.Getpid())), 0o644); err != nil {
			_, _ = fmt.Fprintf(stderr, "labntp serve: pid-file: %v\n", err)
		}
	}
	<-ctx.Done()
	deadline := flags.ShutdownTimeout
	if deadline <= 0 {
		deadline = ntpserver.DefaultShutdownWait
	}
	shctx, cancel := context.WithTimeout(context.Background(), deadline)
	defer cancel()
	if restSrv != nil {
		_ = restSrv.Shutdown(shctx)
	}
	if mcpSrv != nil {
		mcpSrv.Close()
	}
	if metricsLn != nil {
		_ = metricsLn.Shutdown(shctx)
	}
	_ = ntp.Shutdown(shctx)
	_, _ = fmt.Fprintln(stdout, "labntp: shutting down")
	return 0
}
