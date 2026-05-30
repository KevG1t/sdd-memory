Write-Host "Actualizando sdd-memory..." -ForegroundColor Cyan
git pull origin master
pnpm install
Write-Host "¡Actualización completada exitosamente!" -ForegroundColor Green
