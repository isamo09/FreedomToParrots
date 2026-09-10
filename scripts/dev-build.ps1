# Local dev build (Windows): compiles the olcrtc tunnel core (from a
# sibling ..\source checkout if present, otherwise a fresh clone of
# openlibrecommunity/olcrtc), embeds it, and builds .\cmd\fzp for the host
# platform (or $env:GOOS/$env:GOARCH if set). Mirrors what the release
# workflow does per-target in CI - use this to test a real, working build
# locally instead of the dev placeholder core.
$ErrorActionPreference = "Stop"
Set-Location (Split-Path $PSScriptRoot -Parent)
$root = Get-Location

$goos = if ($env:GOOS) { $env:GOOS } else { (go env GOHOSTOS) }
$goarch = if ($env:GOARCH) { $env:GOARCH } else { (go env GOHOSTARCH) }
$coreSrc = if ($env:CORE_SRC) { $env:CORE_SRC } else { "..\source" }

if (-not (Test-Path "$coreSrc\cmd\olcrtc")) {
    Write-Host "no olcrtc core source at $coreSrc - cloning openlibrecommunity/olcrtc"
    $coreSrc = Join-Path $env:TEMP "olcrtc-core-$(Get-Random)"
    git clone --depth 1 https://github.com/openlibrecommunity/olcrtc $coreSrc
}

$ext = ""
$dest = "$root\internal\corebin\bin\core"
if ($goos -eq "windows") {
    $ext = ".exe"
    $dest = "$root\internal\corebin\bin\core.exe"
}

Write-Host "building core ($goos/$goarch) from $coreSrc -> $dest"
if (Test-Path $dest) { Remove-Item $dest -Force }
Push-Location $coreSrc
try {
    $env:GOOS = $goos; $env:GOARCH = $goarch; $env:CGO_ENABLED = "0"
    go build -trimpath -ldflags "-s -w" -o $dest ./cmd/olcrtc
    if ($LASTEXITCODE -ne 0) { throw "core build failed" }
} finally {
    Pop-Location
}

New-Item -ItemType Directory -Force -Path "$root\dist" | Out-Null
$out = "$root\dist\FreedomToParrots-$goos-$goarch$ext"
Write-Host "building fzp -> $out"
$env:GOOS = $goos; $env:GOARCH = $goarch; $env:CGO_ENABLED = "0"
go build -trimpath -ldflags "-s -w -X main.version=dev" -o $out ./cmd/fzp
if ($LASTEXITCODE -ne 0) { throw "fzp build failed" }

Write-Host "`ndone: $out"
Write-Host "`nNOTE: internal\corebin\bin\core$ext now holds a real binary - don't"
Write-Host "git add it. Restore the placeholder before committing:"
Write-Host "  'freedomtoparrots-dev-placeholder' | Set-Content -NoNewline internal\corebin\bin\core$ext; Add-Content internal\corebin\bin\core$ext ''"
