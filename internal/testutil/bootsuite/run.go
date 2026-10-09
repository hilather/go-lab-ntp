// Package bootsuite calls kittest.BootManagementOffNoSecretRead.
// cmd/labntp cannot import controlkit: its import test scans every Go
// file in that directory, including tests, and serveCmd is unexported.
package bootsuite

import (
	"context"
	"testing"

	"github.com/hilather/go-lab-controlkit/kittest"
)

// BootResult is one boot attempt observed by a driver.
type BootResult struct {
	Booted      bool
	DataPlaneOK bool
	Opens       map[string]int
	Message     string
}

// Driver is the ntp serveCmd boot oracle.
type Driver interface {
	Files(arm, files string) []string
	BoundMessage(files string) string
	Boot(ctx context.Context, arm, files string) BootResult
}

// Run executes kittest.BootManagementOffNoSecretRead.
func Run(t *testing.T, d Driver) {
	t.Helper()
	kittest.BootManagementOffNoSecretRead(t, adapter{d: d})
}

type adapter struct{ d Driver }

func (a adapter) Files(arm, files string) []string { return a.d.Files(arm, files) }

func (a adapter) BoundMessage(files string) string { return a.d.BoundMessage(files) }

func (a adapter) Boot(ctx context.Context, arm, files string) kittest.BootResult {
	r := a.d.Boot(ctx, arm, files)
	return kittest.BootResult{
		Booted:      r.Booted,
		DataPlaneOK: r.DataPlaneOK,
		Opens:       r.Opens,
		Message:     r.Message,
	}
}
