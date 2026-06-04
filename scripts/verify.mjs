import { spawnSync } from "node:child_process"
import { dirname, resolve } from "node:path"
import { fileURLToPath } from "node:url"

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..")
const frontendDir = resolve(repoRoot, "frontend")
const websiteDir = resolve(repoRoot, "website")
const isWindows = process.platform === "win32"

function invocation(name, args) {
  if (isWindows && (name === "npm" || name === "npx")) {
    return {
      name: process.env.ComSpec || "cmd.exe",
      args: ["/d", "/c", `${name}.cmd`, ...args],
    }
  }
  return { name, args }
}

function run(name, args, options = {}) {
  console.log(`\n> ${name} ${args.join(" ")}`)
  const command = invocation(name, args)
  const result = spawnSync(command.name, command.args, {
    cwd: options.cwd ?? repoRoot,
    env: { ...process.env, ...options.env },
    stdio: "inherit",
    shell: false,
  })
  if (result.error) throw result.error
  if (result.status !== 0) process.exit(result.status ?? 1)
}

function output(name, args) {
  const command = invocation(name, args)
  const result = spawnSync(command.name, command.args, {
    cwd: repoRoot,
    encoding: "utf8",
    shell: false,
  })
  if (result.error || result.status !== 0) return ""
  return result.stdout.trim()
}

function commandWorks(name, args = ["--version"]) {
  const command = invocation(name, args)
  const result = spawnSync(command.name, command.args, {
    cwd: repoRoot,
    stdio: "ignore",
    shell: false,
  })
  return !result.error && result.status === 0
}

run("npm", ["ci", "--include=optional"], { cwd: frontendDir })
run("npm", ["run", "test"], { cwd: frontendDir })
run("npx", ["tsc", "--noEmit"], { cwd: frontendDir })
run("npm", ["run", "build"], { cwd: frontendDir })
run("npm", ["audit", "--audit-level=moderate"], { cwd: frontendDir })

run("npm", ["ci", "--include=optional"], { cwd: repoRoot })
run("node", ["--test"], { cwd: websiteDir })

run("go", ["test", "-count=1", "./..."])

const goos = output("go", ["env", "GOOS"])
const cc = output("go", ["env", "CC"]) || "gcc"
if (["linux", "windows", "darwin"].includes(goos) && commandWorks(cc)) {
  run("go", ["test", "-race", "-count=1", "./..."], { env: { CGO_ENABLED: "1" } })
} else {
  console.warn(`Race detector skipped: C compiler "${cc}" not available for ${goos || "unknown OS"}.`)
}

run("go", ["vet", "./..."])
run("go", ["run", "golang.org/x/vuln/cmd/govulncheck@v1.3.0", "./..."])
run("go", [
  "run",
  "github.com/rhysd/actionlint/cmd/actionlint@v1.7.12",
  ".github/workflows/ci.yml",
  ".github/workflows/release.yml",
])
