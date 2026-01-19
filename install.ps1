param(
  [string]$Version = "latest"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$Owner = "shinshin86"
$Repo = "vpeakserver"
$BinName = "vpeakserver.exe"

function Write-Info {
  param ([string]$Message)
  Write-Host $Message -ForegroundColor Cyan
}

function Write-Err {
  param ([string]$Message)
  Write-Host "error: $Message" -ForegroundColor Red
  exit 1
}

function Need-Command {
  param ([string]$Cmd)
  if (-not (Get-Command $Cmd -ErrorAction SilentlyContinue)) {
    Write-Err "$Cmd is required but not found in PATH."
  }
}

Need-Command "curl"
Need-Command "tar"

$OS = $env:OS
if ($OS -ne "Windows_NT") {
  Write-Err "unsupported OS: $OS"
}

$ArchRaw = $env:PROCESSOR_ARCHITECTURE
switch ($ArchRaw) {
  "AMD64" { $Arch = "amd64" }
  "ARM64" { $Arch = "arm64" }
  default { Write-Err "unsupported architecture: $ArchRaw" }
}

if ($Version -eq "latest") {
  $Latest = (curl -fsSL "https://api.github.com/repos/$Owner/$Repo/releases/latest" | ConvertFrom-Json)
  $Tag = $Latest.tag_name
} elseif ($Version -match '^v\d+\.\d+\.\d+$') {
  $Tag = $Version
} else {
  Write-Err "invalid version format: $Version (expected vX.Y.Z or 'latest')"
}

$AssetBase = "vpeakserver_${Tag}_windows_${Arch}"
$Asset = "${AssetBase}.tar.gz"
$Checksums = "checksums.txt"
$BaseUrl = "https://github.com/$Owner/$Repo/releases/download/$Tag"

$TempDir = Join-Path $env:TEMP "vpeakserver-install-$(Get-Date -Format yyyyMMddHHmmss)"
New-Item -ItemType Directory -Path $TempDir | Out-Null

Write-Info "Downloading checksums..."
curl -fsSL "$BaseUrl/$Checksums" -o (Join-Path $TempDir $Checksums)

$ExpectedHash = (Get-Content (Join-Path $TempDir $Checksums) | Where-Object { $_ -match $Asset } | ForEach-Object { ($_ -split ' ')[0] })
if (-not $ExpectedHash) {
  Write-Err "checksum entry for $Asset not found"
}

Write-Info "Downloading $Asset..."
curl -fsSL "$BaseUrl/$Asset" -o (Join-Path $TempDir $Asset)

$ActualHash = (Get-FileHash (Join-Path $TempDir $Asset) -Algorithm SHA256).Hash.ToLower()
if ($ExpectedHash.ToLower() -ne $ActualHash) {
  Write-Err "checksum mismatch: expected $ExpectedHash, got $ActualHash"
}

Write-Info "Extracting..."
tar -xzf (Join-Path $TempDir $Asset) -C $TempDir

$BinaryNameInArchive = "${AssetBase}.exe"
$BinaryPath = Join-Path $TempDir $BinaryNameInArchive
if (-not (Test-Path $BinaryPath)) {
  Write-Err "binary not found after extraction: $BinaryPath"
}

$InstallDir = Join-Path $env:LOCALAPPDATA "Programs\\vpeakserver"
New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null

$TargetPath = Join-Path $InstallDir $BinName
if (Test-Path $TargetPath) {
  $Backup = "$TargetPath.bak-$(Get-Date -Format yyyyMMddHHmmss)"
  Copy-Item $TargetPath $Backup
  Write-Info "Backed up existing binary to $Backup"
}

Copy-Item $BinaryPath $TargetPath -Force
Write-Info "Installed to $TargetPath"

if ($env:Path -notlike "*$InstallDir*") {
  Write-Info "Adding $InstallDir to PATH (user scope)..."
  $CurrentPath = [Environment]::GetEnvironmentVariable("Path", "User")
  if (-not $CurrentPath) {
    $CurrentPath = ""
  }
  if ($CurrentPath -notlike "*$InstallDir*") {
    $NewPath = "$InstallDir;$CurrentPath"
    [Environment]::SetEnvironmentVariable("Path", $NewPath, "User")
    Write-Info "PATH updated. You may need to restart your terminal."
  }
}

Write-Info "Done. Run: vpeakserver --version"
