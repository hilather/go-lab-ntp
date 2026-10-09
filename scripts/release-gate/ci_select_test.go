package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
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
	if retryable(err) || strings.Contains(err.Error(), "pending") || strings.Contains(err.Error(), "no matching run") {
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
	if err == nil || !strings.Contains(err.Error(), "no matching run") || !retryable(err) {
		t.Fatalf("err=%v", err)
	}
}

func TestRequireCIRejectsPullRequestFallback(t *testing.T) {
	list := `[{"databaseId":11,"conclusion":"success","status":"completed","headSha":"abc","event":"pull_request","headBranch":"v1.2.3"}]`
	installFakeGH(t, list, "cat <<'EOF'\n"+greenJobsJSON+"\nEOF\nexit 0\n")
	setTagEnv(t)
	err := requireGreenCI("", "")
	if err == nil || !strings.Contains(err.Error(), "no matching run") || !retryable(err) {
		t.Fatalf("err=%v", err)
	}
}

func TestRequireCIPendingTagRun(t *testing.T) {
	list := `[{"databaseId":22,"conclusion":"","status":"in_progress","headSha":"abc","event":"push","headBranch":"v1.2.3"},{"databaseId":11,"conclusion":"success","status":"completed","headSha":"abc","event":"push","headBranch":"v1.2.3"}]`
	installFakeGH(t, list, "cat <<'EOF'\n"+greenJobsJSON+"\nEOF\nexit 0\n")
	setTagEnv(t)
	err := requireGreenCI("", "")
	if err == nil || !strings.Contains(err.Error(), "pending") || !retryable(err) {
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
			if err == nil || retryable(err) {
				t.Fatalf("got %q err %v", got, err)
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
	if retryable(err) || strings.Contains(err.Error(), "pending") || strings.Contains(err.Error(), "no matching run") {
		t.Fatalf("branch ref looks retryable: %v", err)
	}
}

func TestRequireCIExplicitFlagsIgnoreMainRun(t *testing.T) {
	installFakeGH(t, `[{"databaseId":5,"conclusion":"success","status":"completed","headSha":"`+branchSHA+`","event":"push","headBranch":"main"}]`, "cat <<'EOF'\n"+greenJobsJSON+"\nEOF\nexit 0\n")
	dispatchEnv(t)
	err := requireGreenCI("v1.2.3", tagSHA)
	if err == nil || !strings.Contains(err.Error(), "no matching run") || !retryable(err) {
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
			if retryable(err) || strings.Contains(err.Error(), "pending") || strings.Contains(err.Error(), "no matching run") {
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
// tag and peeled commit to release-gate explicitly.
func TestWorkflowContract(t *testing.T) {
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
		"RELEASE_TAG: ${{ steps.rev.outputs.ref }}",
		"RELEASE_SHA: ${{ steps.rev.outputs.sha }}",
		`"$RUNNER_TEMP/release-gate" -require-ci -tag "$RELEASE_TAG" -sha "$RELEASE_SHA"`,
		`go build -o "$RUNNER_TEMP/release-gate"`,
		"refs/tags/${ref}^{commit}",
	} {
		if !strings.Contains(rel, want) {
			t.Errorf("release.yml missing %q", want)
		}
	}
	if strings.Contains(rel, "steps.gate.outputs") {
		t.Error("release.yml still uses steps.gate.outputs")
	}
	if got := strings.Count(rel, "re='"+releaseTagPatternSrc+"'"); got != 4 {
		t.Errorf("shell tag pattern count = %d, want 4", got)
	}
	if got := strings.Count(rel, `refs/tags/*) ref="${ref#refs/tags/}" ;;`); got != 4 {
		t.Errorf("tag prefix strip count = %d, want 4", got)
	}
	if strings.Contains(rel, `^v[0-9A-Za-z.+-]+$`) {
		t.Error("release.yml still uses the old tag pattern")
	}
	if got := strings.Count(rel, "ref: refs/tags/${{ steps.tag.outputs.ref }}"); got != 2 {
		t.Errorf("canonical checkout ref count = %d, want 2", got)
	}
	checkoutParts := strings.Split(rel, "uses: actions/checkout@")
	if len(checkoutParts) != 3 {
		t.Fatalf("actions/checkout steps = %d, want 2", len(checkoutParts)-1)
	}
	for i, chunk := range checkoutParts[1:] {
		if cut := strings.Index(chunk, "\n      - "); cut >= 0 {
			chunk = chunk[:cut]
		}
		if !strings.Contains(chunk, "persist-credentials: false") {
			t.Errorf("checkout step %d missing persist-credentials: false:\n%s", i+1, chunk)
		}
	}
	if got := strings.Count(rel, "persist-credentials: false"); got != 2 {
		t.Errorf("persist-credentials: false count = %d, want 2", got)
	}
	if strings.Contains(rel, "persist-credentials: true") {
		t.Error("release.yml sets persist-credentials: true")
	}
	for _, bad := range []string{
		"ref: ${{ github.event.inputs.ref || github.ref }}",
		"ref: ${{ github.ref }}",
		"github.ref_name",
		"*pending*",
		`*"no matching run"*`,
		`case "$out"`,
		"COMMIT=${{ github.sha }}",
	} {
		if strings.Contains(rel, bad) {
			t.Errorf("release.yml contains %q", bad)
		}
	}
	wantStatus := fmt.Sprintf(`[ "$status" -eq %d ]`, exitRetryable)
	if !strings.Contains(rel, wantStatus) {
		t.Errorf("release.yml missing %s", wantStatus)
	}
	if !strings.Contains(rel, "github.event_name == 'push'") || !strings.Contains(rel, "startsWith(github.ref, 'refs/tags/v')") {
		t.Error("publish-image if no longer requires a tag push")
	}
	requireOrder(t, "release.yml", rel,
		"\n      - name: Canonicalize release ref\n",
		"\n      - uses: actions/checkout@",
	)
	tagStart := strings.Index(rel, "\n  tag-gate:\n")
	pubStart := strings.Index(rel, "\n  publish-image:\n")
	if tagStart < 0 || pubStart < 0 || tagStart >= pubStart {
		t.Fatalf("job keys tag=%d publish=%d", tagStart, pubStart)
	}
	headMismatch := "if [ \"$head\" != \"$sha\" ]; then\n" +
		"            echo \"HEAD ${head} is not the peeled commit ${sha} of ${ref}\" >&2\n" +
		"            exit 1\n" +
		"          fi"
	gatedMismatch := "if [ -z \"$GATED_SHA\" ] || [ \"$ref\" != \"$GATED_REF\" ] || [ \"$sha\" != \"$GATED_SHA\" ]; then\n" +
		"            echo \"ref ${ref} commit ${sha} is not gated ref ${GATED_REF} commit ${GATED_SHA}\" >&2\n" +
		"            exit 1\n" +
		"          fi"
	tagGateOutputs := "outputs:\n" +
		"      sha: ${{ steps.rev.outputs.sha }}\n" +
		"      ref: ${{ steps.rev.outputs.ref }}"
	requireOrder(t, "tag-gate", rel[tagStart:pubStart],
		tagGateOutputs,
		"\n      - name: Canonicalize release ref\n",
		"\n      - uses: actions/checkout@",
		"persist-credentials: false",
		"ref: refs/tags/${{ steps.tag.outputs.ref }}",
		"refs/tags/${ref}^{commit}",
		headMismatch,
	)
	requireOrder(t, "publish-image", rel[pubStart:],
		"\n      - name: Canonicalize release ref\n",
		"\n      - uses: actions/checkout@",
		"persist-credentials: false",
		"ref: refs/tags/${{ steps.tag.outputs.ref }}",
		"GATED_SHA: ${{ needs.tag-gate.outputs.sha }}",
		"GATED_REF: ${{ needs.tag-gate.outputs.ref }}",
		"refs/tags/${ref}^{commit}",
		headMismatch,
		gatedMismatch,
		"VERSION=${{ steps.tag.outputs.ref }}",
		"COMMIT=${{ steps.rev.outputs.sha }}",
		"RELEASE_REF: ${{ steps.tag.outputs.ref }}",
	)
}

func requireOrder(t *testing.T, label, body string, parts ...string) {
	t.Helper()
	from := 0
	for _, part := range parts {
		rel := strings.Index(body[from:], part)
		if rel < 0 {
			t.Fatalf("%s: %q not found at or after offset %d", label, part, from)
		}
		from += rel + len(part)
	}
}

// The env-path tag error is fixed text and does not echo the ref. The retry
// signal is exit 75, and run([]string{"-require-ci"}) returns 1 for
// GITHUB_REF_NAME of pending, no matching run, and x-pending.
func TestEnvTagErrorDoesNotEchoRef(t *testing.T) {
	for _, name := range []string{"pending", "no matching run", "x-pending"} {
		t.Run(name, func(t *testing.T) {
			installFakeGH(t, mainAndTagRuns(), "cat <<'EOF'\n"+greenJobsJSON+"\nEOF\nexit 0\n")
			dispatchEnv(t)
			t.Setenv("GITHUB_REF_NAME", name)
			var errb bytes.Buffer
			if code := run([]string{"-require-ci"}, &errb); code != 1 {
				t.Fatalf("code %d: %s", code, errb.String())
			}
			if strings.Contains(errb.String(), "pending") || strings.Contains(errb.String(), "no matching run") || strings.Contains(errb.String(), name) {
				t.Fatalf("env tag error echoes the ref: %s", errb.String())
			}
			err := requireGreenCI("", "")
			if err == nil || retryable(err) {
				t.Fatalf("err=%v", err)
			}
		})
	}
}

func TestReleaseTagPattern(t *testing.T) {
	accepts := []string{
		"v1.2.3",
		"v1.2.3-rc.1",
		"v1.0.0-pending",
		"v01.2.3",
		"refs/tags/v1.2.3",
		" v1.2.3 ",
	}
	for _, raw := range accepts {
		t.Run("flag/"+raw, func(t *testing.T) {
			got, err := flagTag(raw)
			if err != nil {
				t.Fatal(err)
			}
			want := strings.TrimPrefix(strings.TrimSpace(raw), "refs/tags/")
			if got != want {
				t.Fatalf("got %q want %q", got, want)
			}
		})
	}
	t.Run("env/refs/tags/v1.2.3", func(t *testing.T) {
		t.Setenv("GITHUB_REF", "refs/tags/v1.2.3")
		t.Setenv("GITHUB_REF_NAME", "main")
		got, err := resolveReleaseTag()
		if err != nil || got != "v1.2.3" {
			t.Fatalf("got %q err %v", got, err)
		}
	})
	for _, name := range []string{"v1.2.3", "v1.2.3-rc.1", "v1.0.0-pending", "v01.2.3", " v1.2.3 "} {
		t.Run("env-name/"+name, func(t *testing.T) {
			t.Setenv("GITHUB_REF", "refs/heads/main")
			t.Setenv("GITHUB_REF_NAME", name)
			got, err := resolveReleaseTag()
			want := strings.TrimSpace(name)
			if err != nil || got != want {
				t.Fatalf("got %q err %v want %q", got, err, want)
			}
		})
	}
	rejects := []string{"vpending", "v1", "v1.2", "v1.2.3.4", "v1.2.3+meta", "main", ""}
	for _, raw := range rejects {
		t.Run("flag-reject/"+raw, func(t *testing.T) {
			if _, err := flagTag(raw); err == nil || retryable(err) {
				t.Fatalf("err=%v", err)
			}
		})
		t.Run("env-reject/"+raw, func(t *testing.T) {
			t.Setenv("GITHUB_REF", "refs/heads/main")
			t.Setenv("GITHUB_REF_NAME", raw)
			if _, err := resolveReleaseTag(); err == nil || retryable(err) {
				t.Fatalf("err=%v", err)
			}
		})
	}
}

func TestExitCodes(t *testing.T) {
	viewFail := "echo 'view should not run' >&2\nexit 1\n"
	t.Run("in progress", func(t *testing.T) {
		installFakeGH(t, listRunJSON(tagSHA, "v1.2.3", "in_progress", "", "push"), viewFail)
		dispatchEnv(t)
		code, msg := runRequireCI(t, "v1.2.3", tagSHA)
		if code != exitRetryable || msg != "release-gate: pending CI run for tag v1.2.3\n" {
			t.Fatalf("code %d\n%s", code, msg)
		}
		if !strings.HasPrefix(msg, "release-gate: pending ") {
			t.Fatalf("stderr %q", msg)
		}
	})
	t.Run("no matching run", func(t *testing.T) {
		installFakeGH(t, listRunJSON(tagSHA, "main", "completed", "success", "push"), viewFail)
		dispatchEnv(t)
		code, msg := runRequireCI(t, "v1.2.3", tagSHA)
		want := "release-gate: no matching run for tag v1.2.3 commit " + tagSHA + "\n"
		if code != exitRetryable || msg != want {
			t.Fatalf("code %d\n%s", code, msg)
		}
		if !strings.HasPrefix(msg, "release-gate: no matching run ") || strings.HasPrefix(msg, "release-gate: pending") {
			t.Fatalf("stderr %q", msg)
		}
	})
	t.Run("prerelease red job", func(t *testing.T) {
		const tag = "v1.0.0-pending"
		body := strings.Replace(greenJobsJSON, `{"name":"unit","conclusion":"success"}`, `{"name":"unit","conclusion":"failure"}`, 1)
		installFakeGH(t, listRunJSON(tagSHA, tag, "completed", "success", "push"), "cat <<'EOF'\n"+body+"\nEOF\nexit 0\n")
		dispatchEnv(t)
		err := requireGreenCI(tag, tagSHA)
		if err == nil || retryable(err) || !strings.Contains(err.Error(), "not green") {
			t.Fatalf("err=%v", err)
		}
		code, msg := runRequireCI(t, tag, tagSHA)
		if code != 1 || !strings.Contains(msg, "not green") || strings.HasPrefix(msg, "release-gate: pending ") {
			t.Fatalf("code %d\n%s", code, msg)
		}
	})
	t.Run("prerelease bad sha", func(t *testing.T) {
		dispatchEnv(t)
		err := requireGreenCI("v1.0.0-pending", "pending")
		if err == nil || retryable(err) {
			t.Fatalf("err=%v", err)
		}
		code, msg := runRequireCI(t, "v1.0.0-pending", "pending")
		if code != 1 || strings.Contains(msg, "v1.0.0-pending") || !strings.Contains(msg, "-sha is not a full lowercase commit sha") {
			t.Fatalf("code %d\n%s", code, msg)
		}
	})
	t.Run("gh list text is not retryable", func(t *testing.T) {
		installGHListFails(t)
		dispatchEnv(t)
		code, msg := runRequireCI(t, "v1.0.0-pending", tagSHA)
		if code != 1 || strings.HasPrefix(msg, "release-gate: pending") || strings.HasPrefix(msg, "release-gate: no matching run") {
			t.Fatalf("code %d\n%s", code, msg)
		}
	})
	t.Run("prerelease in progress", func(t *testing.T) {
		const tag = "v1.0.0-pending"
		installFakeGH(t, listRunJSON(tagSHA, tag, "in_progress", "", "push"), viewFail)
		dispatchEnv(t)
		code, msg := runRequireCI(t, tag, tagSHA)
		if code != exitRetryable || !strings.HasPrefix(msg, "release-gate: pending ") || !strings.Contains(msg, tag) {
			t.Fatalf("code %d\n%s", code, msg)
		}
		if msg != "release-gate: pending CI run for tag v1.0.0-pending\n" {
			t.Fatalf("stderr %q", msg)
		}
	})
	t.Run("vpending", func(t *testing.T) {
		installFakeGH(t, listRunJSON(tagSHA, "vpending", "completed", "success", "push"), viewFail)
		dispatchEnv(t)
		err := requireGreenCI("vpending", tagSHA)
		if err == nil || retryable(err) || strings.Contains(err.Error(), "vpending") {
			t.Fatalf("err=%v", err)
		}
		code, msg := runRequireCI(t, "vpending", tagSHA)
		if code != 1 || strings.Contains(msg, "vpending") || !strings.Contains(msg, "-tag is not a release tag") {
			t.Fatalf("code %d\n%s", code, msg)
		}
	})
	t.Run("duplicate failure then success", func(t *testing.T) {
		const tag = "v1.2.3"
		// Failure is first. A last-wins map would keep success and pass.
		body := viewJobs(map[string][]string{"unit": {"failure", "success"}}, "")
		installFakeGH(t, listRunJSON(tagSHA, tag, "completed", "success", "push"), "cat <<'EOF'\n"+body+"\nEOF\nexit 0\n")
		dispatchEnv(t)
		code, msg := runRequireCI(t, tag, tagSHA)
		if code != 1 {
			t.Fatalf("code %d\n%s", code, msg)
		}
		if !strings.Contains(msg, "exactly once") || !strings.Contains(msg, "unit=failure") {
			t.Fatalf("stderr %q", msg)
		}
		if strings.HasPrefix(msg, "release-gate: pending") || strings.HasPrefix(msg, "release-gate: no matching run") {
			t.Fatalf("stderr %q", msg)
		}
	})
	t.Run("duplicate both success", func(t *testing.T) {
		const tag = "v1.2.3"
		body := viewJobs(map[string][]string{"unit": {"success", "success"}}, "")
		installFakeGH(t, listRunJSON(tagSHA, tag, "completed", "success", "push"), "cat <<'EOF'\n"+body+"\nEOF\nexit 0\n")
		dispatchEnv(t)
		code, msg := runRequireCI(t, tag, tagSHA)
		if code != 1 {
			t.Fatalf("code %d\n%s", code, msg)
		}
		if !strings.Contains(msg, "exactly once") || strings.Contains(msg, "unit=failure") {
			t.Fatalf("stderr %q", msg)
		}
		if strings.HasPrefix(msg, "release-gate: pending") || strings.HasPrefix(msg, "release-gate: no matching run") {
			t.Fatalf("stderr %q", msg)
		}
	})
	t.Run("missing job", func(t *testing.T) {
		const tag = "v1.2.3"
		body := viewJobs(nil, "web")
		installFakeGH(t, listRunJSON(tagSHA, tag, "completed", "success", "push"), "cat <<'EOF'\n"+body+"\nEOF\nexit 0\n")
		dispatchEnv(t)
		code, msg := runRequireCI(t, tag, tagSHA)
		if code != 1 {
			t.Fatalf("code %d\n%s", code, msg)
		}
		if !strings.Contains(msg, "web=missing") || !strings.Contains(msg, "not green") {
			t.Fatalf("stderr %q", msg)
		}
	})
	t.Run("extra failed job ignored", func(t *testing.T) {
		const tag = "v1.2.3"
		body := viewJobs(map[string][]string{"apidiff": {"failure"}}, "")
		installFakeGH(t, listRunJSON(tagSHA, tag, "completed", "success", "push"), "cat <<'EOF'\n"+body+"\nEOF\nexit 0\n")
		dispatchEnv(t)
		code, msg := runRequireCI(t, tag, tagSHA)
		if code != 0 {
			t.Fatalf("code %d\n%s", code, msg)
		}
	})
}

// viewJobs builds a gh run view --json jobs document.
// For each required name other than skip, it emits one success job, or
// every conclusion in conclusions[name] in slice order when that key is set.
// conclusions keys that are not required names are appended.
func viewJobs(conclusions map[string][]string, skip string) string {
	required := map[string]bool{}
	for _, name := range requiredCIJobs {
		required[name] = true
	}
	var b strings.Builder
	b.WriteString(`{"jobs":[`)
	first := true
	write := func(name, conclusion string) {
		if !first {
			b.WriteByte(',')
		}
		first = false
		fmt.Fprintf(&b, `{"name":%q,"conclusion":%q}`, name, conclusion)
	}
	for _, name := range requiredCIJobs {
		if name == skip {
			continue
		}
		cs, ok := conclusions[name]
		if !ok {
			cs = []string{"success"}
		}
		for _, c := range cs {
			write(name, c)
		}
	}
	extras := make([]string, 0, len(conclusions))
	for name := range conclusions {
		if !required[name] {
			extras = append(extras, name)
		}
	}
	sort.Strings(extras)
	for _, name := range extras {
		for _, c := range conclusions[name] {
			write(name, c)
		}
	}
	b.WriteString(`]}`)
	return b.String()
}

func runRequireCI(t *testing.T, tag, sha string) (int, string) {
	t.Helper()
	var errb bytes.Buffer
	code := run([]string{"-require-ci", "-tag", tag, "-sha", sha}, &errb)
	return code, errb.String()
}

// listRunJSON is one gh run list row. headSha is the -sha passed to the gate.
func listRunJSON(headSha, headBranch, status, conclusion, event string) string {
	return fmt.Sprintf(`[{"databaseId":11,"conclusion":%q,"status":%q,"headSha":%q,"event":%q,"headBranch":%q}]`,
		conclusion, status, headSha, event, headBranch)
}

// installGHListFails is a gh that prints the old retry words and exits 1.
func installGHListFails(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	script := "#!/bin/sh\necho pending >&2\necho 'no matching run' >&2\nexit 1\n"
	if err := os.WriteFile(filepath.Join(dir, "gh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+":/usr/bin:/bin")
}
