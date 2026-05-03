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
  awk -v current="$current" -v required="$required" '
    BEGIN {
      split(current, c, ".")
      split(required, r, ".")
      for (i = 1; i <= 3; i++) {
        cv = (c[i] == "" ? 0 : c[i]) + 0
        rv = (r[i] == "" ? 0 : r[i]) + 0
        if (cv > rv) exit 0
        if (cv < rv) exit 1
      }
      exit 0
    }'
}

require_token() {
  XDT_TOKEN="${XDT_TOKEN:-${CLAUDE_TOKEN:-${CLAUDE_CLIENT_TOKEN:-}}}"
  if [[ -z "${XDT_TOKEN:-}" ]]; then
    fail "Missing XDT_TOKEN"
  fi
  export XDT_TOKEN
}

normalize_url() {
  local value="${1:-${CLAUDE_API_URL:-https://xuedingtoken.com}}"
  value="${value%/}"
  printf '%s' "$value"
}

download_file() {
  local url="$1"
  local output="$2"
  if command -v curl >/dev/null 2>&1; then
    curl -fL --connect-timeout 15 --retry 2 "$url" -o "$output"
    return $?
  fi
  if command -v wget >/dev/null 2>&1; then
    wget -q --timeout=15 --tries=2 -O "$output" "$url"
    return $?
  fi
  fail "curl or wget is required"
}

download_first() {
  local output="$1"
  shift
  local url
  for url in "$@"; do
    [[ -n "$url" ]] || continue
    log "Downloading: $url"
    if download_file "$url" "$output"; then
      return 0
    fi
  done
  return 1
}

detect_profile() {
  if [[ -n "${PROFILE:-}" ]]; then
    printf '%s' "$PROFILE"
  elif [[ "${SHELL:-}" == *"zsh"* ]]; then
    printf '%s' "$HOME/.zshrc"
  else
    printf '%s' "$HOME/.bashrc"
  fi
}

append_profile_once() {
  local line="$1"
  local profile
  profile="$(detect_profile)"
  touch "$profile"
  if ! grep -Fq "$line" "$profile" 2>/dev/null; then
    printf '\n%s\n' "$line" >> "$profile"
  fi
}

configure_node_mirrors() {
  export NVM_NODEJS_ORG_MIRROR="${XDT_NODE_MIRROR:-https://npmmirror.com/mirrors/node}"
  export NVM_NPM_MIRROR="${XDT_NPM_MIRROR:-https://npmmirror.com/mirrors/npm}"
  export npm_config_registry="${XDT_NPM_REGISTRY:-https://registry.npmmirror.com}"

  append_profile_once "export NVM_NODEJS_ORG_MIRROR=\"$NVM_NODEJS_ORG_MIRROR\""
  append_profile_once "export NVM_NPM_MIRROR=\"$NVM_NPM_MIRROR\""
  append_profile_once "export npm_config_registry=\"$npm_config_registry\""

  if command -v npm >/dev/null 2>&1; then
    npm config set registry "$npm_config_registry" >/dev/null 2>&1 || true
  fi
}

load_nvm() {
  export NVM_DIR="${NVM_DIR:-$HOME/.nvm}"
  if [[ -s "$NVM_DIR/nvm.sh" ]]; then
    set +u
    . "$NVM_DIR/nvm.sh"
    set -u
  fi
  return 0
}

run_nvm() {
  set +u
  nvm "$@"
  local rc=$?
  set -u
  return "$rc"
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
    "$HOME/Applications/CC Switch.app/Contents/MacOS/cc-switch" \
    "/usr/bin/cc-switch" \
    "/usr/local/bin/cc-switch" \
    "$HOME/.local/bin/cc-switch"
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
  configure_node_mirrors
  load_nvm

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

  configure_node_mirrors

  if command -v claude >/dev/null 2>&1; then
    log "Claude Code detected"
    return 0
  fi

  log "Installing Claude Code with npm"
  if npm install -g --registry "$npm_config_registry" @anthropic-ai/claude-code; then
    return 0
  fi
  log "npm mirror failed; retrying with npmjs.org"
  npm install -g --registry https://registry.npmjs.org @anthropic-ai/claude-code
}

