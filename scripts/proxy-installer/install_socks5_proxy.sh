#!/usr/bin/env bash

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

MICROSOCKS_URL_DEFAULT="https://codeload.github.com/rofl0r/microsocks/tar.gz/refs/heads/master"
INSTALL_DIR="/opt/microsocks-src"
BINARY_PATH="/usr/local/bin/microsocks"
SERVICE_NAME="microsocks"
ENV_DIR="/etc/microsocks"
ENV_FILE="${ENV_DIR}/microsocks.env"
SERVICE_FILE="/etc/systemd/system/${SERVICE_NAME}.service"

PROXY_PORT="${PROXY_PORT:-1080}"
PROXY_BIND="${PROXY_BIND:-0.0.0.0}"
PROXY_USERNAME="${PROXY_USERNAME:-}"
PROXY_PASSWORD="${PROXY_PASSWORD:-}"
MICROSOCKS_URL="${MICROSOCKS_URL:-$MICROSOCKS_URL_DEFAULT}"
NON_INTERACTIVE="false"

print_header() {
    echo -e "${BLUE}"
    echo "╔════════════════════════════════════════════════════════════╗"
    echo "║                 NOFX SOCKS5 Proxy Installer               ║"
    echo "╚════════════════════════════════════════════════════════════╝"
    echo -e "${NC}"
}

usage() {
    cat <<'EOF'
Usage:
  sudo bash install_socks5_proxy.sh [options]

Options:
  --username USER       SOCKS5 username
  --password PASS       SOCKS5 password
  --port PORT           Listen port, default 1080
  --bind ADDR           Bind address, default 0.0.0.0
  --non-interactive     Fail instead of prompting for missing values
  --help                Show this help

Examples:
  sudo bash install_socks5_proxy.sh
  sudo bash install_socks5_proxy.sh --username nofx --password 'strong-pass' --port 1080
EOF
}

require_root() {
    if [[ "${EUID}" -ne 0 ]]; then
        echo -e "${RED}This installer must run as root.${NC}"
        exit 1
    fi
}

parse_args() {
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --username)
                PROXY_USERNAME="${2:-}"
                shift 2
                ;;
            --password)
                PROXY_PASSWORD="${2:-}"
                shift 2
                ;;
            --port)
                PROXY_PORT="${2:-}"
                shift 2
                ;;
            --bind)
                PROXY_BIND="${2:-}"
                shift 2
                ;;
            --non-interactive)
                NON_INTERACTIVE="true"
                shift
                ;;
            --help|-h)
                usage
                exit 0
                ;;
            *)
                echo -e "${RED}Unknown argument: $1${NC}"
                usage
                exit 1
                ;;
        esac
    done
}

prompt_if_missing() {
    if [[ -z "${PROXY_USERNAME}" ]]; then
        if [[ "${NON_INTERACTIVE}" == "true" ]]; then
            echo -e "${RED}Missing required --username${NC}"
            exit 1
        fi
        read -r -p "Proxy username: " PROXY_USERNAME
    fi

    if [[ -z "${PROXY_PASSWORD}" ]]; then
        if [[ "${NON_INTERACTIVE}" == "true" ]]; then
            echo -e "${RED}Missing required --password${NC}"
            exit 1
        fi
        read -r -s -p "Proxy password: " PROXY_PASSWORD
        echo
    fi

    if [[ "${NON_INTERACTIVE}" != "true" ]]; then
        read -r -p "Proxy port [${PROXY_PORT}]: " input_port || true
        PROXY_PORT="${input_port:-$PROXY_PORT}"
        read -r -p "Bind address [${PROXY_BIND}]: " input_bind || true
        PROXY_BIND="${input_bind:-$PROXY_BIND}"
    fi
}

validate_inputs() {
    if [[ -z "${PROXY_USERNAME}" || -z "${PROXY_PASSWORD}" ]]; then
        echo -e "${RED}Username and password cannot be empty.${NC}"
        exit 1
    fi

    if ! [[ "${PROXY_PORT}" =~ ^[0-9]+$ ]] || (( PROXY_PORT < 1 || PROXY_PORT > 65535 )); then
        echo -e "${RED}Port must be a valid number between 1 and 65535.${NC}"
        exit 1
    fi
}

detect_pkg_manager() {
    if command -v apt-get >/dev/null 2>&1; then
        PKG_MANAGER="apt"
    elif command -v dnf >/dev/null 2>&1; then
        PKG_MANAGER="dnf"
    elif command -v yum >/dev/null 2>&1; then
        PKG_MANAGER="yum"
    elif command -v apk >/dev/null 2>&1; then
        PKG_MANAGER="apk"
    else
        echo -e "${RED}Unsupported OS: no apt/dnf/yum/apk found.${NC}"
        exit 1
    fi
}

install_dependencies() {
    echo -e "${YELLOW}Installing build dependencies...${NC}"
    case "${PKG_MANAGER}" in
        apt)
            apt-get update
            DEBIAN_FRONTEND=noninteractive apt-get install -y curl wget tar make gcc libc6-dev ca-certificates
            ;;
        dnf)
            dnf install -y curl wget tar make gcc glibc-devel ca-certificates
            ;;
        yum)
            yum install -y curl wget tar make gcc glibc-devel ca-certificates
            ;;
        apk)
            apk add --no-cache curl wget tar make gcc musl-dev ca-certificates
            ;;
    esac
}

