#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SOURCE_DIR="${ROOT_DIR}/proto/agent/v1"
TARGET_REPO="${1:-${HUB_REPO:-}}"

if [[ -z "${TARGET_REPO}" ]]; then
  echo "usage: $0 <hub-repo-path>"
  echo "or set HUB_REPO=/path/to/hub"
  exit 1
fi

TARGET_DIR="${TARGET_REPO}/proto/agent/v1"

mkdir -p "${TARGET_DIR}"
rsync -a --delete "${SOURCE_DIR}/" "${TARGET_DIR}/"

if command -v gofmt >/dev/null 2>&1; then
  gofmt -w "${TARGET_DIR}"/*.go
fi

echo "synced proto/agent/v1 -> ${TARGET_DIR}"