install_node() {
  configure_node_mirrors
  load_nvm

  if ! command -v nvm >/dev/null 2>&1; then
    install_nvm
    load_nvm
  fi

  if command -v nvm >/dev/null 2>&1; then
    local node_version="${XDT_NODE_VERSION:-lts/*}"
    log "Installing Node.js with nvm from $NVM_NODEJS_ORG_MIRROR"
    if [[ "$node_version" == "lts/*" || "$node_version" == "--lts" ]]; then
      run_nvm install --lts
      run_nvm use --lts
      run_nvm alias default 'lts/*' >/dev/null 2>&1 || true
    else
      run_nvm install "$node_version"
      run_nvm use "$node_version"
      run_nvm alias default "$node_version" >/dev/null 2>&1 || true
    fi
    hash -r
    return 0
  fi

  if command -v brew >/dev/null 2>&1; then
    log "Installing Node.js with Homebrew"
    brew install node
    return 0
  fi

  fail "Node.js 18+ is required and automatic installation failed"
}

install_nvm() {
  export NVM_DIR="${NVM_DIR:-$HOME/.nvm}"
  local base_url="${XDT_INSTALLER_BASE:-https://xuedingtoken.com}"
  base_url="${base_url%/}"
  local version="${XDT_NVM_VERSION:-v0.40.3}"
  local tmpdir
  tmpdir="$(mktemp -d)"
  local archive="$tmpdir/nvm.tar.gz"

  mkdir -p "$NVM_DIR"
  if ! download_first "$archive" \
    "${XDT_NVM_TARBALL_URL:-}" \
    "$base_url/downloads/node/nvm-$version.tar.gz" \
    "https://github.com/nvm-sh/nvm/archive/$version.tar.gz"; then
    fail "Unable to download nvm"
  fi

  tar -xzf "$archive" -C "$NVM_DIR" --strip-components=1

  append_profile_once 'export NVM_DIR="$HOME/.nvm"'
  append_profile_once '[ -s "$NVM_DIR/nvm.sh" ] && . "$NVM_DIR/nvm.sh"'
  append_profile_once '[ -s "$NVM_DIR/bash_completion" ] && . "$NVM_DIR/bash_completion"'
  configure_node_mirrors
  rm -rf "$tmpdir"
}

install_ccswitch_macos() {
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

  local archive="$tmpdir/cc-switch-package"
  log "Downloading CC Switch enhanced build"
  download_file "$url" "$archive"

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
  if [[ -n "$mount_dir" ]]; then
    hdiutil detach "$mount_dir" -quiet >/dev/null 2>&1 || true
  fi
  rm -rf "$tmpdir"
}

run_root() {
  if [[ "$(id -u)" -eq 0 ]]; then
    "$@"
    return $?
  fi
  if command -v sudo >/dev/null 2>&1; then
    sudo "$@"
    return $?
  fi
  fail "sudo is required to install the Linux CC Switch package"
}

configure_apt_mirror_fallback() {
  local os_id="" codename=""
  if [[ -r /etc/os-release ]]; then
    set +u
    . /etc/os-release
    set -u
    os_id="${ID:-}"
    codename="${VERSION_CODENAME:-${UBUNTU_CODENAME:-}}"
  fi

  [[ -n "$codename" ]] || return 1

  local tmpfile
  tmpfile="$(mktemp)"
  case "$os_id" in
    ubuntu)
      local mirror="${XDT_APT_MIRROR:-http://mirrors.aliyun.com/ubuntu/}"
      cat > "$tmpfile" <<EOF
deb $mirror $codename main restricted universe multiverse
deb $mirror $codename-updates main restricted universe multiverse
deb $mirror $codename-backports main restricted universe multiverse
deb $mirror $codename-security main restricted universe multiverse
EOF
      ;;
    debian)
      local mirror="${XDT_APT_MIRROR:-http://mirrors.aliyun.com/debian/}"
      local security_mirror="${XDT_APT_SECURITY_MIRROR:-http://mirrors.aliyun.com/debian-security/}"
      cat > "$tmpfile" <<EOF
