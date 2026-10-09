// Command release-gate validates release notes headings and required CI on a SHA.
//
// -require-ci takes the release tag and its commit as -tag and -sha. The
// release workflow passes both. GitHub ignores a step's env override of
// GITHUB_SHA, GITHUB_REF and GITHUB_REF_NAME, and on workflow_dispatch those
// name the dispatching branch and its head, not the tag being re-gated. With
// -tag or -sha set, GITHUB_SHA is not read; with -tag set, GITHUB_REF and
// GITHUB_REF_NAME are not read. -tag without -sha uses git rev-parse HEAD.
// With neither flag the tag and SHA come from the environment, which holds
// on a tag push.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
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

// commitSHAPattern is a full lowercase SHA-1 or SHA-256 commit id.
var commitSHAPattern = regexp.MustCompile(`^([0-9a-f]{40}|[0-9a-f]{64})$`)

const usage = "usage: release-gate -notes-only -notes PATH | -require-ci [-tag TAG] [-sha SHA]\n"

func main() {
	os.Exit(run(os.Args[1:], os.Stderr))
}

func run(args []string, stderr io.Writer) int {
	fs := flag.NewFlagSet("release-gate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	notesOnly := fs.Bool("notes-only", false, "validate notes headings only")
	notes := fs.String("notes", "", "path to docs/releases/vX.Y.Z.md")
	requireCI := fs.Bool("require-ci", false, "require green CI of the tag push (tag and SHA from -tag/-sha, else the environment)")
	tagFlag := fs.String("tag", "", "with -require-ci: release tag, vX.Y.Z or refs/tags/vX.Y.Z; replaces GITHUB_REF/GITHUB_REF_NAME")
	shaFlag := fs.String("sha", "", "with -require-ci: the tag's commit; replaces GITHUB_SHA")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 || *notesOnly == *requireCI || (!*requireCI && (*tagFlag != "" || *shaFlag != "")) {
		_, _ = fmt.Fprint(stderr, usage)
		return 2
	}
	if *notesOnly {
		if *notes == "" {
			return fail(stderr, fmt.Errorf("-notes-only requires -notes"))
		}
		if err := validateNotes(*notes); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *notes != "" {
		_, _ = fmt.Fprint(stderr, usage)
		return 2
	}
	if err := requireGreenCI(*tagFlag, *shaFlag); err != nil {
		return fail(stderr, err)
	}
	return 0
}

func fail(stderr io.Writer, err error) int {
	_, _ = fmt.Fprintf(stderr, "release-gate: %v\n", err)
	return 1
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
		// The ref is not echoed: the workflow retries on "pending" and
		// "no matching run" in the output.
		return "", fmt.Errorf("no release tag in GITHUB_REF or GITHUB_REF_NAME")
	}
	return tag, nil
}

// flagTag canonicalizes -tag: trim, strip one refs/tags/ prefix, and
// require the release tag pattern.
func flagTag(raw string) (string, error) {
	tag := strings.TrimPrefix(strings.TrimSpace(raw), "refs/tags/")
	if !releaseTagPattern.MatchString(tag) {
		// The value is not echoed: the workflow retries on "pending" and
		// "no matching run" in the output.
		return "", fmt.Errorf("-tag is not a release tag")
	}
	return tag, nil
}

// requireGreenCI requires the newest CI run of the tag push to be green.
// tagArg and shaArg come from -tag and -sha; see the package comment for
// which environment variables each one replaces.
func requireGreenCI(tagArg, shaArg string) error {
	tagArg, shaArg = strings.TrimSpace(tagArg), strings.TrimSpace(shaArg)
	var tag string
	var err error
	if tagArg != "" {
		tag, err = flagTag(tagArg)
	} else {
		tag, err = resolveReleaseTag()
	}
	if err != nil {
		return err
	}
	sha := ""
	switch {
	case shaArg != "":
		if !commitSHAPattern.MatchString(shaArg) {
			return fmt.Errorf("-sha is not a full lowercase commit sha")
		}
		sha = shaArg
	case tagArg == "":
		sha = strings.TrimSpace(os.Getenv("GITHUB_SHA"))
	}
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
	// Judge only the highest databaseId. An older queued or in-progress
	// run must not block a newer completed green run.
	best := matched[0]
	for _, r := range matched[1:] {
		if r.id > best.id {
			best = r
		}
	}
	if best.status != "completed" {
		return fmt.Errorf("pending CI run for tag %s", tag)
	}
	view := exec.Command("gh", "run", "view", fmt.Sprintf("%d", best.id), "--json", "jobs")
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
