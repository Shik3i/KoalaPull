package test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerifyScriptExistsAndRunsRequiredChecks(t *testing.T) {
	data := mustReadRepoFile(t, "scripts", "verify.sh")
	for _, want := range []string{
		"npm run test",
		"npm run build",
		"go test -count=1 ./...",
		"go vet ./...",
	} {
		if !strings.Contains(data, want) {
			t.Fatalf("scripts/verify.sh missing %q", want)
		}
	}
	if strings.Index(data, "npm run build") > strings.Index(data, "go test -count=1 ./...") {
		t.Fatal("scripts/verify.sh must build frontend before go test so embedded frontend/dist exists")
	}
}

func TestMakefileUsesSharedVerifyScript(t *testing.T) {
	data := mustReadRepoFile(t, "Makefile")
	if !strings.Contains(data, "verify:") {
		t.Fatal("Makefile missing verify target")
	}
	if !strings.Contains(data, "./scripts/verify.sh") {
		t.Fatal("Makefile verify target must use scripts/verify.sh")
	}
}

func TestCIWorkflowRunsVerificationOnPushAndPullRequest(t *testing.T) {
	data := mustReadRepoFile(t, ".github", "workflows", "ci.yml")
	for _, want := range []string{
		"pull_request:",
		"push:",
		"./scripts/verify.sh",
	} {
		if !strings.Contains(data, want) {
			t.Fatalf(".github/workflows/ci.yml missing %q", want)
		}
	}
}

func TestReleaseWorkflowRunsVerificationBeforeBuild(t *testing.T) {
	data := mustReadRepoFile(t, ".github", "workflows", "release.yml")
	if !strings.Contains(data, "./scripts/verify.sh") {
		t.Fatal("release workflow must run scripts/verify.sh before release build")
	}
}

func TestFrontendPackageHasAutomatedTestScript(t *testing.T) {
	data := mustReadRepoFile(t, "frontend", "package.json")
	if !strings.Contains(data, "\"test\"") {
		t.Fatal("frontend/package.json missing test script")
	}
}

func TestAgentsDefinesNoPushWithoutVerification(t *testing.T) {
	data := mustReadRepoFile(t, "AGENTS.md")
	for _, want := range []string{
		"No push before running `./scripts/verify.sh`.",
		"Only push when `go test -count=1 ./...`, `go vet ./...`, `npm run test`, and `npm run build` all pass.",
	} {
		if !strings.Contains(data, want) {
			t.Fatalf("AGENTS.md missing %q", want)
		}
	}
}

func mustReadRepoFile(t *testing.T, parts ...string) string {
	t.Helper()
	path := filepath.Join(append([]string{".."}, parts...)...)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}
