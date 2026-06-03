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

func TestCIWorkflowInstallsLinuxSystemDepsBeforeVerify(t *testing.T) {
	data := mustReadRepoFile(t, ".github", "workflows", "ci.yml")
	deps := strings.Index(data, "Install Linux system deps")
	verify := strings.Index(data, "run: ./scripts/verify.sh")
	if deps == -1 {
		t.Fatal(".github/workflows/ci.yml missing Linux system dependency install step")
	}
	if verify == -1 {
		t.Fatal(".github/workflows/ci.yml missing verify step")
	}
	if deps > verify {
		t.Fatal(".github/workflows/ci.yml must install Linux system dependencies before verify")
	}
}

func TestReleaseWorkflowInstallsLinuxSystemDepsBeforeVerify(t *testing.T) {
	data := mustReadRepoFile(t, ".github", "workflows", "release.yml")
	deps := strings.Index(data, "Install Linux system deps")
	verify := strings.Index(data, "run: ./scripts/verify.sh")
	if deps == -1 {
		t.Fatal(".github/workflows/release.yml missing Linux system dependency install step")
	}
	if verify == -1 {
		t.Fatal(".github/workflows/release.yml missing verify step")
	}
	if deps > verify {
		t.Fatal(".github/workflows/release.yml must install Linux system dependencies before verify")
	}
}

func TestReleaseWorkflowPackagesArtifactsBeforeUpload(t *testing.T) {
	data := mustReadRepoFile(t, ".github", "workflows", "release.yml")
	for _, want := range []string{
		"name: Package artifact",
		"asset_name=",
		"artifact_path=",
		"files: release-assets/*",
	} {
		if !strings.Contains(data, want) {
			t.Fatalf("release workflow missing %q", want)
		}
	}
	if strings.Contains(data, "files: build/bin/*") {
		t.Fatal("release workflow must not upload raw build/bin/* paths")
	}
}

func TestReleaseWorkflowUsesWebkit241TagForLinuxBuild(t *testing.T) {
	data := mustReadRepoFile(t, ".github", "workflows", "release.yml")
	for _, want := range []string{
		"build_tags=",
		"webkit2_41",
		"wails build -clean -tags \"$build_tags\"",
	} {
		if !strings.Contains(data, want) {
			t.Fatalf("release workflow missing %q", want)
		}
	}
}

func TestReleaseWorkflowUsesBashForInjectVersionStep(t *testing.T) {
	data := mustReadRepoFile(t, ".github", "workflows", "release.yml")
	for _, want := range []string{
		"name: Inject version into wails.json",
		"shell: bash",
		"VER=\"${{ github.ref_name }}\"",
	} {
		if !strings.Contains(data, want) {
			t.Fatalf("release workflow missing %q", want)
		}
	}
}

func TestFrontendPackageHasAutomatedTestScript(t *testing.T) {
	data := mustReadRepoFile(t, "frontend", "package.json")
	if !strings.Contains(data, "\"test\"") {
		t.Fatal("frontend/package.json missing test script")
	}
}

func TestFrontendCapsHistoryRowsRenderedAtOnce(t *testing.T) {
	data := mustReadRepoFile(t, "frontend", "src", "App.tsx")
	for _, want := range []string{
		"maxVisibleHistoryEntries",
		"filtered.slice(0, maxVisibleHistoryEntries)",
		"history.showingLimited",
	} {
		if !strings.Contains(data, want) {
			t.Fatalf("frontend/src/App.tsx missing %q", want)
		}
	}
}

func TestFrontendShowsStartDownloadErrors(t *testing.T) {
	data := mustReadRepoFile(t, "frontend", "src", "App.tsx")
	for _, want := range []string{
		"addQueueError",
		"setAddQueueError",
		"errors.startDownloadFailed",
	} {
		if !strings.Contains(data, want) {
			t.Fatalf("frontend/src/App.tsx missing %q", want)
		}
	}
}

func TestAgentsDefinesNoPushWithoutVerification(t *testing.T) {
	data := mustReadRepoFile(t, "AGENTS.md")
	for _, want := range []string{
		"Before ANY push:",
		"MUST run `./scripts/verify.sh`",
		"Before ANY release tag push (HARD VIOLATION):",
		"cross-compilation for ALL 3 target platforms",
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

func TestWorkflowActionsUseVersionTagsNotRawSHAs(t *testing.T) {
	for _, wf := range []string{"release.yml", "ci.yml"} {
		data := mustReadRepoFile(t, ".github", "workflows", wf)
		for i, line := range strings.Split(data, "\n") {
			trimmed := strings.TrimSpace(line)
			if !strings.HasPrefix(trimmed, "uses:") {
				continue
			}
			// Extract the ref after the @ sign
			atIdx := strings.LastIndex(trimmed, "@")
			if atIdx == -1 {
				continue
			}
			ref := strings.TrimSpace(trimmed[atIdx+1:])
			// Strip inline comments like "# v4"
			if commentIdx := strings.Index(ref, "#"); commentIdx != -1 {
				ref = strings.TrimSpace(ref[:commentIdx])
			}
			// Reject 40-char hex strings — these are almost always hallucinated by AI
			if len(ref) == 40 && isAllHex(ref) {
				t.Fatalf("%s line %d: action pinned to raw 40-char SHA %q — use a version tag (@v4) instead. Raw SHAs are frequently hallucinated by AI and unverifiable without network access.", wf, i+1, ref)
			}
		}
	}
}

func isAllHex(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}
