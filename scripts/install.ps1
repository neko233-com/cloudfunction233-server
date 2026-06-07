$ErrorActionPreference = "Stop"

$Repo = if ($env:CF233_REPO) { $env:CF233_REPO } else { "neko233/cloudfunction233-server" }
$InstallDir = if ($env:CF233_HOME) { $env:CF233_HOME } else { Join-Path $env:ProgramData "cloudfunction233-server" }
$Bin = Join-Path $InstallDir "cloudfunction233-server.exe"

$arch = switch ($env:PROCESSOR_ARCHITECTURE) {
  "ARM64" { "arm64" }
  default { "amd64" }
}

$release = Invoke-RestMethod "https://api.github.com/repos/$Repo/releases/latest"
$asset = $release.assets | Where-Object { $_.name -match "windows_$arch" } | Select-Object -First 1
if (-not $asset) {
  throw "release asset not found for windows_$arch"
}

$tmp = Join-Path $env:TEMP ("cf233-install-" + [guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $tmp | Out-Null
try {
  $archive = Join-Path $tmp $asset.name
  Invoke-WebRequest -Uri $asset.browser_download_url -OutFile $archive
  if ($asset.name.EndsWith(".zip")) {
    Expand-Archive -Path $archive -DestinationPath $tmp -Force
  } else {
    Copy-Item $archive (Join-Path $tmp "cloudfunction233-server.exe")
  }
  New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
  Copy-Item (Join-Path $tmp "cloudfunction233-server.exe") $Bin -Force
  & $Bin init-config | Out-Null
  Write-Host "installed: $Bin"
  Write-Host "start:     $Bin start"
  Write-Host "status:    $Bin status"
  Write-Host "autostart: $Bin autostart enable"
} finally {
  Remove-Item -LiteralPath $tmp -Recurse -Force -ErrorAction SilentlyContinue
}
