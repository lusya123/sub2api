$ErrorActionPreference = 'Stop'

function Write-XdtLog([string]$Message) {
  Write-Host "[XueDingToken] $Message"
}

function Fail-Xdt([string]$Message) {
  throw "[XueDingToken] $Message"
}

function Require-Token {
  if ([string]::IsNullOrWhiteSpace($env:XDT_TOKEN)) {
    Fail-Xdt 'Missing XDT_TOKEN'
  }
}

function Normalize-Url([string]$Value) {
  if ([string]::IsNullOrWhiteSpace($Value)) {
    return 'https://xuedingtoken.com'
  }
  return $Value.Trim().TrimEnd('/')
}

function Test-NodeVersion {
  try {
    $versionText = (& node --version 2>$null)
    if (-not $versionText) { return $false }
    $current = [Version]($versionText.Trim().TrimStart('v'))
    return $current -ge [Version]'18.0.0'
  } catch {
    return $false
  }
}

function Refresh-ProcessPath {
  $machine = [Environment]::GetEnvironmentVariable('Path', 'Machine')
  $user = [Environment]::GetEnvironmentVariable('Path', 'User')
  $env:Path = @($machine, $user, $env:Path) -join ';'
}

function Ensure-NodeAndClaude {
  if (-not (Test-NodeVersion)) {
    if (Get-Command winget -ErrorAction SilentlyContinue) {
      Write-XdtLog 'Installing Node.js with winget'
      winget install OpenJS.NodeJS.LTS --accept-source-agreements --accept-package-agreements
      Refresh-ProcessPath
    }
  }

  if (-not (Test-NodeVersion)) {
    Fail-Xdt 'Node.js 18+ is required. Install Node.js, then rerun this command.'
  }

  if (Get-Command claude -ErrorAction SilentlyContinue) {
    Write-XdtLog 'Claude Code detected'
    return
  }

  Write-XdtLog 'Installing Claude Code with npm'
  & npm install -g '@anthropic-ai/claude-code'
  Refresh-ProcessPath
}

function Find-CcSwitch {
  if (-not [string]::IsNullOrWhiteSpace($env:XDT_CCSWITCH_BIN) -and (Test-Path $env:XDT_CCSWITCH_BIN)) {
    return $env:XDT_CCSWITCH_BIN
  }

  $cmd = Get-Command cc-switch -ErrorAction SilentlyContinue
  if ($cmd) {
    return $cmd.Source
  }

  $candidates = @(
    "$env:LOCALAPPDATA\Programs\CC Switch\cc-switch.exe",
    "$env:ProgramFiles\CC Switch\cc-switch.exe",
    "${env:ProgramFiles(x86)}\CC Switch\cc-switch.exe"
  )

  foreach ($candidate in $candidates) {
    if ($candidate -and (Test-Path $candidate)) {
      return $candidate
    }
  }

  return $null
}

function Test-XdtImportSupport([string]$CcSwitch) {
  if (-not $CcSwitch) { return $false }
  try {
    & $CcSwitch xdt-import --help *> $null
    return $LASTEXITCODE -eq 0
  } catch {
    return $false
  }
}

function Install-XdtCcSwitchForWindows {
  $baseUrl = if ([string]::IsNullOrWhiteSpace($env:XDT_INSTALLER_BASE)) {
    'https://xuedingtoken.com'
  } else {
    $env:XDT_INSTALLER_BASE.TrimEnd('/')
  }
  $url = if ([string]::IsNullOrWhiteSpace($env:XDT_CCSWITCH_WIN_URL)) {
    "$baseUrl/downloads/cc-switch/CC-Switch-XDT-Windows-x64.zip"
  } else {
    $env:XDT_CCSWITCH_WIN_URL
  }

  $tmp = Join-Path ([IO.Path]::GetTempPath()) ("xdt-ccswitch-" + [Guid]::NewGuid().ToString())
  New-Item -ItemType Directory -Path $tmp -Force | Out-Null
  try {
    $package = Join-Path $tmp 'cc-switch-package'
    Write-XdtLog 'Downloading CC Switch enhanced build'
    Invoke-WebRequest -Uri $url -OutFile $package -UseBasicParsing

    if ($url.ToLowerInvariant().EndsWith('.zip')) {
      $unpacked = Join-Path $tmp 'unpacked'
      Expand-Archive -Path $package -DestinationPath $unpacked -Force
      $exe = Get-ChildItem -Path $unpacked -Recurse -Filter 'cc-switch.exe' | Select-Object -First 1
      if (-not $exe) {
        Fail-Xdt 'cc-switch.exe not found in downloaded zip package'
      }
      $targetDir = Join-Path $env:LOCALAPPDATA 'Programs\CC Switch'
      New-Item -ItemType Directory -Path $targetDir -Force | Out-Null
      Copy-Item -Path (Join-Path $exe.DirectoryName '*') -Destination $targetDir -Recurse -Force
    } else {
      Write-XdtLog 'Installing CC Switch MSI'
      $process = Start-Process msiexec.exe -ArgumentList @('/i', $package, '/qn', '/norestart') -Wait -PassThru
      if ($process.ExitCode -ne 0) {
        Fail-Xdt "CC Switch MSI installer failed with exit code $($process.ExitCode)"
      }
    }

    Refresh-ProcessPath
  } finally {
    Remove-Item -Path $tmp -Recurse -Force -ErrorAction SilentlyContinue
  }
}

function Ensure-CcSwitch {
  $ccSwitch = Find-CcSwitch
  if ($ccSwitch -and (Test-XdtImportSupport $ccSwitch)) {
    return $ccSwitch
  }

  if ($ccSwitch) {
    Write-XdtLog 'Existing CC Switch does not support xdt-import; upgrading'
  } else {
    Write-XdtLog 'CC Switch not found; installing'
  }

  Install-XdtCcSwitchForWindows
  $ccSwitch = Find-CcSwitch
  if (-not $ccSwitch) {
    Fail-Xdt 'CC Switch installation completed but binary was not found'
  }
  if (-not (Test-XdtImportSupport $ccSwitch)) {
    Fail-Xdt 'Installed CC Switch does not support xdt-import'
  }
  return $ccSwitch
}

Require-Token
$apiUrl = Normalize-Url $env:XDT_API_URL

Ensure-NodeAndClaude
$ccSwitch = Ensure-CcSwitch

Write-XdtLog 'Importing and switching XueDingToken provider'
& $ccSwitch xdt-import `
  --provider-id xuedingtoken `
  --name XueDingToken `
  --app claude `
  --endpoint $apiUrl `
  --api-key $env:XDT_TOKEN `
  --homepage 'https://xuedingtoken.com' `
  --icon claude `
  --switch

if ($LASTEXITCODE -ne 0) {
  Fail-Xdt "cc-switch xdt-import failed with exit code $LASTEXITCODE"
}

Write-XdtLog 'Claude Code is configured through CC Switch'
if ($env:XDT_SKIP_LAUNCH_CLAUDE -ne '1') {
  Write-XdtLog 'Starting Claude Code'
  & claude
}
