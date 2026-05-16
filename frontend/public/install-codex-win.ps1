$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'

Remove-Item Env:CODEX_API_URL,Env:XDT_API_URL -ErrorAction SilentlyContinue
Invoke-Expression (Invoke-RestMethod -Uri 'https://xuedingtoken.com/install-codex-win-bootstrap.ps1' -UseBasicParsing -TimeoutSec 60)
