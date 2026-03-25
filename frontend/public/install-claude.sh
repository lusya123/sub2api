#!/usr/bin/env bash

set -euo pipefail

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

ensure_node() {
  if command -v node >/dev/null 2>&1; then
    local current
    current="$(node --version | sed 's/^v//')"
    if version_ge "$current" "18.0.0"; then
      return 0
    fi
  fi

  echo "Installing Node.js 18+..." >&2

  if command -v brew >/dev/null 2>&1; then
    brew install node
  elif command -v apt-get >/dev/null 2>&1; then
    sudo apt-get update
    sudo apt-get install -y nodejs npm
  elif command -v dnf >/dev/null 2>&1; then
    sudo dnf install -y nodejs npm
  elif command -v yum >/dev/null 2>&1; then
    sudo yum install -y nodejs npm
  else
    echo "Unable to install Node.js automatically. Please install Node.js 18+ first." >&2
    exit 1
  fi

  local current
  current="$(node --version | sed 's/^v//')"
  if ! version_ge "$current" "18.0.0"; then
    echo "Node.js version is still below 18 after installation: $current" >&2
    exit 1
  fi
}

detect_profile() {
  if [[ "${SHELL:-}" == *zsh* ]]; then
    printf '%s' "$HOME/.zshrc"
    return
  fi
  printf '%s' "$HOME/.bashrc"
}

persist_env() {
  local api_url="$1"
  local token="$2"
  local profile
  profile="$(detect_profile)"
  local env_file="$HOME/.claude/xdt.env.sh"

  mkdir -p "$HOME/.claude"
  cat >"$env_file" <<EOF
export ANTHROPIC_BASE_URL="$api_url"
export ANTHROPIC_AUTH_TOKEN="$token"
export CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1
export CLAUDE_CODE_ATTRIBUTION_HEADER=0
EOF

  touch "$profile"
  if ! grep -Fq "$env_file" "$profile"; then
    {
      echo ''
      echo '# XueDingToken Claude Code'
      echo "[ -f \"$env_file\" ] && source \"$env_file\""
    } >>"$profile"
  fi
}

install_claude_code() {
  npm config set registry https://registry.npmmirror.com >/dev/null 2>&1 || true
  if npm install -g @anthropic-ai/claude-code --registry=https://registry.npmmirror.com; then
    return 0
  fi
  npm install -g @anthropic-ai/claude-code
}

main() {
  require_env "CLAUDE_TOKEN"
  require_env "CLAUDE_API_URL"

  ensure_node
  install_claude_code
  persist_env "${CLAUDE_API_URL%/}" "$CLAUDE_TOKEN"

  echo
  echo "Claude Code installed."
  echo "Reopen your terminal, then run: claude"
}

main "$@"
