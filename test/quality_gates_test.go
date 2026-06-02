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

func TestCIWorkflowIsManualOnly(t *testing.T) {
	data := mustReadRepoFile(t, ".github", "workflows", "ci.yml")
	for _, want := range []string{
		"workflow_dispatch:",
		"./scripts/verify.sh",
	} {
		if !strings.Contains(data, want) {
			t.Fatalf(".github/workflows/ci.yml missing %q", want)
		}
	}
	for _, forbidden := range []string{
		"pull_request:",
		"push:",
		"tags:",
	} {
		if strings.Contains(data, forbidden) {
			t.Fatalf(".github/workflows/ci.yml must not contain %q", forbidden)
		}
	}
}

func TestReleaseWorkflowOnlyRunsOnVersionTags(t *testing.T) {
	data := mustReadRepoFile(t, ".github", "workflows", "release.yml")
	if !strings.Contains(data, "tags:") || !strings.Contains(data, "- \"v*\"") {
		t.Fatal("release workflow must only trigger on v* tags")
	}
	for _, forbidden := range []string{
		"pull_request:",
		"branches:",
	} {
		if strings.Contains(data, forbidden) {
			t.Fatalf("release workflow must not contain %q", forbidden)
		}
	}
}

func TestReleaseWorkflowRunsVerificationBeforeBuild(t *testing.T) {
	data := mustReadRepoFile(t, ".github", "workflows", "release.yml")
	if !strings.Contains(data, "./scripts/verify.sh") {
		t.Fatal("release workflow must run scripts/verify.sh before release build")
	}
}

func TestReleaseWorkflowPackagesArtifactsBeforeUpload(t *testing.T) {
	data := mustReadRepoFile(t, ".github", "workflows", "release.yml")
	for _, want := range []string{
		"name: Package artifact",
		"asset_name=",
		"artifact_path=",
		"files: ${{ steps.package.outputs.artifact_path }}",
	} {
		if !strings.Contains(data, want) {
			t.Fatalf("release workflow missing %q", want)
		}
	}
	if strings.Contains(data, "files: build/bin/*") {
		t.Fatal("release workflow must not upload raw build/bin/* paths")
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
