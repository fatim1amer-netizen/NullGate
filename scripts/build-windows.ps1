$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot
$distDir = Join-Path $repoRoot "dist"
$configDir = Join-Path $distDir "configs"
$exePath = Join-Path $distDir "NullGate.exe"
$configSource = Join-Path $repoRoot "configs\graduation-platform.yaml"

New-Item -ItemType Directory -Path $configDir -Force | Out-Null
Copy-Item -Path $configSource -Destination (Join-Path $configDir "config.yaml") -Force

$oldGOOS = $env:GOOS
$oldGOARCH = $env:GOARCH
try {
    $env:GOOS = "windows"
    $env:GOARCH = "amd64"
    Push-Location $repoRoot
    try {
        go build -trimpath -o $exePath .\cmd\nullgate
        if ($LASTEXITCODE -ne 0) {
            throw "go build failed with exit code $LASTEXITCODE"
        }
    }
    finally {
        Pop-Location
    }
}
finally {
    $env:GOOS = $oldGOOS
    $env:GOARCH = $oldGOARCH
}

if (-not (Test-Path -LiteralPath $exePath)) {
    throw "Build completed without producing $exePath"
}

Write-Host "Built $exePath"
Write-Host "External config: $(Join-Path $configDir 'config.yaml')"
