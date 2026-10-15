#!/bin/sh
set -eu
cd "$(dirname "$0")/.."

VERSION="$(sed -n 's/^[[:space:]]*Version = "\([^"]*\)".*/\1/p' internal/appinfo/version.go | head -1)"
if [ -z "$VERSION" ]; then
  echo "cannot read version" >&2
  exit 1
fi

python3 scripts/genicon.py

GV="$(go env GOPATH)/bin/goversioninfo"
if [ ! -x "$GV" ]; then
  GOPROXY="${GOPROXY:-https://goproxy.cn,https://proxy.golang.org,direct}" \
    GOTOOLCHAIN=local go install github.com/josephspurrier/goversioninfo/cmd/goversioninfo@v1.4.0
fi

(
  cd cmd/cclear
  "$GV" -64 \
    -icon ../../assets/cclear.ico \
    -manifest ../../assets/cclear.manifest \
    -o rsrc_windows_amd64.syso
)

(
  cd cmd/setup
  "$GV" -64 \
    -icon ../../assets/cclear.ico \
    -manifest ../../assets/setup.manifest \
    -o rsrc_windows_amd64.syso
)

mkdir -p dist
rm -f dist/cclear.exe "dist/CClear-Setup-${VERSION}.exe" "dist/cclear-${VERSION}.exe"
GOTOOLCHAIN=local CGO_ENABLED=0 GOOS=windows GOARCH=amd64 \
  go build -trimpath -ldflags "-s -w -H windowsgui" -o dist/cclear.exe ./cmd/cclear
GOTOOLCHAIN=local CGO_ENABLED=0 GOOS=windows GOARCH=amd64 \
  go build -trimpath -ldflags "-s -w -H windowsgui" -o "dist/CClear-Setup-${VERSION}.exe" ./cmd/setup
GOTOOLCHAIN=local go run ./cmd/pack "dist/CClear-Setup-${VERSION}.exe" dist/cclear.exe
cp dist/cclear.exe "dist/cclear-${VERSION}.exe"

python3 - "$VERSION" <<'PY'
import sys
from pathlib import Path
version = sys.argv[1]
setup = Path(f"dist/CClear-Setup-{version}.exe").read_bytes()
app = Path("dist/cclear.exe").read_bytes()
magic = b"CCLEARSETUP1"
footer = len(magic) + 8
tail = setup[-footer:]
if tail[:len(magic)] != magic:
    raise SystemExit("setup payload footer missing")
size = int.from_bytes(tail[len(magic):], "little")
embedded = setup[-footer - size:-footer]
if embedded != app or embedded[:2] != b"MZ" or setup[:2] != b"MZ":
    raise SystemExit("setup payload does not match cclear.exe")
if b"asInvoker" not in app or b"requireAdministrator" in app:
    raise SystemExit("cclear.exe manifest is not asInvoker")
ver = version.encode()
if ver not in app or ver not in setup:
    raise SystemExit(f"version {version} missing from the executables")
if b"requireAdministrator" not in setup:
    raise SystemExit("setup manifest is missing requireAdministrator")
print(f"Built dist/cclear.exe ({len(app)} bytes) and dist/CClear-Setup-{version}.exe ({len(setup)} bytes)")
PY
