#!/bin/bash
echo "Actualizando sdd-memory..."
git pull origin master
pnpm install
# pnpm run build (si corresponde compilar algo extra en el futuro)
echo "¡Actualización completada exitosamente!"
