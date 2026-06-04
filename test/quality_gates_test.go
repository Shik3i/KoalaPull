package test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCanonicalVerifierRunsRequiredChecks(t *testing.T) {
	data := mustReadRepoFile(t, "scripts", "verify.mjs")
	for _, want := range []string{
		`run("npm", ["run", "test"]`,
		`run("npx", ["tsc", "--noEmit"]`,
		`run("npm", ["run", "build"]`,
		`run("npm", ["audit", "--audit-level=moderate"]`,
		`run("node", ["--test"]`,
		`run("go", ["test", "-count=1", "./..."]`,
		`run("go", ["test", "-race", "-count=1", "./..."]`,
		`run("go", ["vet", "./..."]`,
		`govulncheck@v1.3.0`,
	} {
		if !strings.Contains(data, want) {
			t.Fatalf("scripts/verify.mjs missing %q", want)
		}
	}
	if strings.Index(data, `run("npm", ["run", "build"]`) > strings.Index(data, `run("go", ["test", "-count=1", "./..."]`) {
		t.Fatal("scripts/verify.mjs must build frontend before go test so embedded frontend/dist exists")
	}
}

func TestVerifyLaunchersUseCanonicalVerifier(t *testing.T) {
	for _, launcher := range []string{"verify.sh", "verify.bat"} {
		data := mustReadRepoFile(t, "scripts", launcher)
		if !strings.Contains(data, "verify.mjs") {
			t.Fatalf("scripts/%s must use scripts/verify.mjs", launcher)
		}
		for _, duplicated := range []string{"go test", "npm run", "govulncheck"} {
			if strings.Contains(data, duplicated) {
				t.Fatalf("scripts/%s duplicates verifier command %q", launcher, duplicated)
			}
		}
	}
}

func TestMakefileUsesCanonicalVerifier(t *testing.T) {
	data := mustReadRepoFile(t, "Makefile")
	if !strings.Contains(data, "verify:") {
		t.Fatal("Makefile missing verify target")
	}
	if !strings.Contains(data, "node scripts/verify.mjs") {
		t.Fatal("Makefile verify target must use scripts/verify.mjs")
	}
}

func TestCIWorkflowRunsForPullRequestsAndMainPushes(t *testing.T) {
	data := mustReadRepoFile(t, ".github", "workflows", "ci.yml")
	for _, want := range []string{
		"workflow_dispatch:",
		"pull_request:",
		"push:",
		"- main",
		"node scripts/verify.mjs",
	} {
		if !strings.Contains(data, want) {
			t.Fatalf(".github/workflows/ci.yml missing %q", want)
		}
	}
	if strings.Contains(data, "tags:") {
		t.Fatal(".github/workflows/ci.yml must not run for version tags")
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
	for _, want := range []string{"name: Verify", "node scripts/verify.mjs"} {
		if !strings.Contains(data, want) {
			t.Fatalf("release workflow verification missing %q", want)
		}
	}
}

func TestCIWorkflowInstallsLinuxSystemDepsBeforeVerify(t *testing.T) {
	data := mustReadRepoFile(t, ".github", "workflows", "ci.yml")
	deps := strings.Index(data, "Install Linux system deps")
	verify := strings.Index(data, "run: node scripts/verify.mjs")
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
	verify := strings.Index(data, "run: node scripts/verify.mjs")
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
		"sha256sum \"$asset_name\"",
		"shasum -a 256 \"$asset_name\"",
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

func TestWorkflowsUsePatchedToolchains(t *testing.T) {
	for _, wf := range []string{"release.yml", "ci.yml"} {
		data := mustReadRepoFile(t, ".github", "workflows", wf)
		for _, want := range []string{
			"go-version: \"1.26.4\"",
			"node-version: \"22\"",
		} {
			if !strings.Contains(data, want) {
				t.Fatalf("%s missing %q", wf, want)
			}
		}
	}
}

func TestGoModuleRequiresPatchedToolchain(t *testing.T) {
	data := mustReadRepoFile(t, "go.mod")
	if !strings.Contains(data, "go 1.26.4") {
		t.Fatal("go.mod must require patched Go 1.26.4 or newer")
	}
}

func TestReleaseWorkflowUsesLeastPrivilege(t *testing.T) {
	data := strings.ReplaceAll(mustReadRepoFile(t, ".github", "workflows", "release.yml"), "\r\n", "\n")
	read := strings.Index(data, "permissions:\n  contents: read")
	release := strings.Index(data, "release:\n")
	write := strings.LastIndex(data, "permissions:\n      contents: write")
	if read == -1 || release == -1 || write < release {
		t.Fatal("release workflow must default to contents: read and grant contents: write only to release job")
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

func TestWorkflowActionsUseImmutableSHAs(t *testing.T) {
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
			// Immutable action refs prevent mutable-tag supply-chain changes.
			if len(ref) != 40 || !isAllHex(ref) {
				t.Fatalf("%s line %d: action ref %q must be an immutable 40-character SHA", wf, i+1, ref)
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
