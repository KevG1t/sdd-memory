# @kevg1t/sdd-memory

[![npm version](https://badge.fury.io/js/@kevg1t%2Fsdd-memory.svg)](https://www.npmjs.com/package/@kevg1t/sdd-memory)

Un motor de persistencia *Local-First* ultrarrápido y minimalista, diseñado específicamente para dotar de memoria a largo plazo a agentes de IA (como Cursor, Claude Code, etc.).

## Instalación

### Ejecución directa (recomendada)
```bash
npx @kevg1t/sdd-memory
```

### Instalación global
```bash
npm install -g @kevg1t/sdd-memory
```

## Uso

### Interfaz TUI
```bash
npx @kevg1t/sdd-memory
```

### Servidor MCP para agentes AI
```bash
npx @kevg1t/sdd-memory mcp
```

### Sincronización en la nube
```bash
npx @kevg1t/sdd-memory cloud serve
npx @kevg1t/sdd-memory cloud sync proyecto-ejemplo
```

## Configuración en IDEs

### Cursor
Añade a tu `~/.cursor/mcp_config.json`:
```json
{
  "mcpServers": {
    "sdd-memory": {
      "command": "npx",
      "args": ["@kevg1t/sdd-memory", "mcp"]
    }
  }
}
```

### Claude Code
```json
{
  "sdd-memory": {
    "command": "npx",
    "args": ["@kevg1t/sdd-memory", "mcp"]
  }
}
```

## Características

- **Un solo ejecutable (Go)**: Sin dependencias de runtime
- **Local-First (SQLite)**: Persistencia inmediata con FTS5
- **Búsqueda Semántica**: Motor de búsqueda full-text de alta performance
- **TUI Elegante**: Interfaz de terminal inmersiva
- **Protocolo MCP**: 15+ herramientas para agentes AI
- **Sincronización Cloud**: Respaldo y sync entre dispositivos

## Documentación Completa

Para documentación completa, ejemplos y configuración avanzada, visita:
[https://github.com/KevG1t/sdd-memory](https://github.com/KevG1t/sdd-memory)

## Métodos de Instalación Alternativos

Si prefieres no usar npm, también puedes instalar con:

**Script de instalación (Linux/macOS):**
```bash
curl -sSL https://raw.githubusercontent.com/KevG1t/sdd-memory/master/install.sh | bash
```

**PowerShell (Windows):**
```powershell
irm https://raw.githubusercontent.com/KevG1t/sdd-memory/master/install.ps1 | iex
```

**Go install:**
```bash
go install github.com/KevG1t/sdd-memory/cmd/sdd-memory@latest
```