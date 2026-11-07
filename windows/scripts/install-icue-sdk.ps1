param(
    [string]$Destination = (Join-Path ([Environment]::GetFolderPath('ApplicationData')) 'satellite\sdk')
)

$ErrorActionPreference = 'Stop'
if (-not [Environment]::Is64BitProcess) { throw 'Run this script in 64-bit PowerShell.' }
$archive = Join-Path ([IO.Path]::GetTempPath()) ('satellite-icue-' + [Guid]::NewGuid().ToString('N') + '.zip')
try {
    Invoke-WebRequest -Uri 'https://github.com/CorsairOfficial/cue-sdk/releases/download/v4.0.84/iCUESDK_4.0.84.zip' -OutFile $archive
    if ((Get-FileHash -LiteralPath $archive -Algorithm SHA256).Hash -ne 'EB4414CF505145F3E507DC839AD587BF7CA684D64BE3D9A834F416A136736D5D') {
        throw 'The iCUE SDK download did not match its expected SHA256 hash.'
    }
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    $zip = [IO.Compression.ZipFile]::OpenRead($archive)
    try {
        $entry = $zip.GetEntry('iCUESDK/redist/x64/iCUESDK.x64_2019.dll')
        if ($null -eq $entry) { throw 'The SDK DLL is missing from the archive.' }
        New-Item -ItemType Directory -Path $Destination -Force | Out-Null
        $target = Join-Path $Destination 'iCUESDK.x64_2019.dll'
        [IO.Compression.ZipFileExtensions]::ExtractToFile($entry, $target, $true)
    } finally {
        $zip.Dispose()
    }
    Write-Output "Installed iCUE SDK 4.0.84 in $Destination. Restart Satellite, then enable iCUE RGB Colours in Settings."
} finally {
    if (Test-Path -LiteralPath $archive) { Remove-Item -LiteralPath $archive }
}
