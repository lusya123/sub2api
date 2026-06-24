$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
$script:XdtOfficialApiUrl = 'https://xuedingtoken1.com/v1'
$script:XdtAllowedApiHosts = @('xuedingtoken1.com')
$script:XdtCurrentOfficialBaseUrl = 'https://xuedingtoken1.com'
$script:XdtLegacyOfficialBaseUrl = 'https://xuedingtoken.com'

function Write-XdtLog([string]$Message) {
  Write-Host "[XueDingToken] $Message"
}

function Fail-Xdt([string]$Message) {
  throw "[XueDingToken] $Message"
}

function Convert-XdtOfficialUrl([string]$Value) {
  $normalized = $Value.Trim().TrimEnd('/')
  if ($normalized -eq $script:XdtLegacyOfficialBaseUrl -or $normalized.StartsWith($script:XdtLegacyOfficialBaseUrl + '/')) {
    return $script:XdtCurrentOfficialBaseUrl + $normalized.Substring($script:XdtLegacyOfficialBaseUrl.Length)
  }
  return $normalized
}

function Require-Token {
  if ([string]::IsNullOrWhiteSpace($env:XDT_TOKEN)) {
    if (-not [string]::IsNullOrWhiteSpace($env:CODEX_TOKEN)) {
      $env:XDT_TOKEN = $env:CODEX_TOKEN
    } elseif (-not [string]::IsNullOrWhiteSpace($env:OPENAI_API_KEY)) {
      $env:XDT_TOKEN = $env:OPENAI_API_KEY
    }
  }
  if ([string]::IsNullOrWhiteSpace($env:XDT_TOKEN)) {
    Fail-Xdt 'Missing XDT_TOKEN or CODEX_TOKEN'
  }
}

function Normalize-Url([string]$Value) {
  if ([string]::IsNullOrWhiteSpace($Value)) {
    if (-not [string]::IsNullOrWhiteSpace($env:XDT_API_URL)) {
      $Value = $env:XDT_API_URL
    } elseif (-not [string]::IsNullOrWhiteSpace($env:CODEX_API_URL)) {
      $Value = $env:CODEX_API_URL
    } else {
      return $script:XdtOfficialApiUrl
    }
  }

  $normalized = Convert-XdtOfficialUrl $Value
  if ($normalized -match '^https?://[^/]+$') {
    $normalized = "$normalized/v1"
  }

  try {
    $uri = [Uri]$normalized
    if ($uri.Scheme -ne 'https') {
      Fail-Xdt 'XueDingToken installer only supports the official HTTPS API endpoint'
    }
    if ($script:XdtAllowedApiHosts -notcontains $uri.Host.ToLowerInvariant()) {
      Fail-Xdt "This installer is locked to $script:XdtOfficialApiUrl and does not support custom API endpoints"
    }
    if ($uri.AbsolutePath.TrimEnd('/') -ne '/v1') {
      Fail-Xdt "This installer is locked to $script:XdtOfficialApiUrl and does not support custom API paths"
    }
  } catch {
    if ($_.Exception.Message.StartsWith('[XueDingToken]')) {
      throw
    }
    Fail-Xdt "Invalid XueDingToken API endpoint: $Value"
  }

  return $normalized
}

function Get-XdtWindowsArch {
  if (-not [string]::IsNullOrWhiteSpace($env:XDT_WINDOWS_ARCH)) {
    switch ($env:XDT_WINDOWS_ARCH.Trim().ToLowerInvariant()) {
      'x64' { return 'x64' }
      'amd64' { return 'x64' }
      'x86_64' { return 'x64' }
      'arm64' { return 'arm64' }
      'aarch64' { return 'arm64' }
      default { Fail-Xdt "Unsupported XDT_WINDOWS_ARCH: $env:XDT_WINDOWS_ARCH" }
    }
  }

  $arch = $null
  try {
    $runtimeArch = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString()
    if (-not [string]::IsNullOrWhiteSpace($runtimeArch)) {
      $arch = $runtimeArch
    }
  } catch {}

  if ([string]::IsNullOrWhiteSpace($arch)) {
    if (-not [string]::IsNullOrWhiteSpace($env:PROCESSOR_ARCHITEW6432)) {
      $arch = $env:PROCESSOR_ARCHITEW6432
    } else {
      $arch = $env:PROCESSOR_ARCHITECTURE
    }
  }

  switch ($arch.ToUpperInvariant()) {
    'AMD64' { return 'x64' }
    'X64' { return 'x64' }
    'X86_64' { return 'x64' }
    'ARM64' { return 'arm64' }
    'AARCH64' { return 'arm64' }
    'X86' { Fail-Xdt '32-bit Windows is not supported. Please use 64-bit Windows.' }
    'IA64' { Fail-Xdt 'Itanium Windows is not supported.' }
    default { Fail-Xdt "Unsupported Windows architecture: $arch" }
  }
}

function Invoke-XdtDownload([string[]]$Urls, [string]$OutFile) {
  foreach ($url in $Urls) {
    if ([string]::IsNullOrWhiteSpace($url)) { continue }
    try {
      Write-XdtLog "Downloading: $url"
      Invoke-WebRequest -Uri $url -OutFile $OutFile -UseBasicParsing -TimeoutSec 120
      return $url
    } catch {
      Write-XdtLog "Download failed: $url"
    }
  }
  Fail-Xdt 'All download URLs failed'
}

function Invoke-XdtRestJson([string[]]$Urls) {
  foreach ($url in $Urls) {
    if ([string]::IsNullOrWhiteSpace($url)) { continue }
    try {
      return Invoke-RestMethod -Uri $url -UseBasicParsing -TimeoutSec 60
    } catch {
      Write-XdtLog "Metadata request failed: $url"
    }
  }
  Fail-Xdt 'All metadata URLs failed'
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
  $nodeDir = Join-Path $env:ProgramFiles 'nodejs'
  $nodeArmDir = Join-Path $env:LOCALAPPDATA 'Programs\nodejs-arm64'
  $ccSwitchDir = Join-Path $env:LOCALAPPDATA 'Programs\CC Switch'
  $env:Path = @($nodeDir, $nodeArmDir, $ccSwitchDir, $machine, $user, $env:Path) -join ';'
}

