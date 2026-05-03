#!/usr/bin/env bash

set -euo pipefail

DEFAULT_REPO="qidoulij006/qt-proxy-installer"
DEFAULT_REF="main"
DEFAULT_ASSET_PATH="dist/nofx-proxy-installer-20260329-152919.tar.gz"

GITHUB_REPO="${GITHUB_REPO:-$DEFAULT_REPO}"
GITHUB_REF="${GITHUB_REF:-$DEFAULT_REF}"
ASSET_PATH="${ASSET_PATH:-$DEFAULT_ASSET_PATH}"
WORK_DIR="${WORK_DIR:-/tmp/nofx-proxy-installer}"

usage() {
    cat <<'EOF'
Usage:
  bash install-proxy-from-github.sh [options] [-- installer-args...]

Options:
  --repo OWNER/REPO       GitHub repository, default qidoulij006/qt-proxy-installer
  --ref REF               Git ref, default main
  --asset-path PATH       Tarball path in repo, default dist/nofx-proxy-installer-20260329-152919.tar.gz
  --work-dir DIR          Temp work directory, default /tmp/nofx-proxy-installer
  --help                  Show this help

Examples:
  curl -fsSL https://raw.githubusercontent.com/qidoulij006/qt-proxy-installer/main/install-proxy-from-github.sh | \
    sudo bash -s -- --username nofx --password 'strong-pass'

  curl -fsSL https://raw.githubusercontent.com/qidoulij006/qt-proxy-installer/main/install-proxy-from-github.sh | \
    sudo bash -s -- --repo qidoulij006/qt-proxy-installer --ref main --username nofx --password 'strong-pass'
EOF
}

require_download_tool() {
    if command -v curl >/dev/null 2>&1; then
        DOWNLOAD_TOOL="curl"
        return
    fi
    if command -v wget >/dev/null 2>&1; then
        DOWNLOAD_TOOL="wget"
        return
    fi
    echo "Neither curl nor wget is available." >&2
    exit 1
}

download_file() {
    local url="$1"
    local output="$2"

    if [[ "$DOWNLOAD_TOOL" == "curl" ]]; then
        curl -fsSL "$url" -o "$output"
    else
        wget -O "$output" "$url"
    fi
}

parse_args() {
    INSTALLER_ARGS=()

    while [[ $# -gt 0 ]]; do
        case "$1" in
            --repo)
                GITHUB_REPO="${2:-}"
                shift 2
                ;;
            --ref)
                GITHUB_REF="${2:-}"
                shift 2
                ;;
            --asset-path)
                ASSET_PATH="${2:-}"
                shift 2
                ;;
            --work-dir)
                WORK_DIR="${2:-}"
                shift 2
                ;;
            --help|-h)
                usage
                exit 0
                ;;
            --)
                shift
                INSTALLER_ARGS+=("$@")
                break
                ;;
            *)
                INSTALLER_ARGS+=("$1")
                shift
                ;;
        esac
    done
}

main() {
    parse_args "$@"
    require_download_tool

    local tarball_url="https://raw.githubusercontent.com/${GITHUB_REPO}/${GITHUB_REF}/${ASSET_PATH}"
    local tarball_file="${WORK_DIR}/installer.tar.gz"

    rm -rf "$WORK_DIR"
    mkdir -p "$WORK_DIR"

    echo "Downloading ${tarball_url}"
    download_file "$tarball_url" "$tarball_file"

    tar -xzf "$tarball_file" -C "$WORK_DIR"

    if [[ ! -f "${WORK_DIR}/install_socks5_proxy.sh" ]]; then
        echo "install_socks5_proxy.sh not found in archive." >&2
        exit 1
    fi

    chmod +x "${WORK_DIR}/install_socks5_proxy.sh"
    exec bash "${WORK_DIR}/install_socks5_proxy.sh" "${INSTALLER_ARGS[@]}"
}

main "$@"
