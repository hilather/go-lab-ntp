package main

import (
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/hilather/go-lab-ntp/internal/config"
)

func requireConfigFlag(args []string, name string, stderr io.Writer) (path string, err error) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	cfg := fs.String("config", "", "path to bootstrap YAML or JSON")
	if err := fs.Parse(args); err != nil {
		return "", err
	}
	if *cfg == "" {
		_, _ = fmt.Fprintf(stderr, "labntp %s: --config is required\n", name)
		return "", fmt.Errorf("missing --config")
	}
	return *cfg, nil
}

func validateCmd(args []string, stdout, stderr io.Writer) int {
	path, err := requireConfigFlag(args, "validate", stderr)
	if err != nil {
		return 2
	}
	st, warns, err := config.LoadFileWithWarnings(path)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "labntp validate: %v\n", err)
		return 1
	}
	for _, w := range warns {
		_, _ = fmt.Fprintf(stderr, "warning %s: %s\n", w.Path, w.Message)
	}
	rev, err := config.Revision(st)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "labntp validate: %v\n", err)
		return 1
	}
	_, _ = fmt.Fprintf(stdout, "ok revision=%s\n", rev)
	return 0
}

func canonicalizeCmd(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("canonicalize", flag.ContinueOnError)
	fs.SetOutput(stderr)
	path := fs.String("config", "", "path to bootstrap YAML or JSON")
	format := fs.String("format", "yaml", "yaml or json")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *path == "" {
		_, _ = fmt.Fprintln(stderr, "labntp canonicalize: --config is required")
		return 2
	}
	st, err := config.LoadFile(*path)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "labntp canonicalize: %v\n", err)
		return 1
	}
	var body []byte
	switch strings.ToLower(*format) {
	case "json":
		body, err = config.CanonicalJSON(st)
	case "yaml", "yml", "":
		body, err = config.CanonicalYAML(st)
	default:
		_, _ = fmt.Fprintln(stderr, "labntp canonicalize: --format must be yaml or json")
		return 2
	}
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "labntp canonicalize: %v\n", err)
		return 1
	}
	_, _ = stdout.Write(body)
	if len(body) == 0 || body[len(body)-1] != '\n' {
		_, _ = fmt.Fprintln(stdout)
	}
	return 0
}
