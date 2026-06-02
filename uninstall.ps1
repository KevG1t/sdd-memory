# SDD Memory Uninstallation Script for Windows
# Usage: irm https://raw.githubusercontent.com/KevG1t/sdd-memory/master/uninstall.ps1 | iex

param(
    [switch]$Force,
    [switch]$KeepData
)

$ErrorActionPreference = "Stop"

# Colors for output
$Colors = @{
    Info = "Cyan"
    Success = "Green" 
    Warning = "Yellow"
    Error = "Red"
}

function Write-ColorOutput {
    param(
        [string]$Message,
        [string]$Type = "Info"
    )
    
    $color = $Colors[$Type]
    Write-Host "[$Type] $Message" -ForegroundColor $color
}

function Confirm-Action {
    param(
        [string]$Message,
        [bool]$Default = $false
    )
    
    if ($Force) {
        return $true
    }
    
    $prompt = if ($Default) { "$Message [Y/n]" } else { "$Message [y/N]" }
    
    do {
        $response = Read-Host $prompt
        if ([string]::IsNullOrWhiteSpace($response)) {
            $response = if ($Default) { "y" } else { "n" }
        }
        $response = $response.ToLower()
    } while ($response -notin @("y", "yes", "n", "no"))
    
    return $response -in @("y", "yes")
}

function Find-Binary {
    $binaryName = "sdd-memory.exe"
    $possiblePaths = @(
        "$env:USERPROFILE\.local\bin\$binaryName"
        "$env:ProgramFiles\SDD-Memory\$binaryName"
        "$env:LOCALAPPDATA\Programs\SDD-Memory\$binaryName"
    )
    
    # Also check PATH
    try {
        $pathLocation = (Get-Command $binaryName -ErrorAction SilentlyContinue).Source
        if ($pathLocation) {
            $possiblePaths += $pathLocation
        }
    }
    catch {
        # Command not found in PATH
    }
    
    foreach ($path in $possiblePaths) {
        if (Test-Path $path) {
            return $path
        }
    }
    
    return $null
}

function Remove-Binary {
    param([string]$BinaryPath)
    
    if (-not $BinaryPath) {
        Write-ColorOutput "SDD Memory binary not found in common locations" "Warning"
        return $true
    }
    
    Write-ColorOutput "Found binary at: $BinaryPath" "Info"
    
    if (Confirm-Action "Remove the SDD Memory binary?") {
        try {
            Remove-Item $BinaryPath -Force
            Write-ColorOutput "Binary removed: $BinaryPath" "Success"
            return $true
        }
        catch {
            Write-ColorOutput "Failed to remove binary: $_" "Error"
            Write-Host "You may need to run as Administrator or close any running instances"
            return $false
        }
    }
    else {
        Write-ColorOutput "Binary left untouched" "Info"
        return $true
    }
}

function Remove-DataDirectory {
    if ($KeepData) {
        Write-ColorOutput "Skipping data removal (--KeepData specified)" "Info"
        return
    }
    
    $dataDir = "$env:USERPROFILE\.sdd-memory"
    
    if (-not (Test-Path $dataDir)) {
        Write-ColorOutput "No data directory found at $dataDir" "Info"
        return
    }
    
    try {
        $dirSize = (Get-ChildItem $dataDir -Recurse | Measure-Object -Property Length -Sum).Sum
        $sizeText = if ($dirSize -gt 1MB) { "$([math]::Round($dirSize/1MB, 2)) MB" } 
                   elseif ($dirSize -gt 1KB) { "$([math]::Round($dirSize/1KB, 2)) KB" }
                   else { "$dirSize bytes" }
        
        Write-ColorOutput "Found data directory: $dataDir ($sizeText)" "Info"
    }
    catch {
        Write-ColorOutput "Found data directory: $dataDir (unknown size)" "Info"
    }
    
    Write-Host ""
    Write-Host "This directory contains:" -ForegroundColor Yellow
    Write-Host "  • Your local database (local.db)" -ForegroundColor Yellow
    Write-Host "  • All stored memories and observations" -ForegroundColor Yellow
    Write-Host "  • Configuration files" -ForegroundColor Yellow
    Write-Host ""
    
    if (Confirm-Action "⚠️  Remove all SDD Memory data? This cannot be undone!") {
        try {
            Remove-Item $dataDir -Recurse -Force
            Write-ColorOutput "Data directory removed: $dataDir" "Success"
        }
        catch {
            Write-ColorOutput "Failed to remove data directory: $_" "Error"
        }
    }
    else {
        Write-ColorOutput "Data directory preserved at: $dataDir" "Info"
    }
}

function Update-EnvironmentPath {
    $installPath = "$env:USERPROFILE\.local\bin"
    $userPath = [Environment]::GetEnvironmentVariable("PATH", "User")
    
    if ($userPath -and $userPath.Contains($installPath)) {
        if (Confirm-Action "Remove $installPath from user PATH?") {
            try {
                $newPath = ($userPath -split ';' | Where-Object { $_ -ne $installPath }) -join ';'
                $newPath = $newPath.TrimStart(';').TrimEnd(';')
                
                [Environment]::SetEnvironmentVariable("PATH", $newPath, "User")
                Write-ColorOutput "Removed $installPath from user PATH" "Success"
                Write-ColorOutput "Please restart PowerShell for PATH changes to take effect" "Warning"
            }
            catch {
                Write-ColorOutput "Failed to update PATH: $_" "Error"
            }
        }
    }
}

function Test-Uninstallation {
    Write-ColorOutput "Verifying uninstallation..." "Info"
    
    try {
        $command = Get-Command "sdd-memory" -ErrorAction SilentlyContinue
        if ($command) {
            Write-ColorOutput "SDD Memory is still available in PATH" "Warning"
            Write-Host "  Found at: $($command.Source)"
            Write-Host "  You may need to remove it manually or restart PowerShell"
        }
        else {
            Write-ColorOutput "SDD Memory successfully removed from system" "Success"
        }
    }
    catch {
        Write-ColorOutput "SDD Memory successfully removed from system" "Success"
    }
}

function Main {
    Write-Host @"
╭─────────────────────────────────────╮
│     SDD Memory Uninstaller (Win)   │
╰─────────────────────────────────────╯
"@ -ForegroundColor Cyan

    Write-Host ""
    Write-ColorOutput "Starting SDD Memory uninstallation..."
    Write-Host ""
    
    # Find and remove binary
    $binaryPath = Find-Binary
    $success = Remove-Binary -BinaryPath $binaryPath
    Write-Host ""
    
    # Remove data directory
    Remove-DataDirectory
    Write-Host ""
    
    # Clean up PATH
    Update-EnvironmentPath
    Write-Host ""
    
    # Verify uninstallation
    Test-Uninstallation
    Write-Host ""
    
    Write-Host "🧹 Uninstallation process complete!" -ForegroundColor Green
    Write-Host ""
    Write-Host "Thank you for using SDD Memory!"
    Write-Host "If you want to reinstall later:"
    Write-Host "  irm https://raw.githubusercontent.com/KevG1t/sdd-memory/master/install.ps1 | iex"
}

# Run uninstaller
Main