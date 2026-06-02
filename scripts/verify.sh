#!/usr/bin/env bash

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

go test -count=1 ./...
go vet ./...

pushd frontend >/dev/null
npm run test
npm run build
popd >/dev/null
