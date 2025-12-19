#!/usr/bin/env bash
set -euo pipefail

OUTPUT_DIR=${OUTPUT_DIR:-bin}
APP_NAME=${APP_NAME:-r3e-leaderboard}
TARGETS=(
  linux-amd64
  linux-386
  linux-arm64
  linux-armv7
  linux-armv6
)

mkdir -p "$OUTPUT_DIR"

build_target() {
  local target="$1"
  local goos="${target%%-*}"
  local arch="${target##*-}"
  local out_name="$APP_NAME-$goos-$arch"

  export CGO_ENABLED=0
  export GOOS="$goos"

  case "$arch" in
    armv7)
      export GOARCH=arm
      export GOARM=7
      ;;
    armv6)
      export GOARCH=arm
      export GOARM=6
      ;;
    *)
      export GOARCH="$arch"
      ;;
  esac

  echo "Building $target -> $OUTPUT_DIR/$out_name"
  go build -trimpath -ldflags "-s -w" -o "$OUTPUT_DIR/$out_name"
  echo "Built: $OUTPUT_DIR/$out_name"
}

for t in "${TARGETS[@]}"; do
  build_target "$t"
done
