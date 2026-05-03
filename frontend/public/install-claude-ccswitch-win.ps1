$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'

function Write-XdtLog([string]$Message) {
  Write-Host "[XueDingToken] $Message"
}

function Fail-Xdt([string]$Message) {
  throw "[XueDingToken] $Message"
}

function Require-Token {
  if ([string]::IsNullOrWhiteSpace($env:XDT_TOKEN)) {
    if (-not [string]::IsNullOrWhiteSpace($env:CLAUDE_CLIENT_TOKEN)) {
      $env:XDT_TOKEN = $env:CLAUDE_CLIENT_TOKEN
    } elseif (-not [string]::IsNullOrWhiteSpace($env:CLAUDE_TOKEN)) {
      $env:XDT_TOKEN = $env:CLAUDE_TOKEN
    }
  }
  if ([string]::IsNullOrWhiteSpace($env:XDT_TOKEN)) {
    Fail-Xdt 'Missing XDT_TOKEN'
  }
}

function Normalize-Url([string]$Value) {
  if ([string]::IsNullOrWhiteSpace($Value)) {
    if (-not [string]::IsNullOrWhiteSpace($env:CLAUDE_API_URL)) {
      return $env:CLAUDE_API_URL.Trim().TrimEnd('/')
    }
    return 'https://xuedingtoken.com'
  }
  return $Value.Trim().TrimEnd('/')
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
      Invoke-WebRequest -Uri $url -OutFile $OutFile -UseBasicParsing
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
      return Invoke-RestMethod -Uri $url -UseBasicParsing
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

function Start-ClaudeCodeTerminal {
  if ($env:XDT_SKIP_LAUNCH_CLAUDE -eq '1') {
    return
  }
  if (-not (Test-XdtInteractiveDesktop)) {
    Write-XdtLog 'Claude Code launch skipped because this is not an interactive desktop session'
    return
  }

  Write-XdtLog 'Starting Claude Code in a new terminal'
  $command = 'claude'
  $workingDirectory = [Environment]::GetFolderPath('UserProfile')

  if (Get-Command wt.exe -ErrorAction SilentlyContinue) {
    Start-Process -FilePath 'wt.exe' -WorkingDirectory $workingDirectory -ArgumentList @(
      'new-tab',
      'powershell.exe',
      '-NoExit',
      '-ExecutionPolicy',
      'Bypass',
      '-Command',
      $command
    ) | Out-Null
    return
  }

  Start-Process -FilePath 'powershell.exe' -WorkingDirectory $workingDirectory -ArgumentList @(
    '-NoExit',
    '-ExecutionPolicy',
    'Bypass',
    '-Command',
    $command
  ) | Out-Null
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
    Invoke-WebRequest -Uri "https://aka.ms/vs/17/release/$VcFile" -OutFile $installer -UseBasicParsing
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
    'https://xuedingtoken.com'
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

function Configure-NpmMirror {
  $registry = if ([string]::IsNullOrWhiteSpace($env:XDT_NPM_REGISTRY)) {
    'https://registry.npmmirror.com'
  } else {
    $env:XDT_NPM_REGISTRY.TrimEnd('/')
  }
  $env:npm_config_registry = $registry
  try {
    & npm config set registry $registry *> $null
  } catch {}
  return $registry
}

function Install-ClaudeCode {
  $registry = Configure-NpmMirror
  Write-XdtLog 'Installing Claude Code with npm'
  & npm install -g '@anthropic-ai/claude-code' --registry $registry
  if ($LASTEXITCODE -eq 0) {
    Refresh-ProcessPath
    return
  }

  Write-XdtLog 'npm mirror failed; retrying with npmjs.org'
  & npm install -g '@anthropic-ai/claude-code' --registry 'https://registry.npmjs.org'
  if ($LASTEXITCODE -ne 0) {
    Fail-Xdt "Claude Code npm installation failed with exit code $LASTEXITCODE"
  }
  Refresh-ProcessPath
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
    Install-NodeLts
  }

  if (-not (Test-NodeVersion)) {
    Fail-Xdt 'Node.js 18+ is required. Install Node.js, then rerun this command.'
  }

  Configure-NpmMirror | Out-Null

  if (Get-Command claude -ErrorAction SilentlyContinue) {
    Write-XdtLog 'Claude Code detected'
    return
  }

  Install-ClaudeCode
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
    $process = Start-Process -FilePath $CcSwitch -ArgumentList @('xdt-import', '--help') -Wait -PassThru -WindowStyle Hidden
    return $process.ExitCode -eq 0
  } catch {
    return $false
  }
}

