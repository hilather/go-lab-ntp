//go:build !linux

package ntpserver

import (
	"fmt"
	"time"
)

func readClocks() (realtime, monotonic time.Duration, err error) {
	return 0, 0, fmt.Errorf("CLOCK_REALTIME/CLOCK_MONOTONIC read is linux-only in this test")
}