deb $mirror $codename main contrib non-free non-free-firmware
deb $mirror $codename-updates main contrib non-free non-free-firmware
deb $security_mirror $codename-security main contrib non-free non-free-firmware
EOF
      ;;
    *)
      rm -f "$tmpfile"
      return 1
      ;;
  esac

  log "Adding apt mirror fallback for $os_id $codename"
  run_root mkdir -p /etc/apt/sources.list.d
  run_root cp "$tmpfile" /etc/apt/sources.list.d/xdt-mirror-fallback.list
  rm -f "$tmpfile"
}

install_deb_with_apt() {
  local package="$1"
  if run_root env DEBIAN_FRONTEND=noninteractive apt-get update &&
    run_root env DEBIAN_FRONTEND=noninteractive apt-get install -y "$package"; then
    return 0
  fi

  log "apt local package install failed; trying China mirror fallback"
  configure_apt_mirror_fallback || return 1
  run_root env DEBIAN_FRONTEND=noninteractive apt-get update
  run_root env DEBIAN_FRONTEND=noninteractive apt-get install -y "$package"
}

install_ccswitch_linux() {
  local base_url="${XDT_INSTALLER_BASE:-https://xuedingtoken.com}"
  base_url="${base_url%/}"
  local arch
  arch="$(uname -m)"
  local default_url
  case "$arch" in
    x86_64|amd64) default_url="$base_url/downloads/cc-switch/CC-Switch-XDT-Linux-x64.deb" ;;
    aarch64|arm64) default_url="$base_url/downloads/cc-switch/CC-Switch-XDT-Linux-arm64.deb" ;;
    *) fail "Unsupported Linux architecture: $arch" ;;
  esac

  local url="${XDT_CCSWITCH_LINUX_URL:-$default_url}"
  local tmpdir
  tmpdir="$(mktemp -d)"
  local package="$tmpdir/cc-switch-package"
  case "$url" in
    *.deb) package="$package.deb" ;;
    *.rpm) package="$package.rpm" ;;
    *.AppImage) package="$package.AppImage" ;;
  esac

  log "Downloading CC Switch enhanced build"
  download_file "$url" "$package"

  case "$package" in
    *.deb)
      if command -v apt-get >/dev/null 2>&1; then
        install_deb_with_apt "$package"
      elif command -v dpkg >/dev/null 2>&1; then
        run_root dpkg -i "$package"
      else
        fail "apt-get or dpkg is required to install the Linux .deb package"
      fi
      ;;
    *.rpm)
      if command -v rpm >/dev/null 2>&1; then
        run_root rpm -Uvh --replacepkgs "$package"
      else
        fail "rpm is required to install the Linux .rpm package"
      fi
      ;;
    *.AppImage)
      mkdir -p "$HOME/.local/bin"
      cp "$package" "$HOME/.local/bin/cc-switch"
      chmod +x "$HOME/.local/bin/cc-switch"
      export PATH="$HOME/.local/bin:$PATH"
      append_profile_once 'export PATH="$HOME/.local/bin:$PATH"'
      ;;
    *)
      fail "Unsupported Linux CC Switch package URL: $url"
      ;;
  esac

  rm -rf "$tmpdir"
}

install_ccswitch() {
  case "$(uname -s)" in
    Darwin) install_ccswitch_macos ;;
    Linux) install_ccswitch_linux ;;
    *) fail "This installer supports macOS and Linux. Use the Windows PowerShell installer on Windows." ;;
  esac
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

  install_ccswitch >&2

  bin="$(detect_ccswitch)" || fail "CC Switch installation completed but binary was not found"
  supports_xdt_import "$bin" || fail "Installed CC Switch does not support xdt-import"
  printf '%s\n' "$bin"
}

main() {
  require_token
  local api_url
  api_url="$(normalize_url "${XDT_API_URL:-${CLAUDE_API_URL:-https://xuedingtoken.com}}")"

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
