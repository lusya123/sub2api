$ErrorActionPreference = 'Stop'

function Require-Env([string]$Name) {
  $value = [Environment]::GetEnvironmentVariable($Name, 'Process')
  if ([string]::IsNullOrWhiteSpace($value)) {
    throw "Missing required environment variable: $Name"
  }
  return $value
}

function Test-NodeVersion {
  try {
    $version = (& node --version 2>$null)
    if (-not $version) { return $false }
    $current = [Version]($version.TrimStart('v'))
    return $current -ge [Version]'18.0.0'
  } catch {
    return $false
  }
}

function Ensure-Node {
  if (Test-NodeVersion) { return }
  if (Get-Command winget -ErrorAction SilentlyContinue) {
    winget install OpenJS.NodeJS.LTS --accept-source-agreements --accept-package-agreements
    $env:Path = [System.Environment]::GetEnvironmentVariable('Path', 'Machine') + ';' + [System.Environment]::GetEnvironmentVariable('Path', 'User')
  }
  if (-not (Test-NodeVersion)) {
    throw 'Node.js 18+ is required. Please install Node.js and rerun this command.'
  }
}

function Install-ClaudeCode {
  & npm config set registry https://registry.npmmirror.com | Out-Null
  & npm install -g @anthropic-ai/claude-code --registry=https://registry.npmmirror.com
}

$token = Require-Env 'CLAUDE_CLIENT_TOKEN'
$apiUrl = Require-Env 'CLAUDE_API_URL'

Ensure-Node
Install-ClaudeCode

[Environment]::SetEnvironmentVariable('ANTHROPIC_BASE_URL', $apiUrl.TrimEnd('/'), 'User')
[Environment]::SetEnvironmentVariable('ANTHROPIC_AUTH_TOKEN', $token, 'User')
[Environment]::SetEnvironmentVariable('CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC', '1', 'User')
[Environment]::SetEnvironmentVariable('CLAUDE_CODE_ATTRIBUTION_HEADER', '0', 'User')

$env:ANTHROPIC_BASE_URL = $apiUrl.TrimEnd('/')
$env:ANTHROPIC_AUTH_TOKEN = $token
$env:CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC = '1'
$env:CLAUDE_CODE_ATTRIBUTION_HEADER = '0'

Write-Host ''
Write-Host 'Claude Code installed.' -ForegroundColor Green
Write-Host 'Open a new PowerShell window, then run: claude' -ForegroundColor Yellow
