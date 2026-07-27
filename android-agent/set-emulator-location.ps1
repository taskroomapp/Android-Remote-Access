# Inject a mock GPS fix into the running Android emulator, then the panel's
# "Get current location" can return coordinates.
# Usage:
#   .\set-emulator-location.ps1
#   .\set-emulator-location.ps1 -Lat 37.7749 -Lng -122.4194
param(
    [double]$Lat = 37.4220,
    [double]$Lng = -122.0841,
    [string]$Serial = "emulator-5554"
)

$ErrorActionPreference = "Stop"
$adb = $null
$candidates = @(
    "$env:LOCALAPPDATA\Android\Sdk\platform-tools\adb.exe",
    "$env:ANDROID_HOME\platform-tools\adb.exe",
    "$env:ANDROID_SDK_ROOT\platform-tools\adb.exe"
)
foreach ($c in $candidates) {
    if ($c -and (Test-Path $c)) { $adb = $c; break }
}
if (-not $adb) {
    $adb = "adb"
}

Write-Host "Setting emulator location to lat=$Lat lng=$Lng on $Serial ..."

# Method 1: emulator console (lon lat). Often broken on Windows if console port is busy.
& $adb -s $Serial emu geo fix $Lng $Lat 2>$null
if ($LASTEXITCODE -eq 0) {
    Write-Host "OK via adb emu geo fix. In the web panel, click Get current location."
    exit 0
}

# Method 2: location service (API 30+)
& $adb -s $Serial shell cmd location set-location -- "$Lat" "$Lng" 2>$null
if ($LASTEXITCODE -eq 0) {
    Write-Host "OK via cmd location. In the web panel, click Get current location."
    exit 0
}

Write-Host @"
Could not inject GPS over adb (emulator console port may be blocked).

Do this instead:
  1. Open the emulator window → … (Extended controls) → Location
  2. Enter lat=$Lat lng=$Lng → Send / Set location
  3. In the web panel, click Get current location

Or restart the emulator and retry this script.
"@
exit 1
