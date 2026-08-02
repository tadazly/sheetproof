[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string]$SshHost
)

$ErrorActionPreference = 'Stop'

$projectRoot = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$siteOutput = Join-Path $projectRoot 'site\dist\client'
$indexFile = Join-Path $siteOutput 'index.html'
if (-not (Test-Path -LiteralPath $indexFile -PathType Leaf)) {
    throw 'The static website has not been built. Run npm test in site/ first.'
}

$domain = 'sheetproof.luyilabs.com'
$stamp = Get-Date -Format 'yyyyMMddHHmmss'
$archiveName = "$domain-$stamp.tar.gz"
$localArchive = Join-Path ([System.IO.Path]::GetTempPath()) $archiveName
$remoteArchive = "/tmp/$archiveName"

try {
    & tar -czf $localArchive -C $siteOutput .
    if ($LASTEXITCODE -ne 0) {
        throw 'Could not package the static website.'
    }

    & scp $localArchive "${SshHost}:$remoteArchive"
    if ($LASTEXITCODE -ne 0) {
        throw 'Could not upload the website archive.'
    }

    $remoteCommand = @(
        'set -eu',
        "archive='$remoteArchive'",
        "target='/var/www/$domain'",
        "staging='/var/www/.$domain.staging'",
        "backup='/var/www/.$domain.previous'",
        'sudo rm -rf "$staging"',
        'sudo mkdir -p "$staging"',
        'sudo tar -xzf "$archive" -C "$staging"',
        'test -f "$staging/index.html"',
        'sudo chown -R caddy:caddy "$staging"',
        'sudo find "$staging" -type d -exec chmod 755 {} +',
        'sudo find "$staging" -type f -exec chmod 644 {} +',
        'sudo rm -rf "$backup"',
        'had_target=0',
        'if [ -d "$target" ]; then sudo mv "$target" "$backup"; had_target=1; fi',
        'sudo mv "$staging" "$target"',
        'rm -f "$archive"',
        ('if ! curl -fsS --resolve ''{0}:443:127.0.0.1'' ''https://{0}/'' >/dev/null; then sudo rm -rf "$target"; if [ "$had_target" = 1 ]; then sudo mv "$backup" "$target"; fi; exit 1; fi' -f $domain)
    ) -join '; '

    & ssh $SshHost $remoteCommand
    if ($LASTEXITCODE -ne 0) {
        throw 'The remote deployment or origin health check failed.'
    }
} finally {
    Remove-Item -LiteralPath $localArchive -Force -ErrorAction SilentlyContinue
}

Write-Host "Deployed https://$domain/"
