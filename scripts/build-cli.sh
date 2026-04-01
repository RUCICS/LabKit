#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT_DIR="${OUT_DIR:-$ROOT_DIR/dist}"
PKG="./apps/cli/cmd/labkit"
LDFLAGS="${LDFLAGS:--s -w -buildid=}"

mkdir -p "$OUT_DIR"

if [[ $# -eq 0 ]]; then
  set -- \
    linux/amd64 \
    linux/arm64 \
    darwin/arm64 \
    darwin/amd64 \
    windows/amd64
fi

build_target() {
  local target="$1"
  local goos="${target%%/*}"
  local goarch="${target##*/}"
  local ext=""

  if [[ "$goos" == "$goarch" ]]; then
    echo "invalid target: $target (expected GOOS/GOARCH)" >&2
    exit 1
  fi
  if [[ "$goos" == "windows" ]]; then
    ext=".exe"
  fi

  local output="$OUT_DIR/labkit-${goos}-${goarch}${ext}"
  echo "==> building $goos/$goarch -> ${output#$ROOT_DIR/}"
  (
    cd "$ROOT_DIR"
    CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
      go build -trimpath -ldflags="$LDFLAGS" -o "$output" "$PKG"
  )
}

for target in "$@"; do
  build_target "$target"
done
