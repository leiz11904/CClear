#!/bin/sh
# Signs dist/cclear.exe and the current setup if CCLEAR_CERT points to a PFX.
# This script does not create a certificate and does not hide SmartScreen.
set -eu
cd "$(dirname "$0")/.."

VERSION="$(sed -n 's/^[[:space:]]*Version = "\([^"]*\)".*/\1/p' internal/appinfo/version.go | head -1)"
APP="dist/cclear.exe"
SETUP="dist/CClear-Setup-${VERSION}.exe"

if [ -z "${CCLEAR_CERT:-}" ]; then
  echo "CCLEAR_CERT is empty; leaving $APP unsigned."
  echo "SmartScreen may warn on first run. Use '更多信息' -> '仍要运行'."
  exit 0
fi

if [ ! -f "$APP" ] || [ ! -f "$SETUP" ]; then
  echo "build the Windows binaries first: sh scripts/build-windows.sh" >&2
  exit 1
fi

if ! command -v osslsigncode >/dev/null 2>&1; then
  echo "osslsigncode is required to sign on this host" >&2
  exit 1
fi

sign() {
  src=$1
  tmp=$1.signed
  osslsigncode sign -pkcs12 "$CCLEAR_CERT" \
    ${CCLEAR_CERT_PASS:+-pass "$CCLEAR_CERT_PASS"} \
    -n "CClear" -i "https://example.invalid/cclear" \
    -in "$src" -out "$tmp"
  mv "$tmp" "$src"
}

sign "$APP"
sign "$SETUP"
cp "$APP" "dist/cclear-${VERSION}.exe"
echo "Signed $APP and $SETUP"
