package main

import (
	"fmt"
	"os"
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
  {"name":"diff-transcript","conclusion":"success"},
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
	err := requireGreenCI()
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
	if err := requireGreenCI(); err != nil {
		t.Fatal(err)
	}
}

func TestRequireCIRejectsMainPushSameSHA(t *testing.T) {
	list := `[{"databaseId":11,"conclusion":"success","status":"completed","headSha":"abc","event":"push","headBranch":"main"}]`
	installFakeGH(t, list, "cat <<'EOF'\n"+greenJobsJSON+"\nEOF\nexit 0\n")
	setTagEnv(t)
	err := requireGreenCI()
	if err == nil || !strings.Contains(err.Error(), "no matching run") {
		t.Fatalf("err=%v", err)
	}
}

func TestRequireCIRejectsPullRequestFallback(t *testing.T) {
	list := `[{"databaseId":11,"conclusion":"success","status":"completed","headSha":"abc","event":"pull_request","headBranch":"v1.2.3"}]`
	installFakeGH(t, list, "cat <<'EOF'\n"+greenJobsJSON+"\nEOF\nexit 0\n")
	setTagEnv(t)
	err := requireGreenCI()
	if err == nil || !strings.Contains(err.Error(), "no matching run") {
		t.Fatalf("err=%v", err)
	}
}

func TestRequireCIPendingTagRun(t *testing.T) {
	list := `[{"databaseId":22,"conclusion":"","status":"in_progress","headSha":"abc","event":"push","headBranch":"v1.2.3"},{"databaseId":11,"conclusion":"success","status":"completed","headSha":"abc","event":"push","headBranch":"v1.2.3"}]`
	installFakeGH(t, list, "cat <<'EOF'\n"+greenJobsJSON+"\nEOF\nexit 0\n")
	setTagEnv(t)
	err := requireGreenCI()
	if err == nil || !strings.Contains(err.Error(), "pending") {
		t.Fatalf("err=%v", err)
	}
}

// An older in-progress tag run must not block a newer completed green one.
func TestRequireCIOlderInProgressDoesNotBlockNewerGreen(t *testing.T) {
	list := `[{"databaseId":11,"conclusion":"","status":"in_progress","headSha":"abc","event":"push","headBranch":"v1.2.3"},{"databaseId":22,"conclusion":"success","status":"completed","headSha":"abc","event":"push","headBranch":"v1.2.3"}]`
	installFakeGH(t, list, "cat <<'EOF'\n"+greenJobsJSON+"\nEOF\nexit 0\n")
	setTagEnv(t)
	if err := requireGreenCI(); err != nil {
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
