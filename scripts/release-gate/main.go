// Command release-gate validates release notes headings and required CI on a SHA.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

var requiredHeadings = []string{
	"Highlights",
	"Added",
	"Residual",
	"Deployment and operations",
	"CI and release evidence",
}

var requiredCIJobs = []string{
	"format", "lint", "unit", "race", "fuzz-smoke", "documentation",
	"config-compat", "changelog", "generated-file", "parity", "security-scan",
	"container-test", "web",
}

// releaseTagPattern is the tag shape the release workflow accepts.
var releaseTagPattern = regexp.MustCompile(`^v[0-9A-Za-z.+-]+$`)

func main() {
	notesOnly := flag.Bool("notes-only", false, "validate notes headings only")
	notes := flag.String("notes", "", "path to docs/releases/vX.Y.Z.md")
	requireCI := flag.Bool("require-ci", false, "require green CI on GITHUB_SHA or HEAD")
	flag.Parse()
	if *notesOnly {
		if *notes == "" {
			fatal(fmt.Errorf("-notes-only requires -notes"))
		}
		if err := validateNotes(*notes); err != nil {
			fatal(err)
		}
		return
	}
	if *requireCI {
		if err := requireGreenCI(); err != nil {
			fatal(err)
		}
		return
	}
	fmt.Fprintf(os.Stderr, "usage: release-gate -notes-only -notes PATH | -require-ci\n")
	os.Exit(2)
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "release-gate: %v\n", err)
	os.Exit(1)
}

func validateNotes(path string) error {
	body, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	text := string(body)
	for _, h := range requiredHeadings {
		if !strings.Contains(text, "## "+h) && !strings.Contains(text, "# "+h) {
			return fmt.Errorf("notes %s missing heading %q", path, h)
		}
	}
	for _, bad := range []string{"TODO", "TBD", "FIXME"} {
		if strings.Contains(text, bad) {
			return fmt.Errorf("notes %s contains %s", path, bad)
		}
	}
	return nil
}

// resolveReleaseTag reads GITHUB_REF when it is refs/tags/..., otherwise
// GITHUB_REF_NAME, then strips a refs/tags/ prefix. The result must look
// like a release tag.
func resolveReleaseTag() (string, error) {
	ref := strings.TrimSpace(os.Getenv("GITHUB_REF"))
	name := strings.TrimSpace(os.Getenv("GITHUB_REF_NAME"))
	tag := name
	if strings.HasPrefix(ref, "refs/tags/") {
		tag = ref
	}
	tag = strings.TrimPrefix(tag, "refs/tags/")
	if !releaseTagPattern.MatchString(tag) {
		return "", fmt.Errorf("no tag ref %q", tag)
	}
	return tag, nil
}

func requireGreenCI() error {
	tag, err := resolveReleaseTag()
	if err != nil {
		return err
	}
	sha := strings.TrimSpace(os.Getenv("GITHUB_SHA"))
	if sha == "" {
		out, err := exec.Command("git", "rev-parse", "HEAD").Output()
		if err != nil {
			return fmt.Errorf("rev-parse HEAD: %w", err)
		}
		sha = strings.TrimSpace(string(out))
	}
	cmd := exec.Command("gh", "run", "list",
		"--workflow=ci.yml",
		"--commit="+sha,
		"--limit", "200",
		"--json", "databaseId,conclusion,status,headSha,event,headBranch,displayTitle")
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("gh run list: %w", err)
	}
	var runs []struct {
		DatabaseID int    `json:"databaseId"`
		Conclusion string `json:"conclusion"`
		Status     string `json:"status"`
		HeadSHA    string `json:"headSha"`
		Event      string `json:"event"`
		HeadBranch string `json:"headBranch"`
	}
	if err := json.Unmarshal(out, &runs); err != nil {
		return fmt.Errorf("parse gh run list: %w", err)
	}
	type match struct {
		id     int
		status string
	}
	var matched []match
	for _, r := range runs {
		if r.Event != "push" || r.HeadSHA != sha || r.HeadBranch != tag {
			continue
		}
		matched = append(matched, match{id: r.DatabaseID, status: r.Status})
	}
	if len(matched) == 0 {
		return fmt.Errorf("no matching run for tag %s commit %s", tag, sha)
	}
	pending := false
	id := 0
	for _, r := range matched {
		if r.status != "completed" {
			pending = true
			continue
		}
		if r.id > id {
			id = r.id
		}
	}
	if pending {
		return fmt.Errorf("pending CI run for tag %s", tag)
	}
	if id == 0 {
		return fmt.Errorf("no matching run for tag %s commit %s", tag, sha)
	}
	view := exec.Command("gh", "run", "view", fmt.Sprintf("%d", id), "--json", "jobs")
	jobJSON, err := view.Output()
	if err != nil {
		return fmt.Errorf("gh run view: %w", err)
	}
	var payload struct {
		Jobs []struct {
			Name       string `json:"name"`
			Conclusion string `json:"conclusion"`
		} `json:"jobs"`
	}
	if err := json.Unmarshal(jobJSON, &payload); err != nil {
		return fmt.Errorf("parse jobs: %w", err)
	}
	got := map[string]string{}
	for _, j := range payload.Jobs {
		got[j.Name] = j.Conclusion
	}
	var missing []string
	for _, name := range requiredCIJobs {
		if got[name] != "success" {
			missing = append(missing, fmt.Sprintf("%s=%s", name, got[name]))
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("required CI jobs not green: %s", strings.Join(missing, ", "))
	}
	return nil
}
