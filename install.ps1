param([string]$Version = "latest")

$ErrorActionPreference = "Stop"
$Owner = "shinshin86"
$Repo = "vpeakserver"
$BinName = "vpeakserver.exe"
$BinDir = if ($env:BIN_DIR) { $env:BIN_DIR } else { "$env:LOCALAPPDATA\Programs\vpeakserver" }

function Write-Err($Message) {
  Write-Host "ERROR: $Message" -ForegroundColor Red
  exit 1
}

function Write-Info($Message) {
  Write-Host $Message -ForegroundColor Cyan
}

function Invoke-Download {
  param(
    [Parameter(Mandatory = $true)][string]$Uri,
    [Parameter(Mandatory = $true)][string]$OutFile
  )

  $params = @{ Uri = $Uri; OutFile = $OutFile; MaximumRedirection = 5 }
  if ($PSVersionTable.PSVersion.Major -lt 6) {
    $params.UseBasicParsing = $true
  }

  $maxRetries = 3
  for ($i = 1; $i -le $maxRetries; $i++) {
    try {
      Invoke-WebRequest @params
      return
    } catch {
      if ($i -eq $maxRetries) { throw }
      Start-Sleep -Seconds (2 * $i)
    }
  }
}

if ($env:OS -ne "Windows_NT") {
  Write-Err "Unsupported OS: $($env:OS)"
}

$Arch = switch ($env:PROCESSOR_ARCHITECTURE) {
  "AMD64" { "amd64" }
  "ARM64" { "arm64" }
  default { Write-Err "Unsupported architecture: $($env:PROCESSOR_ARCHITECTURE)" }
}

if ($Version -ne "latest" -and $Version -notmatch '^v\d+\.\d+\.\d+$') {
  Write-Err "Invalid version format. Use vX.Y.Z or 'latest'."
}

if ($Version -eq "latest") {
  $latestParams = @{ Uri = "https://api.github.com/repos/$Owner/$Repo/releases/latest" }
  if ($PSVersionTable.PSVersion.Major -lt 6) {
    $latestParams.UseBasicParsing = $true
  }
  $Version = (Invoke-RestMethod @latestParams).tag_name
}

if (-not $Version) {
  Write-Err "Failed to determine the latest version."
}

$Asset = "vpeakserver_${Version}_windows_${Arch}.zip"
$BaseUrl = "https://github.com/$Owner/$Repo/releases/download/$Version"

$TempDir = Join-Path ([System.IO.Path]::GetTempPath()) ("vpeakserver-install-" + [System.IO.Path]::GetRandomFileName())
New-Item -ItemType Directory -Force -Path $TempDir | Out-Null

Write-Info "Downloading checksums..."
$ChecksumsPath = Join-Path $TempDir "checksums.txt"
Invoke-Download -Uri "$BaseUrl/checksums.txt" -OutFile $ChecksumsPath

$ExpectedChecksum = (Select-String -Path $ChecksumsPath -Pattern " $Asset$").Line.Split(' ')[0]
if (-not $ExpectedChecksum) {
  Write-Err "Checksum not found for $Asset in checksums.txt"
}

Write-Info "Downloading $Asset..."
$AssetPath = Join-Path $TempDir $Asset
Invoke-Download -Uri "$BaseUrl/$Asset" -OutFile $AssetPath

$ActualChecksum = (Get-FileHash $AssetPath -Algorithm SHA256).Hash
if ($ExpectedChecksum.ToLower() -ne $ActualChecksum.ToLower()) {
  Write-Err "Checksum mismatch: expected $ExpectedChecksum, got $ActualChecksum"
}

Expand-Archive -Path $AssetPath -DestinationPath $TempDir -Force

$BinPath = Join-Path $TempDir $BinName
if (-not (Test-Path $BinPath)) {
  Write-Err "Binary not found after extraction."
}

New-Item -ItemType Directory -Force -Path $BinDir | Out-Null

$TargetPath = Join-Path $BinDir $BinName
if (Test-Path $TargetPath) {
  Copy-Item $TargetPath "$TargetPath.bak" -Force
  Write-Info "Existing binary backed up to $TargetPath.bak"
}

Copy-Item -Force $BinPath $TargetPath
Write-Info "Installed $BinName to $TargetPath"

$PathValue = [Environment]::GetEnvironmentVariable("Path", "User")
if ($PathValue -notlike "*$BinDir*") {
  [Environment]::SetEnvironmentVariable("Path", "$BinDir;$PathValue", "User")
  Write-Info "Added $BinDir to your user PATH. Restart your terminal to apply."
}

Write-Info "Done! Run 'vpeakserver --version' to verify."
