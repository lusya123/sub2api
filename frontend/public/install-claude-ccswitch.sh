#!/usr/bin/env bash

set -euo pipefail

log() {
  printf '[XueDingToken] %s\n' "$*" >&2
}

fail() {
  printf '[XueDingToken] ERROR: %s\n' "$*" >&2
  exit 1
}

version_ge() {
  local current="$1"
  local required="$2"
  [[ "$(printf '%s\n%s\n' "$required" "$current" | sort -V | head -n1)" == "$required" ]]
}

require_token() {
  if [[ -z "${XDT_TOKEN:-}" ]]; then
    fail "Missing XDT_TOKEN"
  fi
}

normalize_url() {
  local value="${1:-https://xuedingtoken.com}"
  value="${value%/}"
  printf '%s' "$value"
}

detect_ccswitch() {
  if [[ -n "${XDT_CCSWITCH_BIN:-}" && -x "${XDT_CCSWITCH_BIN:-}" ]]; then
    printf '%s\n' "$XDT_CCSWITCH_BIN"
    return 0
  fi

  if command -v cc-switch >/dev/null 2>&1; then
    command -v cc-switch
    return 0
  fi

  for candidate in \
    "/Applications/CC Switch.app/Contents/MacOS/cc-switch" \
    "$HOME/Applications/CC Switch.app/Contents/MacOS/cc-switch"
  do
    if [[ -x "$candidate" ]]; then
      printf '%s\n' "$candidate"
      return 0
    fi
  done

  return 1
}

supports_xdt_import() {
  local bin="$1"
  "$bin" xdt-import --help >/dev/null 2>&1
}

ensure_node_and_claude() {
  if command -v node >/dev/null 2>&1 && command -v npm >/dev/null 2>&1; then
    local current
    current="$(node --version | sed 's/^v//')"
    if version_ge "$current" "18.0.0"; then
      log "Node.js $current detected"
    else
      install_node
    fi
  else
    install_node
  fi

  if command -v claude >/dev/null 2>&1; then
    log "Claude Code detected"
    return 0
  fi

  log "Installing Claude Code with npm"
  npm install -g @anthropic-ai/claude-code
}

install_node() {
  if command -v brew >/dev/null 2>&1; then
    log "Installing Node.js with Homebrew"
    brew install node
    return 0
  fi

  fail "Node.js 18+ is required. Install Node.js or Homebrew, then rerun this command."
}

install_ccswitch_macos() {
  [[ "$(uname -s)" == "Darwin" ]] || fail "This installer currently supports macOS only. Use the Windows PowerShell installer on Windows."

  local base_url="${XDT_INSTALLER_BASE:-https://xuedingtoken.com}"
  base_url="${base_url%/}"
  local arch
  arch="$(uname -m)"
  local default_url
  case "$arch" in
    arm64|aarch64) default_url="$base_url/downloads/cc-switch/CC-Switch-XDT-macOS-arm64.zip" ;;
    x86_64|amd64) default_url="$base_url/downloads/cc-switch/CC-Switch-XDT-macOS-x64.zip" ;;
    *) fail "Unsupported macOS architecture: $arch" ;;
  esac

  local url="${XDT_CCSWITCH_MAC_URL:-$default_url}"
  local tmpdir
  tmpdir="$(mktemp -d)"
  local mount_dir=""
  cleanup_ccswitch_download() {
    if [[ -n "$mount_dir" ]]; then
      hdiutil detach "$mount_dir" -quiet >/dev/null 2>&1 || true
    fi
    rm -rf "$tmpdir"
  }
  trap cleanup_ccswitch_download RETURN

  local archive="$tmpdir/cc-switch-package"
  log "Downloading CC Switch enhanced build"
  curl -fL "$url" -o "$archive"

  local app_path=""
  if file "$archive" | grep -qi 'zlib compressed data\|Zip archive data'; then
    unzip -q "$archive" -d "$tmpdir/unpacked"
    app_path="$(find "$tmpdir/unpacked" -maxdepth 3 -name 'CC Switch.app' -type d | head -n1)"
  else
    mount_dir="$tmpdir/dmg"
    mkdir -p "$mount_dir"
    hdiutil attach "$archive" -mountpoint "$mount_dir" -nobrowse -quiet
    app_path="$(find "$mount_dir" -maxdepth 2 -name 'CC Switch.app' -type d | head -n1)"
  fi

  [[ -n "$app_path" ]] || fail "CC Switch.app not found in downloaded package"

  local target_dir="/Applications"
  if [[ ! -w "$target_dir" ]]; then
    target_dir="$HOME/Applications"
    mkdir -p "$target_dir"
  fi

  log "Installing CC Switch to $target_dir"
  rm -rf "$target_dir/CC Switch.app"
  ditto "$app_path" "$target_dir/CC Switch.app"
  xattr -dr com.apple.quarantine "$target_dir/CC Switch.app" >/dev/null 2>&1 || true
}

ensure_ccswitch() {
  local bin=""
  if bin="$(detect_ccswitch)"; then
    if supports_xdt_import "$bin"; then
      printf '%s\n' "$bin"
      return 0
    fi
    log "Existing CC Switch does not support xdt-import; upgrading"
  else
    log "CC Switch not found; installing"
  fi

  install_ccswitch_macos

  bin="$(detect_ccswitch)" || fail "CC Switch installation completed but binary was not found"
  supports_xdt_import "$bin" || fail "Installed CC Switch does not support xdt-import"
  printf '%s\n' "$bin"
}

main() {
  require_token
  local api_url
  api_url="$(normalize_url "${XDT_API_URL:-https://xuedingtoken.com}")"

  ensure_node_and_claude

  local ccswitch_bin
  ccswitch_bin="$(ensure_ccswitch)"

  log "Importing and switching XueDingToken provider"
  "$ccswitch_bin" xdt-import \
    --provider-id xuedingtoken \
    --name XueDingToken \
    --app claude \
    --endpoint "$api_url" \
    --api-key "$XDT_TOKEN" \
    --homepage "https://xuedingtoken.com" \
    --icon claude \
    --switch

  log "Claude Code is configured through CC Switch"
  if [[ "${XDT_SKIP_LAUNCH_CLAUDE:-0}" != "1" ]]; then
    log "Starting Claude Code"
    exec claude
  fi
}

main "$@"
