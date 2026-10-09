package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const greenJobsJSON = `{"jobs":[
  {"name":"format","conclusion":"success"},
  {"name":"lint","conclusion":"success"},
  {"name":"unit","conclusion":"success"},
  {"name":"race","conclusion":"success"},
  {"name":"fuzz-smoke","conclusion":"success"},
  {"name":"documentation","conclusion":"success"},
  {"name":"config-compat","conclusion":"success"},
  {"name":"changelog","conclusion":"success"},
  {"name":"generated-file","conclusion":"success"},
  {"name":"parity","conclusion":"success"},
  {"name":"security-scan","conclusion":"success"},
  {"name":"container-test","conclusion":"success"},
  {"name":"web","conclusion":"success"}
]}`

func TestRequireCIRejectsGreenMainWhenTagRunRed(t *testing.T) {
	list := `[{"databaseId":11,"conclusion":"success","status":"completed","headSha":"abc","event":"push","headBranch":"main","displayTitle":"main"},{"databaseId":22,"conclusion":"failure","status":"completed","headSha":"abc","event":"push","headBranch":"v1.2.3","displayTitle":"tag v1.2.3"}]`
	view := `#!/bin/sh
case "$3" in
11)
  cat <<'EOF'
` + greenJobsJSON + `
EOF
  exit 0
  ;;
esac
echo '{"jobs":[{"name":"unit","conclusion":"failure"}]}'
exit 0
`
	installFakeGH(t, list, view)
	setTagEnv(t)
	err := requireGreenCI("", "")
	if err == nil {
		t.Fatal("gate accepted the older successful push run and ignored the failed tag run for the same SHA")
	}
	if strings.Contains(err.Error(), "pending") || strings.Contains(err.Error(), "no matching run") {
		t.Fatalf("error %v", err)
	}
}

func TestRequireCIAcceptsMatchingTagPush(t *testing.T) {
	list := `[{"databaseId":11,"conclusion":"success","status":"completed","headSha":"abc","event":"push","headBranch":"v1.2.3"}]`
	installFakeGH(t, list, "cat <<'EOF'\n"+greenJobsJSON+"\nEOF\nexit 0\n")
	setTagEnv(t)
	if err := requireGreenCI("", ""); err != nil {
		t.Fatal(err)
	}
}

func TestRequireCIRejectsMainPushSameSHA(t *testing.T) {
	list := `[{"databaseId":11,"conclusion":"success","status":"completed","headSha":"abc","event":"push","headBranch":"main"}]`
	installFakeGH(t, list, "cat <<'EOF'\n"+greenJobsJSON+"\nEOF\nexit 0\n")
	setTagEnv(t)
	err := requireGreenCI("", "")
	if err == nil || !strings.Contains(err.Error(), "no matching run") {
		t.Fatalf("err=%v", err)
	}
}

func TestRequireCIRejectsPullRequestFallback(t *testing.T) {
	list := `[{"databaseId":11,"conclusion":"success","status":"completed","headSha":"abc","event":"pull_request","headBranch":"v1.2.3"}]`
	installFakeGH(t, list, "cat <<'EOF'\n"+greenJobsJSON+"\nEOF\nexit 0\n")
	setTagEnv(t)
	err := requireGreenCI("", "")
	if err == nil || !strings.Contains(err.Error(), "no matching run") {
		t.Fatalf("err=%v", err)
	}
}

func TestRequireCIPendingTagRun(t *testing.T) {
	list := `[{"databaseId":22,"conclusion":"","status":"in_progress","headSha":"abc","event":"push","headBranch":"v1.2.3"},{"databaseId":11,"conclusion":"success","status":"completed","headSha":"abc","event":"push","headBranch":"v1.2.3"}]`
	installFakeGH(t, list, "cat <<'EOF'\n"+greenJobsJSON+"\nEOF\nexit 0\n")
	setTagEnv(t)
	err := requireGreenCI("", "")
	if err == nil || !strings.Contains(err.Error(), "pending") {
		t.Fatalf("err=%v", err)
	}
}

