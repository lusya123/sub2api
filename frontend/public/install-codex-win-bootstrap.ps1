$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'

$officialApiUrl = 'https://xuedingtoken1.com/v1'
$currentOfficialBaseUrl = 'https://xuedingtoken1.com'
$legacyOfficialBaseUrl = 'https://xuedingtoken.com'
$staticBaseUrls = @(
  'https://xuedingtoken1.com'
)
$installerPath = '/downloads/codex/XueDingToken-Codex-Installer-Windows-x64.exe'
$installerSha256 = '127d2d652e39b3717acc5138249ca5c1984103fe76b530f24511e8019f4e96ce'

function Write-XdtLog([string]$Message) {
  Write-Host "[XueDingToken] $Message"
}

function Fail-Xdt([string]$Message) {
  throw "[XueDingToken] $Message"
}

function Convert-XdtOfficialUrl([string]$Value) {
  $normalized = $Value.Trim().TrimEnd('/')
  if ($normalized -eq $legacyOfficialBaseUrl -or $normalized.StartsWith($legacyOfficialBaseUrl + '/')) {
    return $currentOfficialBaseUrl + $normalized.Substring($legacyOfficialBaseUrl.Length)
  }
  return $normalized
}

function Test-XdtOfficialEndpoint([string]$Value) {
  if ([string]::IsNullOrWhiteSpace($Value)) {
    return
  }
  $normalized = Convert-XdtOfficialUrl $Value
  if ($normalized -eq $currentOfficialBaseUrl -or $normalized -eq $officialApiUrl) {
    return
  }
  Fail-Xdt "This installer is locked to $officialApiUrl and does not support custom API endpoints"
}

$token = if (-not [string]::IsNullOrWhiteSpace($env:XDT_TOKEN)) {
  $env:XDT_TOKEN
} elseif (-not [string]::IsNullOrWhiteSpace($env:CODEX_TOKEN)) {
  $env:CODEX_TOKEN
} elseif (-not [string]::IsNullOrWhiteSpace($env:OPENAI_API_KEY)) {
  $env:OPENAI_API_KEY
} else {
  $null
}

if ([string]::IsNullOrWhiteSpace($token)) {
  Fail-Xdt 'Missing CODEX_TOKEN'
}

Test-XdtOfficialEndpoint $env:XDT_API_URL
Test-XdtOfficialEndpoint $env:CODEX_API_URL

$env:XDT_TOKEN = $token
$env:CODEX_TOKEN = $token
$env:XDT_API_URL = $officialApiUrl
$env:CODEX_API_URL = $officialApiUrl

[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
$tmp = Join-Path ([IO.Path]::GetTempPath()) ("xdt-codex-bootstrap-" + [Guid]::NewGuid().ToString())
New-Item -ItemType Directory -Path $tmp -Force | Out-Null

try {
  $installer = Join-Path $tmp 'XueDingToken-Codex-Installer-Windows-x64.exe'
  $downloaded = $false
  foreach ($baseUrl in $staticBaseUrls) {
    $url = $baseUrl.TrimEnd('/') + $installerPath
    try {
      Write-XdtLog "Downloading XueDingToken Codex installer: $url"
      Invoke-WebRequest -Uri $url -OutFile $installer -UseBasicParsing -TimeoutSec 180
      $downloaded = $true
      break
    } catch {
      Write-XdtLog "Download failed: $url"
    }
  }
  if (-not $downloaded) {
    Fail-Xdt 'Unable to download XueDingToken Codex installer from static mirrors'
  }

  $actualSha256 = (Get-FileHash -Path $installer -Algorithm SHA256).Hash.ToLowerInvariant()
  if ($actualSha256 -ne $installerSha256) {
    Fail-Xdt 'Downloaded installer checksum mismatch'
  }

  Write-XdtLog 'Starting XueDingToken Codex installer'
  & $installer
  if ($LASTEXITCODE -ne 0) {
    Fail-Xdt "XueDingToken Codex installer failed with exit code $LASTEXITCODE"
  }
} finally {
  Remove-Item -Path $tmp -Recurse -Force -ErrorAction SilentlyContinue
}
