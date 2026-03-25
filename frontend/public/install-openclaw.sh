#!/usr/bin/env bash

set -euo pipefail

REQUIRED_NODE_VERSION="22.16.0"
OPENCLAW_NODE_VERSION="22.16.0"
helper=""

require_env() {
  local name="$1"
  if [[ -z "${!name:-}" ]]; then
    echo "Missing required environment variable: $name" >&2
    exit 1
  fi
}

version_ge() {
  local current="$1"
  local required="$2"
  [[ "$(printf '%s\n%s\n' "$required" "$current" | sort -V | head -n1)" == "$required" ]]
}

detect_profile() {
  if [[ "${SHELL:-}" == *zsh* ]]; then
    printf '%s' "$HOME/.zshrc"
    return
  fi
  printf '%s' "$HOME/.bashrc"
}

persist_node_path() {
  local node_bin_dir="$1"
  local profile
  profile="$(detect_profile)"

  touch "$profile"
  if ! grep -Fq "$node_bin_dir" "$profile"; then
    {
      echo ''
      echo '# XueDingToken OpenClaw Node.js'
      echo "export PATH=\"$node_bin_dir:\$PATH\""
    } >>"$profile"
  fi
}

install_portable_node() {
  local os arch tmp_dir package node_dir extract_dir url
  tmp_dir="${TMPDIR:-/tmp}"

  case "$(uname -s)" in
    Darwin) os="darwin" ;;
    Linux) os="linux" ;;
    *)
      echo "Unsupported system for automatic Node.js install: $(uname -s)" >&2
      exit 1
      ;;
  esac

  case "$(uname -m)" in
    x86_64|amd64) arch="x64" ;;
    arm64|aarch64) arch="arm64" ;;
    *)
      echo "Unsupported CPU architecture for automatic Node.js install: $(uname -m)" >&2
      exit 1
      ;;
  esac

  node_dir="$HOME/.local/openclaw-node-v${OPENCLAW_NODE_VERSION}"
  if [[ -x "$node_dir/bin/node" ]]; then
    local current
    current="$("$node_dir/bin/node" --version | sed 's/^v//')"
    if version_ge "$current" "$REQUIRED_NODE_VERSION"; then
      export PATH="$node_dir/bin:$PATH"
      persist_node_path "$node_dir/bin"
      return 0
    fi
  fi

  package="node-v${OPENCLAW_NODE_VERSION}-${os}-${arch}"
  url="https://npmmirror.com/mirrors/node/v${OPENCLAW_NODE_VERSION}/${package}.tar.gz"
  extract_dir="$(mktemp -d "${tmp_dir%/}/openclaw-node.XXXXXX")"

  echo "Installing portable Node.js ${OPENCLAW_NODE_VERSION}..." >&2
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$url" | tar -xzf - -C "$extract_dir"
  elif command -v wget >/dev/null 2>&1; then
    wget -qO- "$url" | tar -xzf - -C "$extract_dir"
  else
    echo "curl or wget is required to install Node.js automatically." >&2
    rm -rf "$extract_dir"
    exit 1
  fi

  mkdir -p "$(dirname "$node_dir")"
  rm -rf "$node_dir"
  mv "$extract_dir/$package" "$node_dir"
  rm -rf "$extract_dir"

  export PATH="$node_dir/bin:$PATH"
  persist_node_path "$node_dir/bin"
}

ensure_node() {
  if command -v node >/dev/null 2>&1; then
    local current
    current="$(node --version | sed 's/^v//')"
    if version_ge "$current" "$REQUIRED_NODE_VERSION"; then
      return 0
    fi
  fi

  install_portable_node

  local current
  current="$(node --version | sed 's/^v//')"
  if ! version_ge "$current" "$REQUIRED_NODE_VERSION"; then
    echo "Node.js ${REQUIRED_NODE_VERSION}+ is required for OpenClaw. Current version: $current" >&2
    exit 1
  fi
}

download_helper() {
  local base="${OPENCLAW_INSTALLER_BASE:-https://xuedingtoken.com}"
  local target
  target="$(mktemp "${TMPDIR:-/tmp}/install-openclaw.XXXXXX")"

  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "${base%/}/install-openclaw.js" -o "$target"
  elif command -v wget >/dev/null 2>&1; then
    wget -qO "$target" "${base%/}/install-openclaw.js"
  else
    echo "curl or wget is required to download the OpenClaw installer." >&2
    exit 1
  fi

  printf '%s' "$target"
}

cleanup() {
  if [[ -n "$helper" && -f "$helper" ]]; then
    rm -f "$helper"
  fi
}

main() {
  require_env "OPENCLAW_TOKEN"
  require_env "OPENCLAW_BASE_URL"
  require_env "OPENCLAW_MODEL"

  ensure_node

  helper="$(download_helper)"
  trap cleanup EXIT

  node "$helper"

  echo
  echo "OpenClaw install finished."
  echo "Run: openclaw tui"
}

main "$@"
