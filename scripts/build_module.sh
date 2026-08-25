#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
DIST_DIR="${ROOT_DIR}/dist"
STAGE_DIR="${DIST_DIR}/staging"
ZIP_OUTPUT="${DIST_DIR}/netclient-magisk.zip"

WITH_WG=false
for arg in "$@"; do
    case $arg in
        --with-wg)
            WITH_WG=true
            shift
            ;;
    esac
done

echo "============="
echo " Building... "
echo "============="

rm -rf "${STAGE_DIR}" "${ZIP_OUTPUT}"
mkdir -p "${DIST_DIR}" "${STAGE_DIR}/system/bin"

echo "==> Building netclient for linux/arm64..."
cd "${ROOT_DIR}"
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -ldflags="-s -w" -o "${STAGE_DIR}/system/bin/netclient" .
chmod 0755 "${STAGE_DIR}/system/bin/netclient"
echo "    [✓] Binary built: ${STAGE_DIR}/system/bin/netclient"

if [ "$WITH_WG" = true ]; then
    echo "==> Fetching static aarch64 wg binary..."
    WG_URL="https://raw.githubusercontent.com/angristan/wireguard-install/master/wireguard-tools/src/wg"
    WG_STATIC_URL="https://github.com/Entware/entware-packages/raw/master/net/wireguard-tools/files/wg"
    
    TMP_WG="${DIST_DIR}/wg-aarch64"
    if curl -fsSL -o "${TMP_WG}" "${WG_STATIC_URL}" 2>/dev/null; then
        cp "${TMP_WG}" "${STAGE_DIR}/system/bin/wg"
        chmod 0755 "${STAGE_DIR}/system/bin/wg"
        echo "    [✓] wg binary included"
    else
        echo "    [!] Warning: Could not download prebuilt static wg binary; skipping."
    fi
fi

echo "==> Packaging Magisk / KernelSU module structure..."
cp "${ROOT_DIR}/magisk/module.prop" "${STAGE_DIR}/"
cp "${ROOT_DIR}/magisk/customize.sh" "${STAGE_DIR}/"
cp "${ROOT_DIR}/magisk/service.sh" "${STAGE_DIR}/"
cp "${ROOT_DIR}/magisk/action.sh" "${STAGE_DIR}/"
cp -r "${ROOT_DIR}/magisk/webroot" "${STAGE_DIR}/"
mkdir -p "${STAGE_DIR}/META-INF/com/google/android"
cp "${ROOT_DIR}/magisk/META-INF/com/google/android/update-binary" "${STAGE_DIR}/META-INF/com/google/android/"
cp "${ROOT_DIR}/magisk/META-INF/com/google/android/updater-script" "${STAGE_DIR}/META-INF/com/google/android/"

chmod 0755 "${STAGE_DIR}/customize.sh" "${STAGE_DIR}/service.sh" "${STAGE_DIR}/action.sh"
chmod 0755 "${STAGE_DIR}/META-INF/com/google/android/update-binary"

echo "==> Creating flashable ZIP: ${ZIP_OUTPUT}..."
cd "${STAGE_DIR}"
zip -r9 "${ZIP_OUTPUT}" ./*

echo "==> Calculating SHA-256 checksum..."
cd "${DIST_DIR}"
sha256sum "$(basename "${ZIP_OUTPUT}")" > "${ZIP_OUTPUT}.sha256"

echo "==============================================="
echo " Build successful!"
echo " Flashable ZIP: ${ZIP_OUTPUT}"
echo " Checksum:      $(cat "${ZIP_OUTPUT}.sha256")"
echo "==============================================="
