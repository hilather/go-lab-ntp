package app

import (
	"context"
	"fmt"
	"math"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/hilather/go-lab-ntp/internal/model"
)

// Offset math.MinInt64 used to reach CanonicalJSON → FormatDuration, which
// recurses on the minimum signed duration and crashes the process. Apply must
// reject it as validation_failed and the child must exit 0.
func TestApplyMinDurationDoesNotCrash(t *testing.T) {
	if os.Getenv("LABNTP_MINDUR_CHILD") == "1" {
		svc, snap := mustBoot(t)
		off := time.Duration(math.MinInt64)
		_, err := svc.Apply(context.Background(), actor(), ChangeIn{
			ExpectedRevision: snap.Revision,
			Operations: []model.Operation{{
				Op: model.OpUpsertFilter,
				Filter: &model.Filter{
					Name:    "shift",
					Enabled: true,
					Match:   model.MatchSpec{CIDRs: []string{"10.0.0.0/8"}},
					View: model.ViewSpec{
						Mode:    model.ModeOffset,
						Offset:  off,
						Leap:    model.LeapNone,
						Stratum: 2,
					},
				},
			}},
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "apply error: %v\n", err)
		}
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestApplyMinDurationDoesNotCrash$", "-test.count=1")
	cmd.Env = append(os.Environ(), "LABNTP_MINDUR_CHILD=1", "GOTRACEBACK=none")
	out, err := cmd.CombinedOutput()
	msg := string(out)
	if err != nil {
		if len(msg) > 400 {
			msg = msg[:400]
		}
		t.Fatalf("apply of min duration crashed the process: %v\n%s", err, msg)
	}
	if !strings.Contains(msg, "validation_failed") {
		t.Fatalf("child output missing validation_failed:\n%s", msg)
	}
}
