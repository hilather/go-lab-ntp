package auth

import (
	"slices"

	"github.com/hilather/go-lab-ntp/internal/model"
)

// DefaultScopes returns the frozen role → scope set. An explicit token
// Scopes list wins over role expansion.
func DefaultScopes(role string) []string {
	switch role {
	case model.RoleViewer:
		return []string{model.ScopeNTPRead}
	case model.RoleOperator:
		return []string{model.ScopeNTPRead, model.ScopeNTPWrite}
	case model.RoleAdministrator:
		return allScopes()
	default:
		return nil
	}
}

func sameScopes(a, b []string) bool {
	aa := append([]string(nil), a...)
	bb := append([]string(nil), b...)
	slices.Sort(aa)
	slices.Sort(bb)
	return slices.Equal(aa, bb)
}

func allScopes() []string {
	return []string{
		model.ScopeNTPRead,
		model.ScopeNTPWrite,
		model.ScopeNTPAdmin,
		model.ScopeNTPAuditRead,
	}
}
