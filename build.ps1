$ErrorActionPreference = "Stop"

Write-Host "Building SDD Memory Lite..."
go build -o bin/sdd-memory.exe ./cmd/sdd-memory

if ($LASTEXITCODE -eq 0) {
    Write-Host "Build successful! Executable is at bin/sdd-memory.exe" -ForegroundColor Green
} else {
    Write-Host "Build failed." -ForegroundColor Red
}
