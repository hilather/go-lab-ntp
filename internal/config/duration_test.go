package config

import (
	"fmt"
	"math"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/hilather/go-lab-ntp/internal/model"
)

func TestFormatDurationMinInt64(t *testing.T) {
	if os.Getenv("LABNTP_FORMAT_MINDUR") == "1" {
		s := FormatDuration(time.Duration(math.MinInt64))
		if s == "" {
			fmt.Fprintln(os.Stderr, "empty duration")
			os.Exit(2)
		}
		fmt.Println(s)
		return
	}
	if got := FormatDuration(-6 * time.Minute); got != "-6m" {
		t.Fatalf("FormatDuration(-6m)=%q", got)
	}
	if got := FormatDuration(0); got != "0s" {
		t.Fatalf("FormatDuration(0)=%q", got)
	}
	if got := FormatDuration(time.Hour); got != "1h" {
		t.Fatalf("FormatDuration(1h)=%q", got)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestFormatDurationMinInt64$", "-test.count=1")
	cmd.Env = append(os.Environ(), "LABNTP_FORMAT_MINDUR=1", "GOTRACEBACK=none")
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := string(out)
		if len(msg) > 400 {
			msg = msg[:400]
		}
		t.Fatalf("FormatDuration(MinInt64) crashed: %v\n%s", err, msg)
	}
	if strings.TrimSpace(string(out)) == "" {
		t.Fatal("FormatDuration(MinInt64) returned empty output")
	}
}

func TestValidateRejectsMinInt64Offset(t *testing.T) {
	st, err := Decode([]byte(mustLoad(t, "valid", "defaults.yaml")))
	if err != nil {
		t.Fatal(err)
	}
	n, _, err := Normalize(st)
	if err != nil {
		t.Fatal(err)
	}
	if len(n.Spec.Filters) == 0 {
		t.Fatal("defaults have no filters")
	}
	n.Spec.Filters[0].View.Mode = model.ModeOffset
	n.Spec.Filters[0].View.Offset = time.Duration(math.MinInt64)
	err = Validate(n)
	de := requireValidation(t, err, violationInvalidValue)
	found := false
	for _, v := range de.FieldViolations {
		if v.Code == violationInvalidValue && strings.HasSuffix(v.Path, ".offset") {
			found = true
			if !strings.Contains(v.Message, "time.ParseDuration") {
				t.Fatalf("offset violation message %q", v.Message)
			}
		}
	}
	if !found {
		t.Fatalf("missing offset invalid_value in %+v", de.FieldViolations)
	}
}