function Test-KnownBadCcSwitchBuild([string]$CcSwitch) {
  if ([string]::IsNullOrWhiteSpace($CcSwitch) -or -not (Test-Path $CcSwitch)) {
    return $false
  }

  $knownBadHashes = @(
    # Built without Tauri production resource embedding; GUI loads http://localhost:3000.
    '26CD1B76957BBFC68773CD1CD86FF67D9A45C6B0DF5A139FF79BE716BB8A7A25',
    '75944F638DD118AA15DC26ECA6E537CA8E07A049EFC4018E751554B732EB6A2D'
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

function Invoke-XdtImport([string]$CcSwitch, [string]$ApiUrl, [string]$Token) {
  $args = @(
    'xdt-import',
    '--provider-id', 'xuedingtoken',
    '--name', 'XueDingToken',
    '--app', 'claude',
    '--endpoint', $ApiUrl,
    '--api-key', $Token,
    '--homepage', 'https://xuedingtoken.com',
    '--icon', 'claude',
    '--switch'
  )

  $process = Start-Process -FilePath $CcSwitch -ArgumentList $args -Wait -PassThru -WindowStyle Hidden
  if ($process.ExitCode -ne 0) {
    Fail-Xdt "cc-switch xdt-import failed with exit code $($process.ExitCode)"
  }

  $settingsPath = Join-Path $env:USERPROFILE '.claude\settings.json'
  if (-not (Test-Path $settingsPath)) {
    Fail-Xdt "cc-switch xdt-import finished but Claude settings were not created: $settingsPath"
  }

  try {
    $settings = Get-Content -Raw -Path $settingsPath | ConvertFrom-Json
    if ($settings.env.ANTHROPIC_AUTH_TOKEN -ne $Token -or $settings.env.ANTHROPIC_BASE_URL -ne $ApiUrl) {
      Fail-Xdt 'cc-switch xdt-import finished but Claude settings do not match the requested provider'
    }
  } catch {
    Fail-Xdt "Unable to verify Claude settings after cc-switch import: $($_.Exception.Message)"
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
    'https://xuedingtoken.com'
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
    Invoke-WebRequest -Uri $url -OutFile $package -UseBasicParsing

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
      New-Item -ItemType Directory -Path $targetDir -Force | Out-Null
      Stop-CcSwitchProcesses
      Copy-Item -Path (Join-Path $exe.DirectoryName '*') -Destination $targetDir -Recurse -Force
      Install-CcSwitchShellIntegration (Join-Path $targetDir 'cc-switch.exe')
    } elseif ($package.ToLowerInvariant().EndsWith('.msi')) {
      Write-XdtLog 'Installing CC Switch MSI'
      $process = Start-Process msiexec.exe -ArgumentList @('/i', $package, '/qn', '/norestart') -Wait -PassThru
      if ($process.ExitCode -ne 0) {
        Fail-Xdt "CC Switch MSI installer failed with exit code $($process.ExitCode)"
      }
      $installed = Find-CcSwitch
      if ($installed) {
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
    Install-XdtCcSwitchForWindows
    $ccSwitch = Find-CcSwitch
  }

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
Ensure-VcRuntime
Ensure-WebView2Runtime
$ccSwitch = Ensure-CcSwitch
Install-CcSwitchShellIntegration $ccSwitch

Write-XdtLog 'Importing and switching XueDingToken provider'
Invoke-XdtImport $ccSwitch $apiUrl $env:XDT_TOKEN

Write-XdtLog 'Claude Code is configured through CC Switch'
Start-CcSwitchGui $ccSwitch
Start-ClaudeCodeTerminal