// An older in-progress tag run must not block a newer completed green one.
func TestRequireCIOlderInProgressDoesNotBlockNewerGreen(t *testing.T) {
	list := `[{"databaseId":11,"conclusion":"","status":"in_progress","headSha":"abc","event":"push","headBranch":"v1.2.3"},{"databaseId":22,"conclusion":"success","status":"completed","headSha":"abc","event":"push","headBranch":"v1.2.3"}]`
	installFakeGH(t, list, "cat <<'EOF'\n"+greenJobsJSON+"\nEOF\nexit 0\n")
	setTagEnv(t)
	if err := requireGreenCI("", ""); err != nil {
		t.Fatal(err)
	}
}

func TestResolveTag(t *testing.T) {
	cases := []struct {
		ref  string
		name string
		want string
		ok   bool
	}{
		{ref: "refs/tags/v1.2.3", name: "v1.2.3", want: "v1.2.3", ok: true},
		{ref: "refs/heads/main", name: "v1.2.3", want: "v1.2.3", ok: true},
		{ref: "refs/heads/main", name: "refs/tags/v1.2.3", want: "v1.2.3", ok: true},
		{ref: "refs/heads/main", name: "main", ok: false},
		{ok: false},
	}
	for _, tc := range cases {
		t.Run(tc.ref+"/"+tc.name, func(t *testing.T) {
			t.Setenv("GITHUB_REF", tc.ref)
			t.Setenv("GITHUB_REF_NAME", tc.name)
			got, err := resolveReleaseTag()
			if tc.ok {
				if err != nil || got != tc.want {
					t.Fatalf("got %q err %v", got, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("got %q, want error", got)
			}
		})
	}
}

func TestReleaseWorkflowDoesNotInterpolateTagIntoShell(t *testing.T) {
	release := filepath.Join("..", "..", ".github", "workflows", "release.yml")
	ci := filepath.Join("..", "..", ".github", "workflows", "ci.yml")
	b, err := os.ReadFile(release)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{release, ci} {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		hits := runValueInterpolations(string(body))
		if len(hits) > 0 {
			t.Fatalf("%s run: interpolates actions expressions:\n%s", path, strings.Join(hits, "\n"))
		}
	}
	if strings.Contains(string(b), `ref="${{ github.event.inputs.ref || github.ref_name }}"`) {
		t.Fatal("tag name is interpolated into the shell script")
	}
}

func TestRunValueInterpolationDetector(t *testing.T) {
	cases := []struct {
		name string
		yaml string
		want int
	}{
		{name: "inline run", yaml: "run: echo ${{ github.ref }}\n", want: 1},
		{name: "block run", yaml: "run: |\n  echo ${{ github.ref }}\n", want: 1},
		{name: "with", yaml: "with:\n  ref: ${{ github.ref }}\n", want: 0},
		{name: "concurrency", yaml: "concurrency:\n  group: ${{ github.ref }}\n", want: 0},
		{name: "clean block", yaml: "run: |\n  echo \"$REF\"\n", want: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := runValueInterpolations(tc.yaml)
			if len(got) != tc.want {
				t.Fatalf("hits=%v want %d", got, tc.want)
			}
		})
	}
}

func setTagEnv(t *testing.T) {
	t.Helper()
	t.Setenv("GITHUB_REF", "refs/tags/v1.2.3")
	t.Setenv("GITHUB_REF_NAME", "v1.2.3")
	t.Setenv("GITHUB_SHA", "abc")
}

func installFakeGH(t *testing.T, listJSON, viewBody string) {
	t.Helper()
	dir := t.TempDir()
	script := fmt.Sprintf(`#!/bin/sh
printf '%%s\n' "$*" >> "$(dirname "$0")/args"
case "$1 $2" in
"run list")
  cat <<'EOF'
%s
EOF
  exit 0
  ;;
"run view")
  %s
  ;;
esac
echo "unexpected: $*" >&2
exit 1
`, listJSON, viewBody)
	if err := os.WriteFile(filepath.Join(dir, "gh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+":/usr/bin:/bin")
}

func runValueInterpolations(src string) []string {
	lines := strings.Split(src, "\n")
	var hits []string
	inRun := false
	runIndent := 0
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if inRun {
			if strings.TrimSpace(line) == "" {
				continue
			}
			if indentOf(line) <= runIndent {
				inRun = false
				i--
				continue
			}
			if strings.Contains(line, "${{") {
				hits = append(hits, fmt.Sprintf("%d:%s", i+1, strings.TrimSpace(line)))
			}
			continue
		}
		trim := strings.TrimSpace(line)
		if trim == "" || strings.HasPrefix(trim, "#") {
			continue
		}
		rest, ok := strings.CutPrefix(strings.TrimLeft(line, " "), "run:")
		if !ok {
			continue
		}
		rest = strings.TrimSpace(rest)
		switch rest {
		case "|", "|-", "|+", ">", ">-", ">+":
			inRun = true
			runIndent = indentOf(line)
		default:
			if strings.Contains(rest, "${{") {
				hits = append(hits, fmt.Sprintf("%d:%s", i+1, trim))
			}
		}
	}
	return hits
}

func indentOf(line string) int {
	return len(line) - len(strings.TrimLeft(line, " "))
}

const (
	tagSHA    = "1111111111111111111111111111111111abcdef"
	branchSHA = "2222222222222222222222222222222222222222"
)

// dispatchEnv is what the gate sees on workflow_dispatch from main: GitHub
// ignores the workflow's step env for GITHUB_*, so they name the branch.
func dispatchEnv(t *testing.T) {
	t.Helper()
	t.Setenv("GITHUB_REF", "refs/heads/main")
	t.Setenv("GITHUB_REF_NAME", "main")
	t.Setenv("GITHUB_SHA", branchSHA)
}

func tagRunAt(sha string) string {
	return `[{"databaseId":11,"conclusion":"success","status":"completed","headSha":"` + sha + `","event":"push","headBranch":"v1.2.3"}]`
}

func ghArgs(t *testing.T) string {
	t.Helper()
	gh, err := exec.LookPath("gh")
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(filepath.Dir(gh), "args"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// mainAndTagRuns has main's green push at the branch head and the tag's
// push at the tag commit.
func mainAndTagRuns() string {
	return `[{"databaseId":5,"conclusion":"success","status":"completed","headSha":"` + branchSHA + `","event":"push","headBranch":"main"},` +
		`{"databaseId":11,"conclusion":"success","status":"completed","headSha":"` + tagSHA + `","event":"push","headBranch":"v1.2.3"}]`
}

// Without -tag/-sha a dispatch from main fails on the tag, before gh, and
// not with retryable text.
func TestRequireCIDispatchEnvWithoutFlagsFails(t *testing.T) {
	installFakeGH(t, mainAndTagRuns(), "cat <<'EOF'\n"+greenJobsJSON+"\nEOF\nexit 0\n")
	dispatchEnv(t)
	err := requireGreenCI("", "")
	if err == nil {
		t.Fatal("dispatch env without -tag/-sha was accepted")
	}
	if strings.Contains(err.Error(), "pending") || strings.Contains(err.Error(), "no matching run") {
		t.Fatalf("branch ref looks retryable: %v", err)
	}
}

func TestRequireCIExplicitFlagsIgnoreMainRun(t *testing.T) {
	installFakeGH(t, `[{"databaseId":5,"conclusion":"success","status":"completed","headSha":"`+branchSHA+`","event":"push","headBranch":"main"}]`, "cat <<'EOF'\n"+greenJobsJSON+"\nEOF\nexit 0\n")
	dispatchEnv(t)
	err := requireGreenCI("v1.2.3", tagSHA)
	if err == nil || !strings.Contains(err.Error(), "no matching run") {
		t.Fatalf("err=%v", err)
	}
	if args := ghArgs(t); !strings.Contains(args, "--commit="+tagSHA) {
		t.Fatalf("gh args:\n%s", args)
	}
}

func TestRequireCIExplicitTagAndSHA(t *testing.T) {
	for _, tag := range []string{"v1.2.3", "refs/tags/v1.2.3", " v1.2.3 "} {
		t.Run(tag, func(t *testing.T) {
			installFakeGH(t, mainAndTagRuns(), "cat <<'EOF'\n"+greenJobsJSON+"\nEOF\nexit 0\n")
			dispatchEnv(t)
			var errb bytes.Buffer
			if code := run([]string{"-require-ci", "-tag", tag, "-sha", tagSHA}, &errb); code != 0 {
				t.Fatalf("code %d: %s", code, errb.String())
			}
			args := ghArgs(t)
			if !strings.Contains(args, "--commit="+tagSHA) || strings.Contains(args, branchSHA) {
				t.Fatalf("gh args:\n%s", args)
			}
		})
	}
}

func TestRequireCITagWithoutSHAUsesHEAD(t *testing.T) {
	out, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err != nil {
		t.Skipf("no git HEAD: %v", err)
	}
	head := strings.TrimSpace(string(out))
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	installFakeGH(t, tagRunAt(head), "cat <<'EOF'\n"+greenJobsJSON+"\nEOF\nexit 0\n")
	t.Setenv("PATH", os.Getenv("PATH")+string(os.PathListSeparator)+filepath.Dir(gitPath))
	dispatchEnv(t)
	if err := requireGreenCI("v1.2.3", ""); err != nil {
		t.Fatal(err)
	}
	if args := ghArgs(t); !strings.Contains(args, "--commit="+head) || strings.Contains(args, branchSHA) {
		t.Fatalf("gh args:\n%s", args)
	}
}

func TestRequireCIExplicitFlagsRejectBadValues(t *testing.T) {
	cases := []struct{ tag, sha, want string }{
		{"main", tagSHA, "-tag"},
		{"v1.2.3;rm", tagSHA, "-tag"},
		{"v1.2.3", "abc", "-sha"},
		{"v1.2.3", strings.ToUpper(tagSHA), "-sha"},
		{"v1.2.3", tagSHA + ";rm", "-sha"},
		{"v1.2.3", "-h", "-sha"},
		{"v1.2.3-pending;", tagSHA, "-tag"},
		{"no matching run", tagSHA, "-tag"},
		{"v1.2.3", "pending", "-sha"},
		{"v1.2.3", "no matching run", "-sha"},
	}
	for _, tc := range cases {
		t.Run(tc.tag+"/"+tc.sha, func(t *testing.T) {
			installFakeGH(t, tagRunAt(tagSHA), "echo 'gh run view must not run' >&2\nexit 9\n")
			dispatchEnv(t)
			err := requireGreenCI(tc.tag, tc.sha)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err=%v", err)
			}
			if strings.Contains(err.Error(), "pending") || strings.Contains(err.Error(), "no matching run") {
				t.Fatalf("bad flag looks retryable: %v", err)
			}
		})
	}
}

func TestRunUsage(t *testing.T) {
	for _, args := range [][]string{
		nil,
		{"-notes-only", "-require-ci"},
		{"-notes-only", "-notes", "x.md", "-tag", "v1.2.3"},
		{"-notes-only", "-notes", "x.md", "-sha", tagSHA},
		{"-require-ci", "-notes", "x.md"},
		{"-require-ci", "extra"},
	} {
		var errb bytes.Buffer
		if code := run(args, &errb); code != 2 {
			t.Fatalf("%q: code %d", args, code)
		}
	}
}

// GitHub ignores step env that sets GITHUB_*, so the re-gate must pass the
// tag and commit to release-gate explicitly.
func TestReleaseWorkflowPassesTagAndSHA(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", ".github", "workflows", "release.yml"))
	if err != nil {
		t.Fatal(err)
	}
	rel := string(b)
	for _, bad := range []string{"GITHUB_SHA:", "GITHUB_REF:", "GITHUB_REF_NAME:"} {
		if strings.Contains(rel, bad) {
			t.Errorf("release.yml sets %s in env; GitHub ignores it", strings.TrimSuffix(bad, ":"))
		}
	}
	for _, want := range []string{
		"RELEASE_TAG: ${{ steps.gate.outputs.ref }}",
		"RELEASE_SHA: ${{ steps.rev.outputs.sha }}",
		`go run ./scripts/release-gate -require-ci -tag "$RELEASE_TAG" -sha "$RELEASE_SHA"`,
	} {
		if !strings.Contains(rel, want) {
			t.Errorf("release.yml missing %q", want)
		}
	}
}

// The env-path tag error is fixed text, so a ref whose name contains the
// workflow's retry words is not retried.
func TestEnvTagErrorDoesNotEchoRef(t *testing.T) {
	for _, name := range []string{"pending", "no matching run", "x-pending"} {
		t.Run(name, func(t *testing.T) {
			installFakeGH(t, mainAndTagRuns(), "cat <<'EOF'\n"+greenJobsJSON+"\nEOF\nexit 0\n")
			dispatchEnv(t)
			t.Setenv("GITHUB_REF_NAME", name)
			err := requireGreenCI("", "")
			if err == nil {
				t.Fatal("branch name accepted as a tag")
			}
			if strings.Contains(err.Error(), "pending") || strings.Contains(err.Error(), "no matching run") {
				t.Fatalf("env tag error looks retryable: %v", err)
			}
		})
	}
}