function Add-UserPath([string]$Directory) {
  if ([string]::IsNullOrWhiteSpace($Directory) -or -not (Test-Path $Directory)) {
    return
  }

  $current = [Environment]::GetEnvironmentVariable('Path', 'User')
  $parts = @()
  if (-not [string]::IsNullOrWhiteSpace($current)) {
    $parts = $current.Split(';') | Where-Object { -not [string]::IsNullOrWhiteSpace($_) }
  }

  $alreadyExists = $false
  foreach ($part in $parts) {
    if ($part.TrimEnd('\') -ieq $Directory.TrimEnd('\')) {
      $alreadyExists = $true
      break
    }
  }

  if (-not $alreadyExists) {
    $newPath = if ([string]::IsNullOrWhiteSpace($current)) { $Directory } else { "$current;$Directory" }
    [Environment]::SetEnvironmentVariable('Path', $newPath, 'User')
  }

  Refresh-ProcessPath
}

function New-XdtShortcut([string]$ShortcutPath, [string]$TargetPath, [string]$WorkingDirectory) {
  try {
    $parent = Split-Path -Parent $ShortcutPath
    New-Item -ItemType Directory -Path $parent -Force | Out-Null
    $shell = New-Object -ComObject WScript.Shell
    $shortcut = $shell.CreateShortcut($ShortcutPath)
    $shortcut.TargetPath = $TargetPath
    $shortcut.WorkingDirectory = $WorkingDirectory
    $shortcut.IconLocation = "$TargetPath,0"
    $shortcut.Description = 'CC Switch'
    $shortcut.Save()
  } catch {
    Write-XdtLog "Shortcut creation skipped: $ShortcutPath"
  }
}

function Install-CcSwitchShellIntegration([string]$CcSwitch) {
  if ([string]::IsNullOrWhiteSpace($CcSwitch) -or -not (Test-Path $CcSwitch)) {
    return
  }

  $installDir = Split-Path -Parent $CcSwitch
  Add-UserPath $installDir

  $startMenu = Join-Path $env:APPDATA 'Microsoft\Windows\Start Menu\Programs\CC Switch.lnk'
  $desktop = Join-Path ([Environment]::GetFolderPath('Desktop')) 'CC Switch.lnk'
  New-XdtShortcut $startMenu $CcSwitch $installDir
  New-XdtShortcut $desktop $CcSwitch $installDir
}

function Test-XdtInteractiveDesktop {
  if (-not [string]::IsNullOrWhiteSpace($env:SSH_CONNECTION) -or
      -not [string]::IsNullOrWhiteSpace($env:SSH_CLIENT) -or
      -not [string]::IsNullOrWhiteSpace($env:SSH_TTY)) {
    return $false
  }

  try {
    return [Environment]::UserInteractive
  } catch {
    return $true
  }
}

function Start-CcSwitchGui([string]$CcSwitch) {
  if ($env:XDT_SKIP_LAUNCH_CCSWITCH -eq '1') {
    return
  }
  if ([string]::IsNullOrWhiteSpace($CcSwitch) -or -not (Test-Path $CcSwitch)) {
    return
  }
  if (-not (Test-XdtInteractiveDesktop)) {
    Write-XdtLog 'CC Switch GUI launch skipped because this is not an interactive desktop session'
    return
  }

  try {
    $currentSessionId = (Get-Process -Id $PID).SessionId
    Get-Process cc-switch -ErrorAction SilentlyContinue |
      Where-Object { $_.SessionId -ne $currentSessionId } |
      Stop-Process -Force -ErrorAction SilentlyContinue
  } catch {}

  Write-XdtLog 'Starting CC Switch'
  Start-Process -FilePath $CcSwitch -WorkingDirectory (Split-Path -Parent $CcSwitch) | Out-Null
}

function Start-CodexTerminal {
  if ($env:XDT_SKIP_LAUNCH_CODEX -eq '1') {
    return
  }
  if (-not (Test-XdtInteractiveDesktop)) {
    Write-XdtLog 'Codex CLI launch skipped because this is not an interactive desktop session'
    return
  }

  Write-XdtLog 'Starting Codex CLI in a new PowerShell window'
  $codexLauncher = 'codex'
  $codexCmd = Get-Command 'codex.cmd' -ErrorAction SilentlyContinue
  if ($codexCmd) {
    $codexLauncher = $codexCmd.Source
  }
  $codexLauncher = $codexLauncher.Replace("'", "''")
  $command = @"
`$Host.UI.RawUI.WindowTitle = 'XueDingToken Codex'
`$env:TERM = 'xterm-256color'
Write-Host ''
Write-Host '[XueDingToken] Codex is ready. You can start chatting in this window.' -ForegroundColor Green
Write-Host ''
& '$codexLauncher'
"@
  $workingDirectory = [Environment]::GetFolderPath('UserProfile')

  try {
    Start-Process -FilePath 'powershell.exe' -WorkingDirectory $workingDirectory -ArgumentList @(
      '-NoExit',
      '-ExecutionPolicy',
      'Bypass',
      '-Command',
      $command
    ) | Out-Null
  } catch {
    Write-XdtLog "Unable to open Codex PowerShell window automatically: $($_.Exception.Message)"
  }
}

function Initialize-CodexSandbox {
  if ($env:XDT_SKIP_CODEX_SANDBOX_PREWARM -eq '1') {
    return
  }

  if ([Console]::IsInputRedirected) {
    Write-XdtLog 'Codex sandbox prewarm skipped because this is not an interactive terminal'
    return
  }

  Write-XdtLog 'Preparing Codex sandbox'
  $codexCommand = Get-Command codex.cmd -ErrorAction SilentlyContinue
  if (-not $codexCommand) {
    $codexCommand = Get-Command codex -ErrorAction SilentlyContinue
  }
  if (-not $codexCommand) {
    Write-XdtLog 'Codex sandbox prewarm skipped because Codex CLI was not found on PATH'
    return
  }

  $previousTerm = $env:TERM
  if ([string]::IsNullOrWhiteSpace($env:TERM) -or $env:TERM -eq 'dumb') {
    $env:TERM = 'xterm-256color'
  }
  try {
    & $codexCommand.Source @(
      'sandbox',
      'windows',
      '--permissions-profile',
      ':workspace',
      '--cd',
      $env:USERPROFILE,
      'cmd.exe',
      '/c',
      'exit',
      '0'
    )
    $exitCode = $LASTEXITCODE

    if ($exitCode -eq 0) {
      Write-XdtLog 'Codex sandbox is ready'
    } else {
      Write-XdtLog "Codex sandbox prewarm failed with exit code $exitCode; Codex will finish setup on first launch"
    }
  } catch {
    Write-XdtLog "Codex sandbox prewarm failed; Codex will finish setup on first launch: $($_.Exception.Message)"
  } finally {
    $env:TERM = $previousTerm
  }
}

function Test-VcRuntime {
  $system32 = [Environment]::GetFolderPath('System')
  return (Test-Path (Join-Path $system32 'VCRUNTIME140.dll')) -and
    (Test-Path (Join-Path $system32 'VCRUNTIME140_1.dll'))
}

function Install-VcRuntimeFile([string]$VcFile) {
  Write-XdtLog "Installing Microsoft Visual C++ Runtime: $VcFile"
  [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
  $tmp = Join-Path ([IO.Path]::GetTempPath()) ("xdt-vcredist-" + [Guid]::NewGuid().ToString())
  New-Item -ItemType Directory -Path $tmp -Force | Out-Null
  try {
    $installer = Join-Path $tmp $VcFile
    Invoke-WebRequest -Uri "https://aka.ms/vs/17/release/$VcFile" -OutFile $installer -UseBasicParsing -TimeoutSec 120
    $process = Start-Process -FilePath $installer -ArgumentList @('/install', '/quiet', '/norestart') -Wait -PassThru
    if ($process.ExitCode -ne 0 -and $process.ExitCode -ne 3010 -and $process.ExitCode -ne 1638) {
      Fail-Xdt "Microsoft Visual C++ Runtime installer failed with exit code $($process.ExitCode)"
    }
  } finally {
    Remove-Item -Path $tmp -Recurse -Force -ErrorAction SilentlyContinue
  }
}

function Ensure-VcRuntime {
  $arch = Get-XdtWindowsArch
  if ($arch -eq 'arm64') {
    if (-not (Test-VcRuntime)) {
      Install-VcRuntimeFile 'vc_redist.arm64.exe'
    }
    # CC Switch currently ships as a Windows x64 executable, which runs on
    # Windows ARM64 through x64 emulation, so install the x64 runtime too.
    Install-VcRuntimeFile 'vc_redist.x64.exe'
    return
  }

  if (Test-VcRuntime) {
    return
  }

  Install-VcRuntimeFile 'vc_redist.x64.exe'

  if (-not (Test-VcRuntime)) {
    Fail-Xdt 'Microsoft Visual C++ Runtime installation finished but required DLLs were not found'
  }
}

function Test-WebView2Runtime {
  $runtimeRoots = @(
    (Join-Path ${env:ProgramFiles(x86)} 'Microsoft\EdgeWebView\Application'),
    (Join-Path $env:ProgramFiles 'Microsoft\EdgeWebView\Application'),
    (Join-Path $env:LOCALAPPDATA 'Microsoft\EdgeWebView\Application')
  )

  foreach ($root in $runtimeRoots) {
    if ([string]::IsNullOrWhiteSpace($root) -or -not (Test-Path $root)) {
      continue
    }

    $runtimeExe = Get-ChildItem -Path $root -Filter 'msedgewebview2.exe' -Recurse -ErrorAction SilentlyContinue |
      Select-Object -First 1
    if ($runtimeExe) {
      return $true
    }
  }

  $clientId = '{F3017226-FE2A-4295-8BDF-00C3A9C7E4C5}'
  $registryPaths = @(
    "HKLM:\SOFTWARE\Microsoft\EdgeUpdate\Clients\$clientId",
    "HKLM:\SOFTWARE\WOW6432Node\Microsoft\EdgeUpdate\Clients\$clientId",
    "HKCU:\SOFTWARE\Microsoft\EdgeUpdate\Clients\$clientId",
    "HKCU:\SOFTWARE\WOW6432Node\Microsoft\EdgeUpdate\Clients\$clientId"
  )

  foreach ($path in $registryPaths) {
    try {
      $props = Get-ItemProperty -Path $path -ErrorAction Stop
      if (-not [string]::IsNullOrWhiteSpace($props.pv)) {
        return $true
      }
    } catch {}
  }

  return $false
}

function Ensure-WebView2Runtime {
  if ($env:XDT_SKIP_WEBVIEW2 -eq '1') {
    return
  }
  if (Test-WebView2Runtime) {
    Write-XdtLog 'Microsoft Edge WebView2 Runtime detected'
    return
  }

  Write-XdtLog 'Installing Microsoft Edge WebView2 Runtime'
  [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
  $baseUrl = if ([string]::IsNullOrWhiteSpace($env:XDT_INSTALLER_BASE)) {
    'https://xuedingtoken1.com'
  } else {
    $env:XDT_INSTALLER_BASE.TrimEnd('/')
  }
  $tmp = Join-Path ([IO.Path]::GetTempPath()) ("xdt-webview2-" + [Guid]::NewGuid().ToString())
  New-Item -ItemType Directory -Path $tmp -Force | Out-Null
  try {
    $arch = Get-XdtWindowsArch
    $installerName = if ($arch -eq 'arm64') {
      'MicrosoftEdgeWebView2RuntimeInstallerARM64.exe'
    } else {
      'MicrosoftEdgeWebView2RuntimeInstallerX64.exe'
    }
    $urls = if ($arch -eq 'arm64') {
      @(
        "$baseUrl/downloads/webview2/MicrosoftEdgeWebView2RuntimeInstallerARM64.exe",
        'https://go.microsoft.com/fwlink/p/?LinkId=2124703'
      )
    } else {
      @(
        "$baseUrl/downloads/webview2/MicrosoftEdgeWebView2RuntimeInstallerX64.exe",
        'https://go.microsoft.com/fwlink/p/?LinkId=2124701',
        'https://go.microsoft.com/fwlink/p/?LinkId=2124703'
      )
    }
    $installer = Join-Path $tmp $installerName
    Invoke-XdtDownload $urls $installer | Out-Null
    $process = Start-Process -FilePath $installer -ArgumentList @('/silent', '/install') -PassThru
    if (-not $process.WaitForExit(300000)) {
      if (Test-WebView2Runtime) {
        Write-XdtLog 'Microsoft Edge WebView2 Runtime installed; installer is still finishing in background'
        Stop-Process -Id $process.Id -Force -ErrorAction SilentlyContinue
      } else {
        Stop-Process -Id $process.Id -Force -ErrorAction SilentlyContinue
        Fail-Xdt 'Microsoft Edge WebView2 Runtime installer timed out'
      }
    }

    if ($process.HasExited -and $process.ExitCode -ne 0 -and $process.ExitCode -ne 3010 -and $process.ExitCode -ne 1638) {
      Fail-Xdt "Microsoft Edge WebView2 Runtime installer failed with exit code $($process.ExitCode)"
    }
  } finally {
    Remove-Item -Path $tmp -Recurse -Force -ErrorAction SilentlyContinue
  }

  if (-not (Test-WebView2Runtime)) {
    Fail-Xdt 'Microsoft Edge WebView2 Runtime installation finished but runtime was not found'
  }
}

function Install-NodeWithZip([string]$Version, [string]$NodeMirror, [string]$Arch) {
  Write-XdtLog "Installing Node.js LTS with zip for Windows $Arch"
  $zipName = "node-$Version-win-$Arch.zip"
  $installDir = Join-Path $env:LOCALAPPDATA "Programs\nodejs-$Arch"
  $tmp = Join-Path ([IO.Path]::GetTempPath()) ("xdt-node-" + [Guid]::NewGuid().ToString())
  New-Item -ItemType Directory -Path $tmp -Force | Out-Null
  try {
    $zip = Join-Path $tmp 'node-lts.zip'
    Invoke-XdtDownload @(
      "$NodeMirror/$Version/$zipName",
      "https://nodejs.org/dist/$Version/$zipName"
    ) $zip | Out-Null
    if (-not (Test-ZipFile $zip)) {
      Fail-Xdt "Downloaded Node.js package is not a zip file: $zipName"
    }
    $unpacked = Join-Path $tmp 'unpacked'
    Expand-Archive -Path $zip -DestinationPath $unpacked -Force
    $root = Join-Path $unpacked "node-$Version-win-$Arch"
    if (-not (Test-Path (Join-Path $root 'node.exe'))) {
      Fail-Xdt "Node.js executable not found in zip package: $zipName"
    }
    Remove-Item -Path $installDir -Recurse -Force -ErrorAction SilentlyContinue
    New-Item -ItemType Directory -Path $installDir -Force | Out-Null
    Copy-Item -Path (Join-Path $root '*') -Destination $installDir -Recurse -Force
    Add-UserPath $installDir
  } finally {
    Remove-Item -Path $tmp -Recurse -Force -ErrorAction SilentlyContinue
  }
}

function Install-NodeLts {
  $arch = Get-XdtWindowsArch
  Write-XdtLog "Installing Node.js LTS for Windows $arch"
  [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

  $nodeMirror = if ([string]::IsNullOrWhiteSpace($env:XDT_NODE_MIRROR)) {
    'https://npmmirror.com/mirrors/node'
  } else {
    $env:XDT_NODE_MIRROR.TrimEnd('/')
  }
  $index = Invoke-XdtRestJson @(
    "$nodeMirror/index.json",
    'https://nodejs.org/dist/index.json'
  )
  $release = $index |
    Where-Object {
      $_.lts -and
      $_.version -match '^v(\d+)\.' -and
      [int]$Matches[1] -ge 18 -and
      (
        ($arch -eq 'x64' -and $_.files -contains 'win-x64-msi') -or
        ($arch -eq 'arm64' -and $_.files -contains 'win-arm64-zip')
      )
    } |
    Select-Object -First 1

  if (-not $release) {
    Fail-Xdt "Could not find a supported Node.js LTS Windows $arch release"
  }

  $version = $release.version
  if ($arch -eq 'arm64') {
    Install-NodeWithZip $version $nodeMirror 'arm64'
    Refresh-ProcessPath
    return
  }

  $msiName = "node-$version-x64.msi"
  $tmp = Join-Path ([IO.Path]::GetTempPath()) ("xdt-node-" + [Guid]::NewGuid().ToString())
  New-Item -ItemType Directory -Path $tmp -Force | Out-Null
  try {
    $msi = Join-Path $tmp 'node-lts-x64.msi'
    Invoke-XdtDownload @(
      "$nodeMirror/$version/$msiName",
      "https://nodejs.org/dist/$version/$msiName"
    ) $msi | Out-Null
    $process = Start-Process msiexec.exe -ArgumentList @('/i', $msi, '/qn', '/norestart') -Wait -PassThru
    if ($process.ExitCode -ne 0 -and $process.ExitCode -ne 3010) {
      Fail-Xdt "Node.js MSI installer failed with exit code $($process.ExitCode)"
    }
    Refresh-ProcessPath
  } finally {
    Remove-Item -Path $tmp -Recurse -Force -ErrorAction SilentlyContinue
  }
}

function Get-NpmInstallRegistries {
  $registries = New-Object System.Collections.Generic.List[string]
  if (-not [string]::IsNullOrWhiteSpace($env:XDT_NPM_REGISTRY)) {
    $registries.Add($env:XDT_NPM_REGISTRY.TrimEnd('/')) | Out-Null
  }

  try {
    $currentRegistry = (& npm config get registry 2>$null | Select-Object -First 1)
    if (-not [string]::IsNullOrWhiteSpace($currentRegistry)) {
      $currentRegistry = $currentRegistry.Trim().TrimEnd('/')
      if ($currentRegistry -notmatch '(?i)^https?://registry\.npmjs\.org/?$') {
        $registries.Add($currentRegistry) | Out-Null
      }
    }
  } catch {}

  $registries.Add('https://registry.npmmirror.com') | Out-Null
  if (-not [string]::IsNullOrWhiteSpace($currentRegistry) -and
      $currentRegistry -match '(?i)^https?://registry\.npmjs\.org/?$') {
    $registries.Add($currentRegistry) | Out-Null
  }
  $registries.Add('https://registry.npmjs.org') | Out-Null

  $deduped = New-Object System.Collections.Generic.List[string]
  foreach ($registry in $registries) {
    if ([string]::IsNullOrWhiteSpace($registry)) { continue }
    if (-not $deduped.Contains($registry)) {
      $deduped.Add($registry) | Out-Null
    }
  }
  return @($deduped.ToArray())
}

function Test-NpmGlobalPackage([string]$PackageName) {
  try {
    & npm ls -g $PackageName --depth=0 --json *> $null
    return $LASTEXITCODE -eq 0
  } catch {
    return $false
  }
}

function Get-NpmGlobalPrefix {
  try {
    $prefix = (& npm prefix -g 2>$null | Select-Object -First 1)
    if (-not [string]::IsNullOrWhiteSpace($prefix)) {
      return $prefix.Trim()
    }
  } catch {}

  return (Join-Path $env:APPDATA 'npm')
}

function Remove-CodexNpmArtifacts {
  $prefix = Get-NpmGlobalPrefix
  $paths = @(
    (Join-Path $prefix 'codex'),
    (Join-Path $prefix 'codex.cmd'),
    (Join-Path $prefix 'codex.ps1'),
    (Join-Path $prefix 'node_modules\codex-cli'),
    (Join-Path $prefix 'node_modules\@openai\codex')
  )

  foreach ($path in $paths) {
    Remove-Item -Path $path -Recurse -Force -ErrorAction SilentlyContinue
  }
}

function Remove-LegacyCodexCli {
  if (-not (Get-Command npm -ErrorAction SilentlyContinue)) {
    return
  }

  $hasLegacyPackage = Test-NpmGlobalPackage 'codex-cli'
  $codexCommand = Get-Command codex -ErrorAction SilentlyContinue
  $looksLegacy = $false
  if ($codexCommand) {
    try {
      $versionOutput = (& $codexCommand.Source --version 2>&1 | Out-String)
      if ($versionOutput -match '(?i)bson|mongodb|padLevels') {
        $looksLegacy = $true
      }
    } catch {
      $looksLegacy = $true
    }
  }

  if (-not $hasLegacyPackage -and -not $looksLegacy) {
    return
  }

  Write-XdtLog 'Removing deprecated npm package codex-cli that shadows the official Codex CLI'
  try {
    & npm uninstall -g codex-cli '@openai/codex' --force *> $null
  } catch {}
  Remove-CodexNpmArtifacts
  Refresh-ProcessPath
}

function Test-OfficialCodexCli {
  if (-not (Get-Command codex -ErrorAction SilentlyContinue)) {
    return $false
  }
  if (-not (Test-NpmGlobalPackage '@openai/codex')) {
    return $false
  }

  try {
    $codexCommand = Get-Command codex -ErrorAction Stop
    $versionOutput = (& $codexCommand.Source --version 2>&1 | Out-String)
    if ($LASTEXITCODE -ne 0) {
      return $false
    }
    if ($versionOutput -match '(?i)bson|mongodb|padLevels') {
      return $false
    }
    return ($versionOutput -match '(?i)codex')
  } catch {
    return $false
  }
}

function Clear-NpmCacheForRetry {
  try {
    Write-XdtLog 'Cleaning npm cache before retry'
    & npm cache clean --force *> $null
  } catch {}
}

function Install-CodexCli {
  $registries = Get-NpmInstallRegistries
  $attempt = 0
  $lastNpmExitCode = 0
  $sawInstallSuccess = $false

  $env:npm_config_fetch_retries = '1'
  $env:npm_config_fetch_retry_mintimeout = '5000'
  $env:npm_config_fetch_retry_maxtimeout = '15000'
  $env:npm_config_fetch_timeout = '30000'

  foreach ($registry in $registries) {
    $attempt++
    Write-XdtLog "Installing official Codex CLI with npm ($registry)"
    $env:npm_config_registry = $registry
    & npm install -g '@openai/codex@latest' --registry $registry
    $lastNpmExitCode = $LASTEXITCODE
    if ($LASTEXITCODE -eq 0) {
      $sawInstallSuccess = $true
      Refresh-ProcessPath
      if (Test-OfficialCodexCli) {
        return
      }
      Write-XdtLog 'npm reported success but the official Codex CLI is still not active'
    } else {
      Remove-CodexNpmArtifacts
    }

    if ($attempt -eq 1) {
      Clear-NpmCacheForRetry
    }
  }

  if ($sawInstallSuccess) {
    Fail-Xdt 'Codex CLI npm installation finished but the official codex command is still not active'
  }
  Fail-Xdt "Codex CLI npm installation failed with exit code $lastNpmExitCode"
}

function Assert-CodexCliUsable {
  if (Test-OfficialCodexCli) {
    return
  }

  Remove-LegacyCodexCli
  Install-CodexCli

  if (-not (Test-OfficialCodexCli)) {
    Fail-Xdt 'Official Codex CLI installation completed but the codex command is still not usable'
  }
}

function Ensure-NodeAndCodex {
  if (-not (Test-NodeVersion)) {
    if (Get-Command winget -ErrorAction SilentlyContinue) {
      Write-XdtLog 'Installing Node.js with winget'
      winget install OpenJS.NodeJS.LTS --accept-source-agreements --accept-package-agreements
      Refresh-ProcessPath
    }
  }

  if (-not (Test-NodeVersion)) {
    Install-NodeLts
  }

  if (-not (Test-NodeVersion)) {
    Fail-Xdt 'Node.js 18+ is required. Install Node.js, then rerun this command.'
  }

  Remove-LegacyCodexCli

  if (Test-OfficialCodexCli) {
    Write-XdtLog 'Codex CLI detected'
    return
  }

  Install-CodexCli
  Assert-CodexCliUsable
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
  $tmp = Join-Path ([IO.Path]::GetTempPath()) ("xdt-ccswitch-help-" + [Guid]::NewGuid().ToString())
  $stdout = "$tmp.out"
  $stderr = "$tmp.err"
  try {
    $process = Start-Process -FilePath $CcSwitch -ArgumentList @('xdt-import', '--help') -Wait -PassThru -WindowStyle Hidden -RedirectStandardOutput $stdout -RedirectStandardError $stderr
    $text = ''
    if (Test-Path $stdout) { $text += Get-Content -Raw -Path $stdout }
    if (Test-Path $stderr) { $text += Get-Content -Raw -Path $stderr }
    return ($process.ExitCode -eq 0 -and $text -match 'xdt-import' -and $text -match 'codex')
  } catch {
    return $false
  } finally {
    Remove-Item -Path $stdout,$stderr -Force -ErrorAction SilentlyContinue
  }
}

function Test-KnownBadCcSwitchBuild([string]$CcSwitch) {
  if ([string]::IsNullOrWhiteSpace($CcSwitch) -or -not (Test-Path $CcSwitch)) {
    return $false
  }

  try {
    $stream = [IO.File]::OpenRead($CcSwitch)
    try {
      $reader = New-Object IO.BinaryReader($stream)
      $stream.Seek(0x3c, [IO.SeekOrigin]::Begin) | Out-Null
      $peOffset = $reader.ReadInt32()
      $stream.Seek($peOffset + 24 + 68, [IO.SeekOrigin]::Begin) | Out-Null
      $subsystem = $reader.ReadUInt16()
      if ($subsystem -eq 3) {
        return $true
      }
    } finally {
      $stream.Close()
    }
  } catch {}

  $knownBadHashes = @(
    # Built without Tauri production resource embedding; GUI loads http://localhost:3000.
    '26CD1B76957BBFC68773CD1CD86FF67D9A45C6B0DF5A139FF79BE716BB8A7A25',
    '75944F638DD118AA15DC26ECA6E537CA8E07A049EFC4018E751554B732EB6A2D',
    'EE59B4AABDAD80E4E06008DAB9C8C7D00F5A83F9CE79F2249D460A1FE10F9D6B'
  )

  try {
    $hash = (Get-FileHash -Path $CcSwitch -Algorithm SHA256).Hash.ToUpperInvariant()
    return $knownBadHashes -contains $hash
  } catch {
    return $false
  }
}

function Stop-CcSwitchProcesses {
  try {
    Get-Process cc-switch -ErrorAction SilentlyContinue |
      Stop-Process -Force -ErrorAction SilentlyContinue
    Start-Sleep -Milliseconds 500
  } catch {}
}

function Backup-CcSwitchInstall([string]$InstallDir, [string]$TempDir) {
  if ([string]::IsNullOrWhiteSpace($InstallDir) -or -not (Test-Path $InstallDir)) {
    return $null
  }

  $backupDir = Join-Path $TempDir 'cc-switch-backup'
  try {
    Copy-Item -Path $InstallDir -Destination $backupDir -Recurse -Force
    return $backupDir
  } catch {
    Write-XdtLog "CC Switch backup skipped: $($_.Exception.Message)"
    return $null
  }
}

function Restore-CcSwitchInstall([string]$BackupDir, [string]$InstallDir) {
  if ([string]::IsNullOrWhiteSpace($BackupDir) -or
      [string]::IsNullOrWhiteSpace($InstallDir) -or
      -not (Test-Path $BackupDir)) {
    return
  }

  try {
    Stop-CcSwitchProcesses
    Remove-Item -Path $InstallDir -Recurse -Force -ErrorAction SilentlyContinue
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    Copy-Item -Path (Join-Path $BackupDir '*') -Destination $InstallDir -Recurse -Force
    Write-XdtLog 'Restored the previous CC Switch installation'
  } catch {
    Write-XdtLog "Unable to restore previous CC Switch installation: $($_.Exception.Message)"
  }
}

function Invoke-XdtImport([string]$CcSwitch, [string]$ApiUrl, [string]$Token) {
  $args = @(
    'xdt-import',
    '--provider-id', 'xuedingtoken',
    '--name', 'XueDingToken',
    '--app', 'codex',
    '--endpoint', $ApiUrl,
    '--api-key', $Token,
    '--homepage', 'https://xuedingtoken1.com',
    '--icon', 'codex',
    '--switch'
  )

  $process = Start-Process -FilePath $CcSwitch -ArgumentList $args -Wait -PassThru -WindowStyle Hidden
  if ($process.ExitCode -ne 0) {
    Fail-Xdt "cc-switch xdt-import failed with exit code $($process.ExitCode)"
  }

  $authPath = Join-Path $env:USERPROFILE '.codex\auth.json'
  if (-not (Test-Path $authPath)) {
    Fail-Xdt "cc-switch xdt-import finished but Codex auth was not created: $authPath"
  }

  try {
    $auth = Get-Content -Raw -Path $authPath | ConvertFrom-Json
    if ($auth.OPENAI_API_KEY -ne $Token) {
      Fail-Xdt 'cc-switch xdt-import finished but Codex auth does not match the requested provider'
    }

    $configPath = Join-Path $env:USERPROFILE '.codex\config.toml'
    if (-not (Test-Path $configPath)) {
      Fail-Xdt "cc-switch xdt-import finished but Codex config was not created: $configPath"
    }
    $config = Get-Content -Raw -Path $configPath
    if ($config -notmatch [regex]::Escape($ApiUrl)) {
      Fail-Xdt 'cc-switch xdt-import finished but Codex config does not match the requested endpoint'
    }
  } catch {
    Fail-Xdt "Unable to verify Codex settings after cc-switch import: $($_.Exception.Message)"
  }
}

function Get-CodexModelSortKey([string]$ModelId) {
  $numbers = @([regex]::Matches($ModelId, '\d+') | ForEach-Object { [int64]$_.Value })
  $versionNumbers = @($numbers | Where-Object { $_ -lt 10000000 })
  $dateNumbers = @($numbers | Where-Object { $_ -ge 10000000 })
  $parts = New-Object System.Collections.Generic.List[string]
  for ($i = 0; $i -lt 6; $i++) {
    $value = if ($i -lt $versionNumbers.Count) { $versionNumbers[$i] } else { 0 }
    $parts.Add($value.ToString('D12'))
  }
  $dateValue = if ($dateNumbers.Count -gt 0) { ($dateNumbers | Sort-Object -Descending | Select-Object -First 1) } else { 0 }
  $parts.Add($dateValue.ToString('D12'))
  return ($parts -join '.')
}

function Select-CodexModelCandidates([string[]]$ModelIds) {
  if (-not $ModelIds -or $ModelIds.Count -eq 0) {
    Fail-Xdt 'No models were returned by the provider'
  }

  $preferredPatterns = @(
    '(?i)^gpt-5',
    '(?i)codex',
    '(?i)^gpt-',
    '(?i)^o[0-9]'
  )
  $fallbackPatterns = @(
    '(?i)^glm-5',
    '(?i)minimax',
    '(?i)claude.*sonnet',
    '(?i)claude.*haiku',
    '(?i)claude'
  )

  $ordered = New-Object System.Collections.Generic.List[string]
  $pushCandidates = {
    param([string[]]$Patterns)
    foreach ($pattern in $Patterns) {
      $candidates = $ModelIds |
        Where-Object { $_ -match $pattern } |
        Sort-Object @{ Expression = { Get-CodexModelSortKey $_ }; Descending = $true }, @{ Expression = { $_ }; Descending = $true }
      foreach ($candidate in $candidates) {
        if (-not $ordered.Contains($candidate)) {
          $ordered.Add($candidate) | Out-Null
        }
      }
    }
  }

  & $pushCandidates $preferredPatterns
  if (-not ($ModelIds -contains 'gpt-5.5')) {
    $ordered.Add('gpt-5.5') | Out-Null
  }
  & $pushCandidates $fallbackPatterns
  $rest = $ModelIds |
    Where-Object { -not $ordered.Contains($_) } |
    Sort-Object @{ Expression = { Get-CodexModelSortKey $_ }; Descending = $true }, @{ Expression = { $_ }; Descending = $true }
  foreach ($candidate in $rest) {
    $ordered.Add($candidate) | Out-Null
  }

  return @($ordered.ToArray())
}

function Test-CodexResponsesModel([string]$ApiUrl, [string]$Token, [string]$Model) {
  $responsesUrl = "$($ApiUrl.TrimEnd('/'))/responses"
  $body = @{
    model = $Model
    input = 'Reply with OK.'
    max_output_tokens = 4
  } | ConvertTo-Json -Depth 8 -Compress

  try {
    Invoke-RestMethod -Method Post -Uri $responsesUrl -Headers @{ Authorization = "Bearer $Token" } -ContentType 'application/json' -Body $body -TimeoutSec 30 -UseBasicParsing | Out-Null
    return $true
  } catch {
    $statusCode = $null
    try { $statusCode = [int]$_.Exception.Response.StatusCode } catch {}
    if ($statusCode -eq 401 -or $statusCode -eq 403) {
      Fail-Xdt "The API key was rejected by $responsesUrl with HTTP $statusCode."
    }
    $message = $_.Exception.Message
    try {
      $stream = $_.Exception.Response.GetResponseStream()
      if ($stream) {
        $reader = New-Object System.IO.StreamReader($stream)
        $text = $reader.ReadToEnd()
        if (-not [string]::IsNullOrWhiteSpace($text)) {
          $message = $text
        }
      }
    } catch {}
    if ($message.Length -gt 160) {
      $message = $message.Substring(0, 160)
    }
    Write-XdtLog "Codex model probe failed for ${Model}: HTTP $statusCode $message"
    return $false
  }
}

function Select-WorkingCodexModel([string]$ApiUrl, [string]$Token, [string[]]$ModelIds) {
  $candidates = Select-CodexModelCandidates $ModelIds
  foreach ($candidate in $candidates) {
    if (Test-CodexResponsesModel $ApiUrl $Token $candidate) {
      if ($candidate -notmatch '(?i)^(gpt-|o[0-9])' -and $candidate -notmatch '(?i)codex') {
        Write-XdtLog "GPT/Codex models are not currently available through this key. Using working Responses model: $candidate"
      }
      return $candidate
    }
  }

  Fail-Xdt "No returned model can be called through $($ApiUrl.TrimEnd('/'))/responses"
}

function Resolve-CodexModel([string]$ApiUrl, [string]$Token) {
  if (-not [string]::IsNullOrWhiteSpace($env:XDT_CODEX_MODEL)) {
    return $env:XDT_CODEX_MODEL.Trim()
  }
  if (-not [string]::IsNullOrWhiteSpace($env:CODEX_MODEL)) {
    return $env:CODEX_MODEL.Trim()
  }

  $selected = 'gpt-5.5'
  Write-XdtLog "Selected Codex model: $selected"
  return $selected
}

function Clear-ReadOnlyFile([string]$Path) {
  if ([string]::IsNullOrWhiteSpace($Path) -or -not (Test-Path $Path)) {
    return
  }
  try {
    $item = Get-Item -LiteralPath $Path -ErrorAction Stop
    if (($item.Attributes -band [IO.FileAttributes]::ReadOnly) -ne 0) {
      $item.Attributes = ($item.Attributes -band (-bnot [IO.FileAttributes]::ReadOnly))
    }
  } catch {
    Write-XdtLog "Unable to clear read-only attribute for $Path; continuing"
  }
}

function Write-CodexDirectConfig([string]$ApiUrl, [string]$Token) {
  $codexDir = Join-Path $env:USERPROFILE '.codex'
  New-Item -ItemType Directory -Path $codexDir -Force | Out-Null

  $authPath = Join-Path $codexDir 'auth.json'
  $auth = [ordered]@{
    auth_mode = 'apikey'
    OPENAI_API_KEY = $Token
  }
  $utf8NoBom = New-Object System.Text.UTF8Encoding($false)
  Clear-ReadOnlyFile $authPath
  [IO.File]::WriteAllText($authPath, (($auth | ConvertTo-Json -Depth 4) + "`n"), $utf8NoBom)

  $model = Resolve-CodexModel $ApiUrl $Token
  $escapedUrl = $ApiUrl.Replace('\', '\\').Replace('"', '\"')
  $escapedModel = $model.Replace('\', '\\').Replace('"', '\"')
  $escapedProjectPath = $env:USERPROFILE.Replace('\', '\\').Replace('"', '\"')
  $configPath = Join-Path $codexDir 'config.toml'
  Clear-ReadOnlyFile $configPath
$config = @"
model_provider = "xuedingtoken"
model = "$escapedModel"
review_model = "$escapedModel"
model_reasoning_effort = "xhigh"
approval_policy = "on-request"
default_permissions = ":workspace"
disable_response_storage = true
network_access = "enabled"
model_context_window = 1000000
model_auto_compact_token_limit = 900000

[model_providers.xuedingtoken]
name = "XueDingToken"
base_url = "$escapedUrl"
wire_api = "responses"
requires_openai_auth = true

[projects."$escapedProjectPath"]
trust_level = "trusted"
"@
  [IO.File]::WriteAllText($configPath, $config, $utf8NoBom)

  try {
    $writtenAuth = Get-Content -Raw -Path $authPath | ConvertFrom-Json
    if ($writtenAuth.OPENAI_API_KEY -ne $Token) {
      Fail-Xdt 'Codex auth direct configuration does not match the requested provider'
    }
    $writtenConfig = Get-Content -Raw -Path $configPath
    if ($writtenConfig -notmatch [regex]::Escape($ApiUrl)) {
      Fail-Xdt 'Codex config direct configuration does not match the requested endpoint'
    }
  } catch {
    Fail-Xdt "Unable to verify Codex direct configuration: $($_.Exception.Message)"
  }
}

function Test-ZipFile([string]$Path) {
  if (-not (Test-Path $Path)) { return $false }
  try {
    $stream = [IO.File]::OpenRead($Path)
    try {
      if ($stream.Length -lt 4) { return $false }
      $buffer = New-Object byte[] 4
      [void]$stream.Read($buffer, 0, 4)
      return $buffer[0] -eq 0x50 -and $buffer[1] -eq 0x4B -and (
        ($buffer[2] -eq 0x03 -and $buffer[3] -eq 0x04) -or
        ($buffer[2] -eq 0x05 -and $buffer[3] -eq 0x06) -or
        ($buffer[2] -eq 0x07 -and $buffer[3] -eq 0x08)
      )
    } finally {
      $stream.Dispose()
    }
  } catch {
    return $false
  }
}

function Get-XdtPackagePath([string]$TempDir, [string]$Url) {
  $extension = [IO.Path]::GetExtension(([Uri]$Url).AbsolutePath).ToLowerInvariant()
  if ($extension -eq '.zip') {
    return Join-Path $TempDir 'cc-switch-package.zip'
  }
  if ($extension -eq '.msi') {
    return Join-Path $TempDir 'cc-switch-package.msi'
  }
  return Join-Path $TempDir 'cc-switch-package'
}

function Get-XdtCcSwitchWindowsPackageArch {
  $arch = Get-XdtWindowsArch
  if ($arch -eq 'arm64') {
    Write-XdtLog 'Windows ARM64 detected; using CC Switch Windows x64 package through Windows x64 emulation'
    return 'x64'
  }
  return $arch
}

function Install-XdtCcSwitchForWindows {
  $arch = Get-XdtCcSwitchWindowsPackageArch
  Write-XdtLog "Using Windows $arch package for CC Switch"
  $baseUrl = if ([string]::IsNullOrWhiteSpace($env:XDT_INSTALLER_BASE)) {
    'https://xuedingtoken1.com'
  } else {
    $env:XDT_INSTALLER_BASE.TrimEnd('/')
  }
  $url = if ([string]::IsNullOrWhiteSpace($env:XDT_CCSWITCH_WIN_URL)) {
    "$baseUrl/downloads/cc-switch/CC-Switch-XDT-Windows-$arch.zip"
  } else {
    $env:XDT_CCSWITCH_WIN_URL
  }

  $tmp = Join-Path ([IO.Path]::GetTempPath()) ("xdt-ccswitch-" + [Guid]::NewGuid().ToString())
  New-Item -ItemType Directory -Path $tmp -Force | Out-Null
  try {
    $package = Get-XdtPackagePath $tmp $url
    Write-XdtLog 'Downloading CC Switch enhanced build'
    Invoke-WebRequest -Uri $url -OutFile $package -UseBasicParsing -TimeoutSec 120

    if ($package.ToLowerInvariant().EndsWith('.zip')) {
      if (-not (Test-ZipFile $package)) {
        Fail-Xdt "Downloaded CC Switch package is not a zip file. URL: $url"
      }
      $unpacked = Join-Path $tmp 'unpacked'
      Expand-Archive -Path $package -DestinationPath $unpacked -Force
      $exe = Get-ChildItem -Path $unpacked -Recurse -Filter 'cc-switch.exe' | Select-Object -First 1
      if (-not $exe) {
        Fail-Xdt 'cc-switch.exe not found in downloaded zip package'
      }
      $targetDir = Join-Path $env:LOCALAPPDATA 'Programs\CC Switch'
      $backupDir = Backup-CcSwitchInstall $targetDir $tmp
      Stop-CcSwitchProcesses
      try {
        Remove-Item -Path $targetDir -Recurse -Force -ErrorAction SilentlyContinue
        New-Item -ItemType Directory -Path $targetDir -Force | Out-Null
        Copy-Item -Path (Join-Path $exe.DirectoryName '*') -Destination $targetDir -Recurse -Force
        $installedExe = Join-Path $targetDir 'cc-switch.exe'
        if (-not (Test-Path $installedExe)) {
          Fail-Xdt 'CC Switch copy finished but cc-switch.exe was not found'
        }
        if (Test-KnownBadCcSwitchBuild $installedExe) {
          Fail-Xdt 'Downloaded CC Switch package is a known broken GUI build'
        }
        if (-not (Test-XdtImportSupport $installedExe)) {
          Fail-Xdt 'Downloaded CC Switch package does not support xdt-import'
        }
        Install-CcSwitchShellIntegration $installedExe
      } catch {
        Restore-CcSwitchInstall $backupDir $targetDir
        throw
      }
    } elseif ($package.ToLowerInvariant().EndsWith('.msi')) {
      Write-XdtLog 'Installing CC Switch MSI'
      $process = Start-Process msiexec.exe -ArgumentList @('/i', $package, '/qn', '/norestart') -Wait -PassThru
      if ($process.ExitCode -ne 0) {
        Fail-Xdt "CC Switch MSI installer failed with exit code $($process.ExitCode)"
      }
      $installed = Find-CcSwitch
      if ($installed) {
        if (Test-KnownBadCcSwitchBuild $installed) {
          Fail-Xdt 'Installed CC Switch MSI is a known broken GUI build'
        }
        if (-not (Test-XdtImportSupport $installed)) {
          Fail-Xdt 'Installed CC Switch MSI does not support xdt-import'
        }
        Install-CcSwitchShellIntegration $installed
      }
    } else {
      Fail-Xdt "Unsupported CC Switch package URL. Use a .zip or .msi package: $url"
    }

    Refresh-ProcessPath
  } finally {
    Remove-Item -Path $tmp -Recurse -Force -ErrorAction SilentlyContinue
  }
}

function Ensure-CcSwitch {
  $ccSwitch = Find-CcSwitch
  if ($ccSwitch -and (Test-KnownBadCcSwitchBuild $ccSwitch)) {
    Write-XdtLog 'Existing CC Switch build has a broken GUI package; upgrading'
    try {
      Install-XdtCcSwitchForWindows
    } catch {
      Write-XdtLog "CC Switch upgrade failed; skipping CC Switch launch and continuing with direct Codex configuration: $($_.Exception.Message)"
      return $null
    }
    $ccSwitch = Find-CcSwitch
  }

  if ($ccSwitch -and (Test-XdtImportSupport $ccSwitch)) {
    return $ccSwitch
  }

  if ($ccSwitch) {
    Write-XdtLog 'Existing CC Switch does not support Codex xdt-import; upgrading'
  } else {
    Write-XdtLog 'CC Switch not found; installing'
  }

  try {
    Install-XdtCcSwitchForWindows
  } catch {
    if ($ccSwitch) {
      Write-XdtLog "CC Switch upgrade failed; preserving the existing installation and continuing with direct Codex configuration: $($_.Exception.Message)"
      return $ccSwitch
    }
    Write-XdtLog "CC Switch installation failed; continuing with direct Codex configuration: $($_.Exception.Message)"
    return $null
  }

  $ccSwitch = Find-CcSwitch
  if (-not $ccSwitch) {
    Write-XdtLog 'CC Switch installation completed but binary was not found; continuing with direct Codex configuration'
    return $null
  }
  if (-not (Test-XdtImportSupport $ccSwitch)) {
    Write-XdtLog 'Installed CC Switch still does not support Codex xdt-import; Codex CLI will be configured directly'
  }
  return $ccSwitch
}

Require-Token
$apiUrl = Normalize-Url $env:XDT_API_URL

Ensure-NodeAndCodex
Ensure-VcRuntime
Ensure-WebView2Runtime
$ccSwitch = Ensure-CcSwitch
Install-CcSwitchShellIntegration $ccSwitch

Write-XdtLog 'Importing and switching XueDingToken provider'
$importedWithCcSwitch = $false
if (Test-XdtImportSupport $ccSwitch) {
  try {
    Invoke-XdtImport $ccSwitch $apiUrl $env:XDT_TOKEN
    $importedWithCcSwitch = $true
  } catch {
    Write-XdtLog "CC Switch import failed; falling back to direct Codex configuration: $($_.Exception.Message)"
  }
}

Write-CodexDirectConfig $apiUrl $env:XDT_TOKEN
if ($importedWithCcSwitch) {
  Write-XdtLog 'Codex CLI is configured through CC Switch and normalized to the selected Codex model'
} else {
  Write-XdtLog 'Codex CLI is configured directly'
}

Initialize-CodexSandbox
Start-CcSwitchGui $ccSwitch
Start-CodexTerminal
