#!/usr/bin/env bash
set -euo pipefail

# Vela CLI bootstrap installer
# Usage: curl -fsSL https://raw.githubusercontent.com/khala-matrix/vela/main/bootstrap.sh | bash
#   or:  bash bootstrap.sh

REPO="khala-matrix/vela"
INSTALL_DIR="${VELA_INSTALL_DIR:-$HOME/.vela/bin}"
SKILL_DIR="${HOME}/.claude/skills"

# Colors (disabled if not a terminal)
if [ -t 1 ]; then
  BOLD="\033[1m"
  GREEN="\033[32m"
  YELLOW="\033[33m"
  RED="\033[31m"
  RESET="\033[0m"
else
  BOLD="" GREEN="" YELLOW="" RED="" RESET=""
fi

info()  { echo -e "${GREEN}==>${RESET} ${BOLD}$*${RESET}"; }
warn()  { echo -e "${YELLOW}warning:${RESET} $*"; }
error() { echo -e "${RED}error:${RESET} $*" >&2; exit 1; }

detect_platform() {
  OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
  ARCH="$(uname -m)"

  case "$OS" in
    linux)  OS="linux" ;;
    darwin) OS="darwin" ;;
    *)      error "Unsupported OS: $OS" ;;
  esac

  case "$ARCH" in
    x86_64|amd64)  ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *)             error "Unsupported architecture: $ARCH" ;;
  esac
}

get_latest_version() {
  local url="https://api.github.com/repos/${REPO}/releases/latest"
  local auth_header=""
  if [ -n "${GITHUB_TOKEN:-}" ]; then
    auth_header="Authorization: token ${GITHUB_TOKEN}"
  fi

  if command -v curl &>/dev/null; then
    VERSION=$(curl -fsSL ${auth_header:+-H "$auth_header"} "$url" | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"//;s/".*//')
  elif command -v wget &>/dev/null; then
    VERSION=$(wget -qO- ${auth_header:+--header="$auth_header"} "$url" | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"//;s/".*//')
  else
    error "Neither curl nor wget found. Install one and retry."
  fi

  if [ -z "${VERSION:-}" ]; then
    error "Failed to fetch latest version from GitHub. If rate-limited, set GITHUB_TOKEN."
  fi
}

download_binary() {
  local asset="vela_${OS}_${ARCH}"
  local checksums="checksums.txt"
  local base_url="https://github.com/${REPO}/releases/download/${VERSION}"
  local tmp_dir
  tmp_dir="$(mktemp -d)"

  info "Downloading vela ${VERSION} for ${OS}/${ARCH}..."

  if command -v curl &>/dev/null; then
    curl -fsSL "${base_url}/${asset}" -o "${tmp_dir}/vela"
    curl -fsSL "${base_url}/${checksums}" -o "${tmp_dir}/${checksums}" 2>/dev/null || true
  else
    wget -q "${base_url}/${asset}" -O "${tmp_dir}/vela"
    wget -q "${base_url}/${checksums}" -O "${tmp_dir}/${checksums}" 2>/dev/null || true
  fi

  if [ -f "${tmp_dir}/${checksums}" ]; then
    info "Verifying checksum..."
    local expected
    expected=$(grep "${asset}" "${tmp_dir}/${checksums}" | awk '{print $1}')
    if [ -n "$expected" ]; then
      local actual
      if command -v sha256sum &>/dev/null; then
        actual=$(sha256sum "${tmp_dir}/vela" | awk '{print $1}')
      elif command -v shasum &>/dev/null; then
        actual=$(shasum -a 256 "${tmp_dir}/vela" | awk '{print $1}')
      fi
      if [ -n "${actual:-}" ] && [ "$actual" != "$expected" ]; then
        rm -rf "$tmp_dir"
        error "Checksum mismatch: expected ${expected}, got ${actual}"
      fi
    fi
  fi

  mkdir -p "$INSTALL_DIR"
  mv "${tmp_dir}/vela" "${INSTALL_DIR}/vela"
  chmod +x "${INSTALL_DIR}/vela"
  rm -rf "$tmp_dir"
}

install_skill() {
  local skill_url="https://raw.githubusercontent.com/${REPO}/${VERSION}/skills/vela-infra.md"
  local skill_dir="${SKILL_DIR}/vela-infra"

  info "Installing Claude Code skill..."
  mkdir -p "$skill_dir"

  if command -v curl &>/dev/null; then
    curl -fsSL "$skill_url" -o "${skill_dir}/SKILL.md"
  else
    wget -q "$skill_url" -O "${skill_dir}/SKILL.md"
  fi
}

setup_path() {
  if echo "$PATH" | tr ':' '\n' | grep -qx "$INSTALL_DIR"; then
    return
  fi

  local shell_name
  shell_name="$(basename "${SHELL:-/bin/bash}")"
  local profile=""

  case "$shell_name" in
    zsh)  profile="$HOME/.zshrc" ;;
    bash)
      if [ -f "$HOME/.bash_profile" ]; then
        profile="$HOME/.bash_profile"
      else
        profile="$HOME/.bashrc"
      fi
      ;;
    fish) profile="$HOME/.config/fish/config.fish" ;;
    *)    profile="$HOME/.profile" ;;
  esac

  local path_line
  if [ "$shell_name" = "fish" ]; then
    path_line="fish_add_path ${INSTALL_DIR}"
  else
    path_line="export PATH=\"${INSTALL_DIR}:\$PATH\""
  fi

  if [ -n "$profile" ] && [ -f "$profile" ]; then
    if ! grep -q "$INSTALL_DIR" "$profile" 2>/dev/null; then
      echo "" >> "$profile"
      echo "# Vela CLI" >> "$profile"
      echo "$path_line" >> "$profile"
      info "Added ${INSTALL_DIR} to PATH in ${profile}"
    fi
  else
    warn "Could not detect shell profile. Add this to your shell config manually:"
    echo "  $path_line"
  fi
}

main() {
  info "Vela CLI bootstrap installer"
  echo ""

  detect_platform
  get_latest_version
  download_binary
  install_skill
  setup_path

  echo ""
  info "Vela ${VERSION} installed to ${INSTALL_DIR}/vela"
  echo ""
  echo "  Installed:"
  echo "    Binary:  ${INSTALL_DIR}/vela"
  echo "    Skill:   ${SKILL_DIR}/vela-infra/SKILL.md"
  echo ""
  echo "  Quick start:"
  echo "    vela create my-app -t nextjs-fastapi-pg"
  echo "    cd my-app && ./build.sh && vela deploy"
  echo ""

  if ! echo "$PATH" | tr ':' '\n' | grep -qx "$INSTALL_DIR"; then
    warn "Restart your shell or run: source ${profile:-~/.profile}"
  fi
}

main "$@"
