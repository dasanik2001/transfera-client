<#
.SYNOPSIS
    One-line installer for Transfera CLI on Windows.

.DESCRIPTION
    Downloads the latest transfera.exe from GitHub Releases and installs it
    to %LOCALAPPDATA%\Programs\Transfera, then adds it to your user PATH.

.EXAMPLE
    irm https://raw.githubusercontent.com/dasanik2001/transfera-client/main/install.ps1 | iex
#>

$ErrorActionPreference = "Stop"

$repo = "dasanik2001/transfera-client"
$binaryName = "transfera-windows-amd64.exe"

# --- Colors ---
function Write-Step($msg)  { Write-Host "  [*] $msg" -ForegroundColor Cyan }
function Write-Ok($msg)    { Write-Host "  [OK] $msg" -ForegroundColor Green }
function Write-Err($msg)   { Write-Host "  [!] $msg" -ForegroundColor Red }

Write-Host ""
Write-Host "  ============================================" -ForegroundColor Cyan
Write-Host "    Transfera CLI — Windows Installer" -ForegroundColor Cyan
Write-Host "  ============================================" -ForegroundColor Cyan
Write-Host ""

# 1. Find the latest release
Write-Step "Finding latest release..."
try {
    $release = Invoke-RestMethod -Uri "https://api.github.com/repos/$repo/releases/latest" -Headers @{ "User-Agent" = "Transfera-Installer" }
    $tag = $release.tag_name
    Write-Ok "Latest version: $tag"
} catch {
    Write-Err "Could not fetch latest release from GitHub."
    Write-Err "Check your internet connection or visit: https://github.com/$repo/releases"
    exit 1
}

# 2. Find the Windows binary asset
$asset = $release.assets | Where-Object { $_.name -eq $binaryName }
if (-not $asset) {
    Write-Err "Binary '$binaryName' not found in release $tag."
    Write-Err "Available assets: $($release.assets.name -join ', ')"
    exit 1
}

$downloadUrl = $asset.browser_download_url

# 3. Set up install directory
$installDir = Join-Path $env:LOCALAPPDATA "Programs\Transfera"
$destPath = Join-Path $installDir "transfera.exe"

if (-not (Test-Path $installDir)) {
    New-Item -ItemType Directory -Path $installDir -Force | Out-Null
}

# 4. Download the binary
Write-Step "Downloading transfera $tag..."
try {
    Invoke-WebRequest -Uri $downloadUrl -OutFile $destPath -UseBasicParsing
    Write-Ok "Downloaded to $destPath"
} catch {
    Write-Err "Download failed: $_"
    exit 1
}

# 5. Unblock the file (remove Mark of the Web)
Unblock-File -Path $destPath -ErrorAction SilentlyContinue

# 6. Add to PATH if not already there
$currentPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($currentPath -split ";" | Where-Object { $_.Trim() -ieq $installDir }) {
    Write-Ok "Already in PATH."
} else {
    Write-Step "Adding to PATH..."
    $newPath = "$currentPath;$installDir"
    [Environment]::SetEnvironmentVariable("Path", $newPath, "User")

    # Broadcast WM_SETTINGCHANGE so new terminals pick up the change
    if (-not ("Win32.NativeMethods" -as [type])) {
        Add-Type -Namespace Win32 -Name NativeMethods -MemberDefinition @"
[DllImport("user32.dll", SetLastError = true, CharSet = CharSet.Auto)]
public static extern IntPtr SendMessageTimeout(
    IntPtr hWnd, uint Msg, UIntPtr wParam, string lParam,
    uint fuFlags, uint uTimeout, out UIntPtr lpdwResult);
"@
    }
    $result = [UIntPtr]::Zero
    [Win32.NativeMethods]::SendMessageTimeout(
        [IntPtr]0xFFFF, 0x001A, [UIntPtr]::Zero, "Environment",
        0x0002, 5000, [ref]$result
    ) | Out-Null

    Write-Ok "Added to PATH."
}

# 7. Create Desktop shortcut
Write-Step "Creating desktop shortcut..."
try {
    $desktopDir = [Environment]::GetFolderPath("Desktop")
    $shortcutPath = Join-Path $desktopDir "Transfera.lnk"
    $ws = New-Object -ComObject WScript.Shell
    $shortcut = $ws.CreateShortcut($shortcutPath)
    $shortcut.TargetPath = $destPath
    $shortcut.WorkingDirectory = $env:USERPROFILE
    $shortcut.Description = "Transfera CLI - P2P File Sharing"
    $shortcut.Save()
    Write-Ok "Desktop shortcut created."
} catch {
    # Non-fatal — skip if shortcut creation fails
}

# 8. Done!
Write-Host ""
Write-Host "  ============================================" -ForegroundColor Green
Write-Host "    Transfera $tag installed successfully!" -ForegroundColor Green
Write-Host "  ============================================" -ForegroundColor Green
Write-Host ""
Write-Host "  Open a NEW terminal and type:" -ForegroundColor Yellow
Write-Host "    transfera" -ForegroundColor White
Write-Host ""
