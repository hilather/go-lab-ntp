package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"time"
)

func healthcheckCmd(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("healthcheck", flag.ContinueOnError)
	fs.SetOutput(stderr)
	url := fs.String("url", "http://127.0.0.1:8088/v1/health/ready", "ready URL")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(*url)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "labntp healthcheck: %v\n", err)
		return 1
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		_, _ = fmt.Fprintf(stderr, "labntp healthcheck: status %d\n", resp.StatusCode)
		return 1
	}
	_, _ = fmt.Fprintln(stdout, "ok")
	return 0
}
