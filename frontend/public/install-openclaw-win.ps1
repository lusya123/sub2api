$ErrorActionPreference = 'Stop'
$RequiredNodeVersion = [Version]'22.16.0'

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
    return $current -ge $RequiredNodeVersion
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
    throw "OpenClaw requires Node.js $RequiredNodeVersion or newer. Please install Node.js and rerun this command."
  }
}

$null = Require-Env 'OPENCLAW_TOKEN'
$null = Require-Env 'OPENCLAW_BASE_URL'
$null = Require-Env 'OPENCLAW_MODEL'
$installerBase = [Environment]::GetEnvironmentVariable('OPENCLAW_INSTALLER_BASE', 'Process')
if ([string]::IsNullOrWhiteSpace($installerBase)) {
  $installerBase = 'https://xuedingtoken.com'
}

Ensure-Node

$target = Join-Path $env:TEMP 'install-openclaw.js'
Invoke-WebRequest -Uri ($installerBase.TrimEnd('/') + '/install-openclaw.js') -OutFile $target
& node $target

Write-Host ''
Write-Host 'OpenClaw install finished.' -ForegroundColor Green
Write-Host 'Run: openclaw tui' -ForegroundColor Yellow
