# SDD Memory (Lite)

Un motor de persistencia *Local-First* ultrarrápido y minimalista, diseñado específicamente para dotar de memoria a largo plazo a agentes de IA (como Cursor, Claude Code, etc.), basado en la filosofía de [Engram](https://github.com/KevG1t/engram).

## Características Arquitectónicas

- **Un solo ejecutable (Go):** Compilado nativamente. Sin dependencias de Node.js, npm, o Python.
- **Local-First (SQLite):** Persistencia inmediata usando `modernc.org/sqlite` (sin requerir compiladores CGO, funciona en cualquier plataforma).
- **Búsqueda FTS5:** Motor de búsqueda full-text nativo de alta performance para recuperar contexto exacto.
- **TUI Elegante:** Interfaz de terminal inmersiva y fluida (basada en `bubbletea` y `lipgloss`) para explorar tu memoria.
- **Protocolo MCP Completo:** Exposición limpia de 15 herramientas a través de `stdio` compatibles con el estándar Model Context Protocol.

---

## 🚀 Instalación

Si tienes Go instalado, la forma más fácil de instalar es usando `go install`:

```bash
go install github.com/KevG1t/sdd-memory/cmd/sdd-memory@v1.0.0
```

*(Esto colocará el binario en tu carpeta `GOPATH/bin`, asegúrate de tenerla en tu variable de entorno PATH).*

### Actualización
Para actualizar a la última versión, simplemente vuelve a ejecutar el comando de instalación:
```bash
go install github.com/KevG1t/sdd-memory/cmd/sdd-memory@v1.0.0
```

### Desinstalación
Si deseas eliminar por completo SDD Memory Lite de tu sistema:
1. Elimina el binario: `rm $(go env GOPATH)/bin/sdd-memory` (en Windows: `del $env:GOPATH\bin\sdd-memory.exe`).
2. Elimina tu base de datos y memoria local: `rm -rf ~/.sdd-memory`.

---

## 🎮 Uso y Comandos

Todos los datos (la base de datos SQLite FTS5) se guardan por defecto de forma centralizada y segura en tu directorio de usuario: `~/.sdd-memory/local.db`.

### 1. Explorador Visual (TUI)
Para navegar por tus memorias, buscar observaciones o revisar métricas, simplemente ejecuta el comando sin argumentos en tu terminal. Esto abrirá una aplicación a pantalla completa con navegación por pestañas:

```bash
sdd-memory
```

*Controles de la TUI:*
- `Tab` / `Shift+Tab` o Flechas Izquierda/Derecha: Cambiar de pestaña (Dashboard, Search, Observations, Sessions, Setup).
- `j` / `k` o Flechas Arriba/Abajo: Navegar por los listados.
- `Enter`: Buscar (en la pestaña de búsqueda).
- `q` o `Esc`: Salir.

### 2. Integración con Agentes (Modo MCP)
Este es el comando que tu IDE o agente LLM (como Cursor) debe ejecutar por detrás para enchufarse al motor de memoria usando el protocolo MCP a través de `stdio`:

```bash
sdd-memory mcp
```

#### ¿Cómo configurarlo en Cursor?
1. Ve a `Cursor Settings` > `Features` > `MCP`.
2. Añade un nuevo servidor:
   - **Type**: `command`
   - **Name**: `sdd-memory`
   - **Command**: `sdd-memory mcp` (o la ruta absoluta si no lo tienes en tu PATH).

---

## 🛠️ Herramientas MCP Expuestas

El modo servidor expone **15 herramientas** vitales para que los agentes administren la información sin necesidad de intervención manual:

- **Búsqueda y Recuperación:**
  - `mem_search`: Busca conocimiento histórico usando el motor FTS5.
  - `mem_context`: Recupera activamente las sesiones y observaciones más recientes para poner al agente en contexto al inicio de una tarea.
  - `mem_get_observation`: Recupera el detalle de una memoria específica por su ID.
- **Gestión de Sesiones:**
  - `mem_session_start` / `mem_session_end`: Las sesiones se guardan en su propia tabla para hacer seguimiento del trabajo en curso.
  - `mem_session_summary`: Guarda resúmenes detallados al finalizar una iteración.
- **Escritura y Modificación:**
  - `mem_save`: Guarda o actualiza una observación manual.
  - `mem_capture_passive`: Captura aprendizajes de contexto pasivamente.
  - `mem_update`: Corrige detalles específicos de una observación existente en SQL.
  - `mem_save_prompt`: Almacena templates y prompts útiles.
- **Utilidades del Sistema:**
  - `mem_current_project`: Detecta y retorna automáticamente tu directorio actual de trabajo.
  - `mem_doctor`: Diagnósticos de estado de la base de datos.
  - `mem_suggest_topic_key`: Utilidad para sanitizar títulos de observaciones.
  - `mem_judge` / `mem_compare`: Aliases de compatibilidad para flujos lógicos avanzados.
