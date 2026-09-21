param(
  [Parameter(Mandatory)] [string] $Version,
  [Parameter(Mandatory)] [ValidateSet('amd64', 'arm64')] [string] $Arch
)
$ErrorActionPreference = 'Stop'
$oldDir = 'C:\Program Files\Saveliy Ludin\Tetiva'
$newExe = 'C:\Program Files\Saveliy Yudin\Tetiva\client.exe'
$uninstall = 'HKLM:\Software\Microsoft\Windows\CurrentVersion\Uninstall'

function Invoke-Installer([string] $Path) {
  # WaitForExit, not -Wait: -Wait also waits for a lingering WebView2 updater.
  $p = Start-Process $Path -ArgumentList '/S' -PassThru
  $null = $p.Handle
  $p.WaitForExit()
  if ($p.ExitCode -ne 0) { throw "$Path exited with $($p.ExitCode)" }
}

Invoke-WebRequest "https://s3.twcstorage.ru/ccquota/releases/Tetiva-1.1.1-windows-$Arch-installer.exe" -OutFile old.exe
Invoke-Installer .\old.exe
if (-not (Test-Path "$oldDir\client.exe")) { throw "1.1.1 did not install into $oldDir" }
if (-not (Test-Path "$uninstall\Saveliy LudinTetiva")) { throw '1.1.1 wrote no uninstall key' }

Invoke-Installer ".\Tetiva-$Version-windows-$Arch-installer.exe"
if (Test-Path $oldDir) { throw 'the 1.1.1 folder is still there' }
if (Test-Path 'C:\Program Files\Saveliy Ludin') { throw 'the old publisher folder is still there' }
if (Test-Path "$uninstall\Saveliy LudinTetiva") { throw 'the 1.1.1 uninstall key is still there' }
$publisher = (Get-ItemProperty "$uninstall\Saveliy YudinTetiva").Publisher
if ($publisher -ne 'Saveliy Yudin') { throw "Publisher '$publisher'" }
$fileVersion = (Get-Item $newExe).VersionInfo.FileVersionRaw.ToString(3)
if ($fileVersion -ne $Version) { throw "file version $fileVersion, want $Version" }

$app = Start-Process $newExe -PassThru -RedirectStandardError stderr.txt
# Holding the handle keeps ExitCode readable after the process is gone.
$null = $app.Handle
$webview = $null
foreach ($i in 1..30) {
  Start-Sleep -Seconds 2
  $app.Refresh()
  if ($app.HasExited) { break }
  $webview = Get-CimInstance Win32_Process -Filter "Name = 'msedgewebview2.exe'" |
    Where-Object ParentProcessId -eq $app.Id
  if ($app.MainWindowTitle -and $webview) { break }
}
$failures = @()
if ($app.HasExited) {
  $failures += "client.exe exited with code $($app.ExitCode)"
} elseif ($app.MainWindowTitle -ne "Tetiva $Version") {
  $failures += "window title '$($app.MainWindowTitle)', want 'Tetiva $Version'"
}
if (-not $webview) { $failures += 'no msedgewebview2.exe under client.exe' }
if (-not (Test-Path "$env:APPDATA\client.exe\EBWebView")) { $failures += 'no WebView2 profile' }
if ($failures) {
  Get-Content stderr.txt -ErrorAction SilentlyContinue
  throw ($failures -join '; ')
}
Stop-Process -Id $app.Id
