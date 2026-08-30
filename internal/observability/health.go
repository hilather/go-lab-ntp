package observability

// Warning codes are a bounded, stable Status DTO surface.
const (
	WarnNTPUnbound      = "ntp_unbound"
	WarnSnapshotMissing = "snapshot_missing"
	WarnMgmtUnbound     = "management_unbound"
	WarnListenerUnbound = "listener_unbound"
)

// MaxWarnings caps the Status warning list.
const MaxWarnings = 16

// Warning is one agent-readable operational note.
type Warning struct {
	Code    string
	Message string
}

// Facts are process observations used to evaluate health.
type Facts struct {
	ProcessDown bool
	NTPBound    bool
	SnapshotUp  bool
	// MgmtBound is true when the management listener is accepting.
	MgmtBound bool
	// MgmtOff is true when management was explicitly disabled (off/none/-).
	MgmtOff bool
}

// Probe is liveness and readiness plus bounded warnings.
type Probe struct {
	Live     bool
	Ready    bool
	Warnings []Warning
}

// Evaluate implements Ready = NTP bound + snapshot + (mgmt bound or off).
// Ready stays true on the old NTP socket until a new bind succeeds.
func Evaluate(in Facts) Probe {
	p := Probe{Live: !in.ProcessDown}
	mgmtOK := in.MgmtBound || in.MgmtOff
	p.Ready = p.Live && in.NTPBound && in.SnapshotUp && mgmtOK

	add := func(code, msg string) {
		if len(p.Warnings) >= MaxWarnings {
			return
		}
		p.Warnings = append(p.Warnings, Warning{Code: code, Message: msg})
	}
	if !in.NTPBound {
		add(WarnNTPUnbound, "NTP UDP listener is not bound")
		add(WarnListenerUnbound, "a required listener is not bound")
	}
	if !in.SnapshotUp {
		add(WarnSnapshotMissing, "compiled snapshot is not installed")
	}
	if !mgmtOK {
		add(WarnMgmtUnbound, "management listener is not bound")
		if in.NTPBound {
			add(WarnListenerUnbound, "a required listener is not bound")
		}
	}
	return p
}
