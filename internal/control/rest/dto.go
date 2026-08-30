package rest

import (
	"encoding/json"

	"github.com/hilather/go-lab-ntp/internal/app"
	"github.com/hilather/go-lab-ntp/internal/buildinfo"
	"github.com/hilather/go-lab-ntp/internal/capabilities"
	"github.com/hilather/go-lab-ntp/internal/model"
	"github.com/hilather/go-lab-ntp/internal/querylog"
)

type healthResponse struct {
	Status string `json:"status"`
}

type versionResponse struct {
	Version   string           `json:"version"`
	Commit    string           `json:"commit"`
	BuildTime string           `json:"buildTime"`
	Protocols versionProtocols `json:"protocols"`
}

type versionProtocols struct {
	ConfigAPI string `json:"configAPI"`
	REST      string `json:"rest"`
	MCP       string `json:"mcp"`
}

type capabilityViewResponse struct {
	Capabilities []capabilityInfo `json:"capabilities"`
}

type capabilityInfo struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Mutating    bool   `json:"mutating"`
	Idempotent  bool   `json:"idempotent"`
}

type statusResponse struct {
	Ready     bool            `json:"ready"`
	Revisions json.RawMessage `json:"revisions"`
	Listeners []listenerJSON  `json:"listeners"`
	HostTime  string          `json:"hostTime"`
	Warnings  []app.Warning   `json:"warnings,omitempty"`
}

type listenerJSON struct {
	Name    string `json:"name"`
	Address string `json:"address"`
}

type stateViewJSON struct {
	BootstrapRevision string          `json:"bootstrapRevision"`
	RuntimeRevision   string          `json:"runtimeRevision"`
	Generation        uint64          `json:"generation"`
	Drifted           bool            `json:"drifted"`
	LoadedAt          string          `json:"loadedAt,omitempty"`
	Canonical         json.RawMessage `json:"canonical"`
}

type changeRequest struct {
	ExpectedRevision string            `json:"expectedRevision"`
	IdempotencyKey   string            `json:"idempotencyKey"`
	Reason           string            `json:"reason"`
	Force            bool              `json:"force"`
	Operations       []model.Operation `json:"operations"`
	State            json.RawMessage   `json:"state"`
}

type resetRequest struct {
	Reason string `json:"reason"`
}

type sessionCreateJSON struct {
	CSRF      string `json:"csrf"`
	ExpiresAt string `json:"expiresAt"`
}

type sessionViewJSON struct {
	ID        string   `json:"id"`
	Role      string   `json:"role"`
	Scopes    []string `json:"scopes"`
	CSRF      string   `json:"csrf,omitempty"`
	ExpiresAt string   `json:"expiresAt,omitempty"`
}

type planJSON struct {
	PreviousRevision  string            `json:"previousRevision"`
	CandidateRevision string            `json:"candidateRevision"`
	Drifted           bool              `json:"drifted"`
	Diff              []app.DiffEntry   `json:"diff"`
	Warnings          []app.Warning     `json:"warnings,omitempty"`
	Operations        []model.Operation `json:"operations,omitempty"`
	Applied           bool              `json:"applied,omitempty"`
	Generation        uint64            `json:"generation,omitempty"`
	RuntimeRevision   string            `json:"runtimeRevision,omitempty"`
	AuditEventID      string            `json:"auditEventId,omitempty"`
}

type exportJSON struct {
	Format            string          `json:"format"`
	Revision          string          `json:"revision"`
	BootstrapRevision string          `json:"bootstrapRevision"`
	Drifted           bool            `json:"drifted"`
	Body              json.RawMessage `json:"body"`
	HumanDiff         string          `json:"humanDiff,omitempty"`
}

type previewJSON struct {
	IP             string  `json:"ip"`
	Filter         string  `json:"filter"`
	ServedTime     *string `json:"servedTime"`
	HostTime       string  `json:"hostTime"`
	Mode           string  `json:"mode,omitempty"`
	Leap           string  `json:"leap,omitempty"`
	Stratum        int     `json:"stratum,omitempty"`
	RefID          string  `json:"refid,omitempty"`
	OffsetFromHost string  `json:"offsetFromHost,omitempty"`
	Reason         string  `json:"reason,omitempty"`
}

type queryJSON struct {
	ClientIP   string `json:"clientIP"`
	Filter     string `json:"filter"`
	ServedTime string `json:"servedTime,omitempty"`
	Leap       string `json:"leap,omitempty"`
	Mode       string `json:"mode,omitempty"`
	VN         uint8  `json:"vn"`
	WhenHost   string `json:"whenHost,omitempty"`
}

type putFilterRequest struct {
	ExpectedRevision string        `json:"expectedRevision"`
	IdempotencyKey   string        `json:"idempotencyKey"`
	Reason           string        `json:"reason"`
	Filter           *model.Filter `json:"filter"`
}

type deleteFilterRequest struct {
	ExpectedRevision string `json:"expectedRevision"`
	IdempotencyKey   string `json:"idempotencyKey"`
	Reason           string `json:"reason"`
}

func fromVersion(info buildinfo.Info) versionResponse {
	return versionResponse{
		Version:   info.Version,
		Commit:    info.Commit,
		BuildTime: info.BuildTime,
		Protocols: versionProtocols{
			ConfigAPI: info.Protocols.ConfigAPI,
			REST:      info.Protocols.REST,
			MCP:       info.Protocols.MCP,
		},
	}
}

func fromCapabilities() capabilityViewResponse {
	src := capabilities.DiscoveryList()
	out := make([]capabilityInfo, 0, len(src))
	for _, d := range src {
		out = append(out, capabilityInfo{
			Name: d.Name, Version: d.Version, Description: d.Description,
			Mutating: d.Mutating, Idempotent: d.Idempotent,
		})
	}
	return capabilityViewResponse{Capabilities: out}
}

func fromPlan(p *app.Plan) planJSON {
	if p == nil {
		return planJSON{Diff: []app.DiffEntry{}}
	}
	return planJSON{
		PreviousRevision:  string(p.PreviousRevision),
		CandidateRevision: string(p.CandidateRevision),
		Drifted:           p.Drifted,
		Diff:              p.Diff,
		Warnings:          p.Warnings,
		Operations:        p.Operations,
	}
}

func fromApply(r *app.ApplyResult) planJSON {
	if r == nil {
		return planJSON{Diff: []app.DiffEntry{}}
	}
	out := fromPlan(&r.Plan)
	out.Applied = r.Applied
	out.Generation = uint64(r.Generation)
	out.RuntimeRevision = string(r.RuntimeRevision)
	out.AuditEventID = r.AuditEventID
	return out
}

func fromPreview(p *app.Preview) previewJSON {
	out := previewJSON{
		IP:             p.IP,
		Filter:         p.Filter,
		HostTime:       rfc3339(p.HostTime),
		Mode:           p.Mode,
		Leap:           p.Leap,
		Stratum:        p.Stratum,
		RefID:          p.RefID,
		OffsetFromHost: p.OffsetFromHost,
		Reason:         p.Reason,
	}
	if p.ServedTime != nil {
		s := rfc3339(*p.ServedTime)
		out.ServedTime = &s
	}
	return out
}

func fromQuery(e querylog.Entry) queryJSON {
	return queryJSON{
		ClientIP:   e.ClientIP,
		Filter:     e.Filter,
		ServedTime: rfc3339(e.ServedTime),
		Leap:       e.Leap,
		Mode:       e.Mode,
		VN:         e.VN,
		WhenHost:   rfc3339(e.WhenHost),
	}
}
