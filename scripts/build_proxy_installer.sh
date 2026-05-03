#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SOURCE_DIR="${ROOT_DIR}/scripts/proxy-installer"
DIST_DIR="${ROOT_DIR}/dist"
VERSION="$(date +%Y%m%d-%H%M%S)"
PACKAGE_NAME="nofx-proxy-installer-${VERSION}.tar.gz"
PACKAGE_PATH="${DIST_DIR}/${PACKAGE_NAME}"

mkdir -p "${DIST_DIR}"
tar -C "${SOURCE_DIR}" -czf "${PACKAGE_PATH}" .

echo "Created package:"
echo "  ${PACKAGE_PATH}"
