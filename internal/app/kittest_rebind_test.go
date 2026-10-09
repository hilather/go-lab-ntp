package app_test

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"testing"

	"github.com/hilather/go-lab-controlkit/kittest"
)

func TestManagementRebindOverAPIMove(t *testing.T) {
	kittest.ManagementRebindOverAPI(t, &rebindDriver{t: t, variant: kittest.RebindMove})
}

func TestManagementRebindOverAPIOff(t *testing.T) {
	kittest.ManagementRebindOverAPI(t, &rebindDriver{t: t, variant: kittest.RebindOff})
}

func TestManagementRebindOverAPITaken(t *testing.T) {
	kittest.ManagementRebindOverAPI(t, &rebindDriver{t: t, variant: kittest.RebindTaken})
}

func TestManagementRebindOverAPISame(t *testing.T) {
	kittest.ManagementRebindOverAPI(t, &rebindDriver{t: t, variant: kittest.RebindSame})
}

type rebindDriver struct {
	t       *testing.T
	variant kittest.RebindVariant
}

func (d *rebindDriver) Variant() kittest.RebindVariant { return d.variant }

func (d *rebindDriver) FailureText() string {
	if d.variant == kittest.RebindTaken {
		return "internal_error"
	}
	return ""
}

func (d *rebindDriver) Run(context.Context) kittest.RebindObs {
	d.t.Helper()
	env := bootManagement(d.t)
	old := env.addr
	next := old
	switch d.variant {
	case kittest.RebindMove:
		next = freeAddr(d.t)
		env.rewrite(d.t, next, "rebind")
	case kittest.RebindOff:
		env.svc.SetMgmtOverrideForTest("off")
	case kittest.RebindTaken:
		hold, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			d.t.Fatal(err)
		}
		d.t.Cleanup(func() { _ = hold.Close() })
		next = hold.Addr().String()
		env.rewrite(d.t, next, "rebind")
	case kittest.RebindSame:
		env.rewrite(d.t, old, "rebind-same")
	}
	before := env.svc.Active()
	beforeRev := string(before.Revision)
	beforeGen := uint64(before.Generation)

	status, body, elapsed := env.postReset(d.t)
	code := ""
	if status != http.StatusOK {
		var prob struct {
			Code string `json:"code"`
		}
		_ = json.Unmarshal(body, &prob)
		code = prob.Code
	}
	var applied struct {
		Applied    bool   `json:"applied"`
		Generation uint64 `json:"generation"`
	}
	_ = json.Unmarshal(body, &applied)
	after := env.svc.Active()
	return kittest.RebindObs{
		Elapsed:         elapsed,
		Code:            code,
		NewServes:       d.variant == kittest.RebindMove && rebindServes(env, next),
		OldRefuses:      dialResult(old) == "refused",
		ResponseIntact:  status == http.StatusOK && applied.Applied && applied.Generation == beforeGen+1,
		RevisionChanged: string(after.Revision) != beforeRev,
		OldStillServes:  rebindServes(env, old),
	}
}

func rebindServes(env mgmtEnv, addr string) bool {
	req, err := http.NewRequest(http.MethodGet, "http://"+addr+"/v1/state", nil)
	if err != nil {
		return false
	}
	req.Header.Set("Authorization", "Bearer "+rebindToken)
	resp, err := env.client.Do(req)
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode == http.StatusOK
}
