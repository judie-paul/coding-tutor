<#
.SYNOPSIS
    Builds, tests and runs the AI Programming Tutor.

.DESCRIPTION
    With no switches the script checks formatting, runs go vet, runs all tests
    and builds the server binary into .\bin.

.EXAMPLE
    .\build.ps1                 # check, test and build
    .\build.ps1 -Run            # build then start the server
    .\build.ps1 -Migrate        # build then apply database migrations only
    .\build.ps1 -SkipTests      # build without running tests
    .\build.ps1 -Release        # smaller, production-ready binary
    .\build.ps1 -Release -OS linux -Arch amd64   # cross-compile for a Linux server
    .\build.ps1 -Clean          # remove build output
#>
[CmdletBinding()]
param(
    [switch]$Run,
    [switch]$Migrate,
    [switch]$SkipTests,
    [switch]$Release,
    [switch]$Clean,
    [string]$OS = "",
    [string]$Arch = ""
)

$ErrorActionPreference = "Stop"
Set-Location -Path $PSScriptRoot

function Step([string]$Message) {
    Write-Host "==> $Message" -ForegroundColor Cyan
}

function Invoke-Checked([string]$Command, [string[]]$Arguments) {
    & $Command @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "'$Command $($Arguments -join ' ')' failed with exit code $LASTEXITCODE"
    }
}

if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    throw "Go is not installed or not on PATH. Install it from https://go.dev/dl/"
}

$binDir = Join-Path $PSScriptRoot "bin"

if ($Clean) {
    Step "Removing build output"
    if (Test-Path $binDir) { Remove-Item -Recurse -Force $binDir }
    Write-Host "Clean complete." -ForegroundColor Green
    return
}

Step "Checking formatting"
$goDirs = @("cmd", "internal", "migrations", "web") | Where-Object { Test-Path $_ }
$unformatted = & gofmt -l @goDirs
if ($LASTEXITCODE -ne 0) { throw "gofmt failed" }
if ($unformatted) {
    Write-Host "These files need 'gofmt -w':" -ForegroundColor Yellow
    $unformatted | ForEach-Object { Write-Host "  $_" }
    throw "Formatting check failed"
}

Step "Running go vet"
Invoke-Checked "go" @("vet", "./...")

if (-not $SkipTests) {
    Step "Running tests"
    Invoke-Checked "go" @("test", "./...")
}

$targetOS = if ($OS) { $OS } else { (& go env GOOS) }
$exeName = if ($targetOS -eq "windows") { "tutor.exe" } else { "tutor" }
$output = Join-Path $binDir $exeName

$buildArgs = @("build", "-o", $output)
if ($Release) { $buildArgs += @("-trimpath", "-ldflags", "-s -w") }
$buildArgs += "./cmd/web"

Step "Building $output"
$oldOS = $env:GOOS; $oldArch = $env:GOARCH
try {
    if ($OS) { $env:GOOS = $OS }
    if ($Arch) { $env:GOARCH = $Arch }
    Invoke-Checked "go" $buildArgs
}
finally {
    $env:GOOS = $oldOS; $env:GOARCH = $oldArch
}
Write-Host "Build complete: $output" -ForegroundColor Green

if ($Migrate) {
    Step "Applying database migrations"
    Invoke-Checked $output @("-migrate")
}

if ($Run) {
    Step "Starting server (Ctrl+C to stop)"
    & $output
}
