[CmdletBinding()]
param(
    [Parameter(Position = 0, ValueFromRemainingArguments = $true)]
    [string[]]$WailsArguments = @()
)

$ErrorActionPreference = 'Stop'

$expectedVersion = '2.10.2'
$projectRoot = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$projectTool = Join-Path $projectRoot "build\tools\wails-v$expectedVersion.exe"
$projectGoCache = Join-Path $projectRoot 'build\cache\go-build'
[System.IO.Directory]::CreateDirectory($projectGoCache) | Out-Null
$env:GOCACHE = $projectGoCache

function Test-WailsVersion {
    param([string]$Path)

    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) {
        return $false
    }
    $output = & $Path version 2>&1 | Out-String
    return $LASTEXITCODE -eq 0 -and $output -match "(?<![0-9])v?$([regex]::Escape($expectedVersion))(?![0-9])"
}

$candidates = [System.Collections.Generic.List[string]]::new()
$candidates.Add($projectTool)
$installed = Get-Command wails -CommandType Application -ErrorAction SilentlyContinue
if ($installed) {
    $candidates.Add($installed.Source)
}
$goPath = (& go env GOPATH).Trim()
if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($goPath)) {
    throw 'Could not read GOPATH while checking for a local Wails CLI.'
}
$candidates.Add((Join-Path $goPath 'bin\wails.exe'))

$wails = $null
foreach ($candidate in $candidates | Select-Object -Unique) {
    if (Test-WailsVersion $candidate) {
        $wails = $candidate
        break
    }
}

if (-not $wails) {
    $moduleCache = (& go env GOMODCACHE).Trim()
    if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($moduleCache)) {
        throw 'Could not read GOMODCACHE while checking for cached Wails sources.'
    }
    $cachedModule = Join-Path $moduleCache "github.com\wailsapp\wails\v2@v$expectedVersion"
    if (-not (Test-Path -LiteralPath $cachedModule -PathType Container)) {
        throw "Wails v$expectedVersion is genuinely absent locally: $cachedModule was not found."
    }

    [System.IO.Directory]::CreateDirectory((Split-Path -Parent $projectTool)) | Out-Null
    $previousProxy = $env:GOPROXY
    Push-Location $cachedModule
    try {
        $env:GOPROXY = 'off'
        & go build -o $projectTool ./cmd/wails
        if ($LASTEXITCODE -ne 0) {
            throw "Cached Wails v$expectedVersion sources exist, but the offline CLI build failed. Check for incomplete cached Go dependencies."
        }
    } finally {
        $env:GOPROXY = $previousProxy
        Pop-Location
    }
    if (-not (Test-WailsVersion $projectTool)) {
        throw "The offline-built Wails CLI does not report v$expectedVersion."
    }
    $wails = $projectTool
}

& $wails @WailsArguments
exit $LASTEXITCODE
