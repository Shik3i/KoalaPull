#!/usr/bin/env bash

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

pushd frontend >/dev/null
npm run test
npm run build
popd >/dev/null

go test -count=1 ./...
go vet ./...

if command -v ruby >/dev/null 2>&1; then
    ruby -ryaml -e "YAML.load_file('.github/workflows/release.yml')" >/dev/null
    echo "Workflow YAML syntax verified."
else
    echo "Warning: ruby not found, skipping workflow YAML verification."
fi

