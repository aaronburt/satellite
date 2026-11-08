param(
    [string]$Version
)

$ErrorActionPreference = "Stop"
Push-Location (Join-Path $PSScriptRoot "..")
try {
    if (-not $Version) {
        $config = Get-Content "internal/config/config.go" -Raw
        if ($config -notmatch 'var Version = "([^"]+)"') {
            throw "Could not read the default application version."
        }
        $Version = $Matches[1]
    }
    if ($Version -notmatch '^\d+\.\d+(\.\d+){0,2}$') {
        throw "Version must contain two to four numeric components."
    }
    foreach ($part in $Version.Split(".")) {
        if ([uint64]$part -gt 65535) {
            throw "Windows version components must be at most 65535."
        }
    }
    go run github.com/tc-hib/go-winres@v0.3.3 make --in winres/winres.json --out resource --arch amd64,arm64,386 --file-version $Version --product-version $Version
    if ($LASTEXITCODE -ne 0) {
        throw "Windows resource generation failed."
    }
} finally {
    Pop-Location
}
