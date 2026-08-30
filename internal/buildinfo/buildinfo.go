// Package buildinfo reports version, commit, build time, and supported protocols.
package buildinfo

import (
	"fmt"
	"runtime/debug"
	"sync"
)

// Set via -ldflags at release builds. Commit and build time fall back to
// runtime/debug VCS settings, then "unknown". Version stays "dev" unless ldflags set it.
var (
	version   = "dev"
	commit    = ""
	buildTime = ""
)

// First-GA protocol placeholders. These are not ldflag-overridable.
const (
	ConfigAPIVersion = "labntp.dev/v1alpha1"
	RESTPrefix       = "/v1"
	MCPProtocol      = "2026-07-28"
)

// Protocols names the supported on-the-wire and config identities.
type Protocols struct {
	ConfigAPI string
	REST      string
	MCP       string
}

// Info is process build metadata plus protocol placeholders.
type Info struct {
	Version   string
	Commit    string
	BuildTime string
	Protocols Protocols
}

func (i Info) String() string {
	return fmt.Sprintf("labntp %s commit=%s built=%s config=%s rest=%s mcp=%s",
		i.Version, i.Commit, i.BuildTime, i.Protocols.ConfigAPI, i.Protocols.REST, i.Protocols.MCP)
}

var (
	readOnce sync.Once
	cached   Info
)

// Current returns process build metadata. Missing ldflags fall back to
// runtime/debug VCS info, then "unknown".
func Current() Info {
	readOnce.Do(func() {
		rev := commit
		when := buildTime
		if rev == "" || when == "" {
			if info, ok := debug.ReadBuildInfo(); ok {
				for _, s := range info.Settings {
					switch s.Key {
					case "vcs.revision":
						if rev == "" {
							rev = s.Value
							if len(rev) > 12 {
								rev = rev[:12]
							}
						}
					case "vcs.time":
						if when == "" {
							when = s.Value
						}
					}
				}
			}
		}
		if rev == "" {
			rev = "unknown"
		}
		if when == "" {
			when = "unknown"
		}
		cached = Info{
			Version:   version,
			Commit:    rev,
			BuildTime: when,
			Protocols: Protocols{
				ConfigAPI: ConfigAPIVersion,
				REST:      RESTPrefix,
				MCP:       MCPProtocol,
			},
		}
	})
	return cached
}
