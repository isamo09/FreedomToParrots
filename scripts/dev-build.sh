#!/usr/bin/env bash
# Local dev build: compiles the olcrtc tunnel core (from a sibling
# ../source checkout if present, otherwise a fresh clone of
# openlibrecommunity/olcrtc), embeds it, and builds ./cmd/fzp for the host
# platform (or GOOS/GOARCH if set). This is what the release workflow does
# per-target in CI; use this to test a real, working build locally.
set -euo pipefail
cd "$(dirname "$0")/.."
root="$(pwd)"

GOOS="${GOOS:-$(go env GOHOSTOS)}"
GOARCH="${GOARCH:-$(go env GOHOSTARCH)}"
CORE_SRC="${CORE_SRC:-../source}"

if [ ! -d "$CORE_SRC/cmd/olcrtc" ]; then
  echo "no olcrtc core source at $CORE_SRC - cloning openlibrecommunity/olcrtc"
  CORE_SRC="$(mktemp -d)/olcrtc-core"
  git clone --depth 1 https://github.com/openlibrecommunity/olcrtc "$CORE_SRC"
fi

ext=""
dest="$root/internal/corebin/bin/core"
if [ "$GOOS" = "windows" ]; then
  ext=".exe"
  dest="$root/internal/corebin/bin/core.exe"
fi

echo "building core ($GOOS/$GOARCH) from $CORE_SRC -> $dest"
rm -f "$dest"
(cd "$CORE_SRC" && GOOS="$GOOS" GOARCH="$GOARCH" CGO_ENABLED=0 \
  go build -trimpath -ldflags "-s -w" -o "$dest" ./cmd/olcrtc)

if [ "$GOOS" = "windows" ] && command -v go-winres >/dev/null 2>&1; then
  echo "refreshing Windows icon/version resource"
  (cd "$root" && go-winres simply --arch "$GOARCH" --icon build/icon/icon-square.png \
    --manifest cli --file-description "Freedom To Parrots" --product-name "Freedom To Parrots" \
    --out cmd/fzp/rsrc)
elif [ "$GOOS" = "windows" ]; then
  echo "go-winres not installed - keeping the committed cmd/fzp/rsrc_windows_*.syso as-is" \
    "(go install github.com/tc-hib/go-winres@latest to refresh it)"
fi

mkdir -p "$root/dist"
out="$root/dist/FreedomToParrots-$GOOS-$GOARCH$ext"
echo "building fzp -> $out"
GOOS="$GOOS" GOARCH="$GOARCH" CGO_ENABLED=0 \
  go build -trimpath -ldflags "-s -w -X main.version=dev" -o "$out" ./cmd/fzp

echo "done: $out"
echo
echo "NOTE: internal/corebin/bin/core$ext now holds a real binary - don't"
echo "git add it. Restore the placeholder before committing:"
echo "  printf 'freedomtoparrots-dev-placeholder\\n' > internal/corebin/bin/core$ext"
