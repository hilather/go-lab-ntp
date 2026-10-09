package main

import (
	"strings"
	"testing"
)

// The release gate must refuse a tag run whose diff-transcript job is
// missing or not green, even when every other required job passed.
func TestRequireCIRejectsTagRunWithoutGreenDiffTranscript(t *testing.T) {
	cases := map[string]string{
		"missing": strings.Replace(greenJobsJSON, `  {"name":"diff-transcript","conclusion":"success"},`+"\n", "", 1),
		"failure": strings.Replace(greenJobsJSON, `{"name":"diff-transcript","conclusion":"success"}`, `{"name":"diff-transcript","conclusion":"failure"}`, 1),
	}
	for name, jobs := range cases {
		t.Run(name, func(t *testing.T) {
			if jobs == greenJobsJSON {
				t.Fatal("fixture edit did not apply")
			}
			list := `[{"databaseId":11,"conclusion":"success","status":"completed","headSha":"abc","event":"push","headBranch":"v1.2.3"}]`
			installFakeGH(t, list, "cat <<'EOF'\n"+jobs+"\nEOF\nexit 0\n")
			setTagEnv(t)
			err := requireGreenCI()
			if err == nil || !strings.Contains(err.Error(), "diff-transcript=") {
				t.Fatalf("err=%v, want a required-job failure naming diff-transcript", err)
			}
		})
	}
}
