# SDD Memory Installation Script for Windows
# Usage: irm https://raw.githubusercontent.com/KevG1t/sdd-memory/main/install.ps1 | iex

param(
    [string]$InstallPath = "$env:USERPROFILE\.local\bin"
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

function Get-Architecture {
    $arch = $env:PROCESSOR_ARCHITECTURE
    switch ($arch) {
        "AMD64" { return "amd64" }
        "ARM64" { return "arm64" }
        default { 
            Write-ColorOutput "Architecture $arch may not be supported, trying amd64" "Warning"
            return "amd64"
        }
    }
}

function Get-LatestVersion {
    Write-ColorOutput "Fetching latest release information..."
    
    try {
        $latestUrl = "https://api.github.com/repos/KevG1t/sdd-memory/releases/latest"
        $response = Invoke-RestMethod -Uri $latestUrl
        return $response.tag_name
    }
    catch {
        Write-ColorOutput "Failed to get latest version: $_" "Error"
        exit 1
    }
}

function Install-Binary {
    param(
        [string]$Version,
        [string]$Architecture
    )
    
    $binaryName = "sdd-memory.exe"
    $downloadName = "sdd-memory-windows.exe"
    $downloadUrl = "https://github.com/KevG1t/sdd-memory/releases/download/$Version/$downloadName"
    
    # Create install directory
    if (!(Test-Path $InstallPath)) {
        New-Item -ItemType Directory -Path $InstallPath -Force | Out-Null
        Write-ColorOutput "Created directory: $InstallPath"
    }
    
    $tempFile = "$env:TEMP\$binaryName"
    $finalPath = Join-Path $InstallPath $binaryName
    
    Write-ColorOutput "Downloading from: $downloadUrl"
    
    try {
        # Download binary
        Invoke-WebRequest -Uri $downloadUrl -OutFile $tempFile
        
        # Verify download
        if (!(Test-Path $tempFile)) {
            throw "Download failed - file not found"
        }
        
        # Move to install directory
        Move-Item $tempFile $finalPath -Force
        
        Write-ColorOutput "Binary installed to: $finalPath" "Success"
        return $finalPath
    }
    catch {
        Write-ColorOutput "Download failed: $_" "Error"
        exit 1
    }
}

function Add-ToPath {
    param([string]$Path)
    
    $userPath = [Environment]::GetEnvironmentVariable("PATH", "User")
    
    if ($userPath -notlike "*$Path*") {
        Write-ColorOutput "$Path is not in your PATH, adding it..."
        
        $newPath = if ($userPath) { "$userPath;$Path" } else { $Path }
        [Environment]::SetEnvironmentVariable("PATH", $newPath, "User")
        
        # Update current session PATH
        $env:PATH = "$env:PATH;$Path"
        
        Write-ColorOutput "Added $Path to user PATH" "Success"
        Write-ColorOutput "Please restart PowerShell for PATH changes to take effect" "Warning"
    }
}

function Test-Installation {
    param([string]$BinaryPath)
    
    Write-ColorOutput "Testing installation..."
    
    try {
        $result = & $BinaryPath mcp --help 2>$null
        if ($LASTEXITCODE -eq 0) {
            Write-ColorOutput "SDD Memory is working correctly" "Success"
        }
        else {
            Write-ColorOutput "Binary installed but may not be working correctly" "Warning"
        }
    }
    catch {
        Write-ColorOutput "Could not test binary: $_" "Warning"
    }
}

function Main {
    Write-Host @"
╭─────────────────────────────────────╮
│      SDD Memory Installer (Win)    │
╰─────────────────────────────────────╯
"@ -ForegroundColor Cyan

    Write-Host ""
    
    $architecture = Get-Architecture
    Write-ColorOutput "Detected architecture: $architecture"
    
    $version = Get-LatestVersion
    Write-ColorOutput "Latest version: $version"
    
    $binaryPath = Install-Binary -Version $version -Architecture $architecture
    Add-ToPath -Path $InstallPath
    Test-Installation -BinaryPath $binaryPath
    
    Write-Host ""
    Write-Host "🎉 Installation complete!" -ForegroundColor Green
    Write-Host ""
    Write-Host "Next steps:"
    Write-Host "1. Restart PowerShell (for PATH changes)"
    Write-Host "2. Run: sdd-memory"
    Write-Host "3. For IDE integration: sdd-memory mcp"
    Write-Host ""
    Write-Host "Documentation: https://github.com/KevG1t/sdd-memory"
}

# Run installer
Main