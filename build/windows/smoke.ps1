param(
  [Parameter(Mandatory)] [string] $Version,
  [Parameter(Mandatory)] [ValidateSet('amd64', 'arm64')] [string] $Arch
)
$ErrorActionPreference = 'Stop'
$oldDir = 'C:\Program Files\Saveliy Yudin\Tetiva'
$oldKey = 'HKLM:\Software\Microsoft\Windows\CurrentVersion\Uninstall\Saveliy YudinTetiva'
$newDir = "$env:LOCALAPPDATA\Programs\Tetiva"
$newExe = "$newDir\client.exe"
$newKey = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\Saveliy YudinTetiva'
$installer = ".\Tetiva-$Version-windows-$Arch-installer.exe"

function Invoke-Installer([string] $Path) {
  # WaitForExit, not -Wait: -Wait also waits for a lingering WebView2 updater.
  $p = Start-Process $Path -ArgumentList '/S' -PassThru
  $null = $p.Handle
  $p.WaitForExit()
  if ($p.ExitCode -ne 0) { throw "$Path exited with $($p.ExitCode)" }
}

Invoke-WebRequest "https://s3.twcstorage.ru/ccquota/releases/Tetiva-1.2.0-windows-$Arch-installer.exe" -OutFile old.exe
Invoke-Installer .\old.exe
if (-not (Test-Path "$oldDir\client.exe")) { throw "1.2.0 did not install into $oldDir" }
if (-not (Test-Path $oldKey)) { throw '1.2.0 wrote no uninstall key' }

Invoke-Installer $installer
if (Test-Path $oldDir) { throw 'the 1.2.0 folder is still there' }
if (Test-Path $oldKey) { throw 'the 1.2.0 uninstall key is still there' }
$publisher = (Get-ItemProperty $newKey).Publisher
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

$marker = "$env:APPDATA\client.exe\EBWebView\marker"
Set-Content $marker 'smoke'
Invoke-Installer "$newDir\uninstall.exe"
foreach ($i in 1..60) {
  if (-not (Test-Path $newKey)) { break }
  Start-Sleep -Seconds 1
}
if (Test-Path $newKey) { throw 'the uninstall key is still there after uninstall' }
if (-not (Test-Path $marker)) { throw 'uninstall removed the WebView2 profile' }

Invoke-Installer $installer
if (-not (Test-Path $marker)) { throw 'reinstall removed the WebView2 profile' }
