param(
  [Parameter(Mandatory)] [ValidateSet('amd64', 'arm64')] [string] $Arch,
  [Parameter(Mandatory)] [string] $Version
)
$ErrorActionPreference = 'Stop'
$dir = 'C:\Users\Public\Те ст\Tetiva'
$exe = "$dir\client.exe"
$key = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\Saveliy YudinTetiva'
$installerA = ".\Tetiva-$Version-windows-$Arch-installer-updatetest.exe"
$hashB = (Get-FileHash ".\client-b-$Arch.exe").Hash
$dataDir = 'C:\tt data'

function Install-A {
  $p = Start-Process $installerA -ArgumentList "/S /D=$dir" -PassThru
  $null = $p.Handle
  $p.WaitForExit()
  if ($p.ExitCode -ne 0) { throw "$installerA exited with $($p.ExitCode)" }
  $icon = (Get-ItemProperty $key).DisplayIcon
  if (-not $icon.StartsWith($dir)) { throw "DisplayIcon '$icon', want it under $dir" }
}

function Get-InstalledVersion { (Get-ItemProperty $key -ErrorAction SilentlyContinue).DisplayVersion }

function Get-Gets([string] $Pattern) {
  @(Get-Content srv.log -ErrorAction SilentlyContinue | Select-String "GET /$Pattern").Count
}

function Get-InstallerGets { Get-Gets 'Tetiva-[^ ]*-installer\.exe' }

function Show-State {
  Get-Content srv.log -ErrorAction SilentlyContinue
  Get-Content app.out, app.err -ErrorAction SilentlyContinue
  Get-ChildItem -Recurse -Force $dataDir -ErrorAction SilentlyContinue | Select-Object FullName, Length | Format-Table -AutoSize | Out-String -Width 300
  Get-Process | Where-Object { $_.Name -like 'client*' -or $_.Name -like 'Tetiva*' } | Select-Object Id, Name, Path | Format-Table -AutoSize | Out-String -Width 300
  Get-ItemProperty $key -ErrorAction SilentlyContinue | Select-Object DisplayVersion, DisplayIcon | Format-List | Out-String
}

Install-A

$env:TETIVA_DATA_DIR = $dataDir
New-Item -ItemType Directory -Force $dataDir | Out-Null
Set-Content "$dataDir\updatetest.json" '{"manifestUrl":"http://127.0.0.1:8765/latest.json","autoApply":true}'

$server = Start-Process python -ArgumentList '-m http.server 8765 --bind 127.0.0.1 --directory served' -RedirectStandardError srv.log -PassThru
try {
  # The test build checks once, 3 s after start; a server still starting would end the cycle there.
  foreach ($i in 1..30) {
    try { $null = Invoke-WebRequest http://127.0.0.1:8765/ -UseBasicParsing -TimeoutSec 2; break } catch { Start-Sleep -Seconds 1 }
  }
  Start-Process $exe -RedirectStandardOutput app.out -RedirectStandardError app.err
  $updated = $false
  foreach ($i in 1..90) {
    Start-Sleep -Seconds 2
    $running = Get-Process client -ErrorAction SilentlyContinue | Where-Object Path -eq $exe
    if ((Get-InstalledVersion) -eq '99.0.0' -and
        (Get-FileHash $exe -ErrorAction SilentlyContinue).Hash -eq $hashB -and
        $running -and
        -not (Test-Path "$dataDir\updates\attempt")) {
      $updated = $true
      break
    }
  }
  if (-not $updated) {
    Show-State
    throw "no update to 99.0.0 within 180 s: DisplayVersion '$(Get-InstalledVersion)'"
  }
  Start-Sleep -Seconds 30
  if ((Get-InstallerGets) -ne 1) { throw "installer downloaded $(Get-InstallerGets) times, want 1" }

  Stop-Process -Name client -Force
  while (Get-Process client -ErrorAction SilentlyContinue) { Start-Sleep -Seconds 1 }
  Install-A
  if ((Get-FileHash $exe).Hash -eq $hashB) { throw 'reinstalling A left B in place' }

  $raw = Get-Content served\latest.json -Raw
  $payload = ($raw | ConvertFrom-Json).payload
  $bytes = [Convert]::FromBase64String($payload)
  $pos = [Array]::IndexOf($bytes, [byte][char]'9')
  $bytes[$pos] = [byte][char]'8'
  Set-Content served\latest.json $raw.Replace($payload, [Convert]::ToBase64String($bytes)) -NoNewline
  $manifestGets = Get-Gets 'latest\.json'

  $app = Start-Process $exe -PassThru
  Start-Sleep -Seconds 60
  $app.Refresh()
  if ($app.HasExited) { throw 'A exited after reading the tampered manifest' }
  if ((Get-InstalledVersion) -ne $Version) { throw "DisplayVersion '$(Get-InstalledVersion)' after the tampered manifest, want $Version" }
  if ((Get-Gets 'latest\.json') -le $manifestGets) { throw 'A never fetched the tampered manifest' }
  if ((Get-InstallerGets) -ne 1) { throw 'the installer was downloaded for the tampered manifest' }
} finally {
  Stop-Process -Name client -Force -ErrorAction SilentlyContinue
  Stop-Process -Id $server.Id -ErrorAction SilentlyContinue
}