download_microsocks() {
    echo -e "${YELLOW}Downloading microsocks source...${NC}"
    rm -rf "${INSTALL_DIR}"
    mkdir -p "${INSTALL_DIR}"

    local archive="/tmp/microsocks.tar.gz"
    rm -f "${archive}"

    if command -v curl >/dev/null 2>&1; then
        curl -fsSL "${MICROSOCKS_URL}" -o "${archive}"
    else
        wget -O "${archive}" "${MICROSOCKS_URL}"
    fi

    tar -xzf "${archive}" -C "${INSTALL_DIR}" --strip-components=1
}

build_and_install_microsocks() {
    echo -e "${YELLOW}Building microsocks...${NC}"
    make -C "${INSTALL_DIR}"
    install -m 0755 "${INSTALL_DIR}/microsocks" "${BINARY_PATH}"
}

write_env_file() {
    echo -e "${YELLOW}Writing secure runtime config...${NC}"
    mkdir -p "${ENV_DIR}"
    chmod 700 "${ENV_DIR}"
    cat > "${ENV_FILE}" <<EOF
PROXY_USERNAME=${PROXY_USERNAME}
PROXY_PASSWORD=${PROXY_PASSWORD}
PROXY_PORT=${PROXY_PORT}
PROXY_BIND=${PROXY_BIND}
EOF
    chmod 600 "${ENV_FILE}"
}

write_systemd_service() {
    echo -e "${YELLOW}Installing systemd service...${NC}"
    cat > "${SERVICE_FILE}" <<'EOF'
[Unit]
Description=MicroSocks SOCKS5 Proxy
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
EnvironmentFile=/etc/microsocks/microsocks.env
ExecStart=/usr/local/bin/microsocks -i ${PROXY_BIND} -p ${PROXY_PORT} -u ${PROXY_USERNAME} -P ${PROXY_PASSWORD}
Restart=always
RestartSec=3
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/etc/microsocks

[Install]
WantedBy=multi-user.target
EOF

    systemctl daemon-reload
    systemctl enable --now "${SERVICE_NAME}"
}

open_firewall_if_possible() {
    echo -e "${YELLOW}Checking firewall rules...${NC}"

    if command -v ufw >/dev/null 2>&1; then
        ufw allow "${PROXY_PORT}/tcp" >/dev/null 2>&1 || true
        echo -e "${GREEN}✓ UFW rule ensured for ${PROXY_PORT}/tcp${NC}"
        return
    fi

    if command -v firewall-cmd >/dev/null 2>&1 && systemctl is-active --quiet firewalld; then
        firewall-cmd --permanent --add-port="${PROXY_PORT}/tcp" >/dev/null 2>&1 || true
        firewall-cmd --reload >/dev/null 2>&1 || true
        echo -e "${GREEN}✓ firewalld rule ensured for ${PROXY_PORT}/tcp${NC}"
        return
    fi

    echo -e "${YELLOW}No managed firewall tool detected. Ensure ${PROXY_PORT}/tcp is open in your cloud security group.${NC}"
}

detect_public_ip() {
    local ip=""
    if command -v curl >/dev/null 2>&1; then
        ip="$(curl -fsSL --max-time 10 https://api.ipify.org || true)"
    fi
    echo "${ip}"
}

verify_service() {
    echo -e "${YELLOW}Verifying proxy service...${NC}"
    systemctl is-active --quiet "${SERVICE_NAME}"

    if command -v curl >/dev/null 2>&1; then
        local ip
        ip="$(curl -fsSL --max-time 20 --proxy "socks5h://${PROXY_USERNAME}:${PROXY_PASSWORD}@127.0.0.1:${PROXY_PORT}" https://api.ipify.org || true)"
        if [[ -n "${ip}" ]]; then
            echo -e "${GREEN}✓ Local proxy verification succeeded, egress IP: ${ip}${NC}"
        else
            echo -e "${YELLOW}Proxy service is running, but egress IP verification did not return a value.${NC}"
        fi
    fi
}

print_summary() {
    local public_ip
    public_ip="$(detect_public_ip)"

    echo
    echo -e "${GREEN}Installation complete.${NC}"
    echo "Service: ${SERVICE_NAME}"
    echo "Port: ${PROXY_PORT}"
    echo "Bind: ${PROXY_BIND}"
    echo "Status: $(systemctl is-active "${SERVICE_NAME}")"
    if [[ -n "${public_ip}" ]]; then
        echo "Server public IP: ${public_ip}"
        echo "NOFX proxy_url: socks5://${PROXY_USERNAME}:${PROXY_PASSWORD}@${public_ip}:${PROXY_PORT}"
    else
        echo "NOFX proxy_url: socks5://${PROXY_USERNAME}:${PROXY_PASSWORD}@<YOUR_SERVER_PUBLIC_IP>:${PROXY_PORT}"
    fi
    echo
    echo "Useful commands:"
    echo "  systemctl status ${SERVICE_NAME}"
    echo "  journalctl -u ${SERVICE_NAME} -n 100 --no-pager"
    echo "  systemctl restart ${SERVICE_NAME}"
}

main() {
    print_header
    require_root
    parse_args "$@"
    prompt_if_missing
    validate_inputs
    detect_pkg_manager
    install_dependencies
    download_microsocks
    build_and_install_microsocks
    write_env_file
    write_systemd_service
    open_firewall_if_possible
    verify_service
    print_summary
}

main "$@"
