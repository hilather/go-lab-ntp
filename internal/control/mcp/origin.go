package mcp

import "github.com/hilather/go-lab-ntp/internal/auth"

func checkOrigin(origin string, allowlist []string) error {
	return auth.CheckOrigin(origin, allowlist)
}
