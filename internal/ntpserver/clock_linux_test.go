package ntpserver

import (
	"syscall"
	"time"
	"unsafe"
)

func readClocks() (realtime, monotonic time.Duration, err error) {
	var rt, mono syscall.Timespec
	if _, _, e := syscall.Syscall(syscall.SYS_CLOCK_GETTIME, uintptr(0), uintptr(unsafe.Pointer(&rt)), 0); e != 0 {
		return 0, 0, e
	}
	if _, _, e := syscall.Syscall(syscall.SYS_CLOCK_GETTIME, uintptr(1), uintptr(unsafe.Pointer(&mono)), 0); e != 0 {
		return 0, 0, e
	}
	return time.Duration(rt.Nano()), time.Duration(mono.Nano()), nil
}
