# Forward host port 8443 to the Android emulator (run after each emulator boot).
$adb = "$env:LOCALAPPDATA\Android\Sdk\platform-tools\adb.exe"
if (-not (Test-Path $adb)) {
    Write-Error "adb not found at $adb"
    exit 1
}
& $adb reverse tcp:8443 tcp:8443
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
Write-Host "OK: emulator http://127.0.0.1:8443 -> host localhost:8443"
Write-Host "In Remote Agent, set Server URL to: http://127.0.0.1:8443"
