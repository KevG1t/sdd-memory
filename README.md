# SDD Memory (Lite)

Un motor de persistencia *Local-First* ultrarrápido y minimalista, diseñado específicamente para dotar de memoria a largo plazo a agentes de IA (como Cursor, Claude Code, etc.).

## Características Arquitectónicas

- **Un solo ejecutable (Go):** Compilado nativamente. Sin dependencias de Node.js, npm, o Python.
- **Local-First (SQLite):** Persistencia inmediata usando `modernc.org/sqlite` (sin requerir compiladores CGO, funciona en cualquier plataforma).
- **Búsqueda FTS5:** Motor de búsqueda full-text nativo de alta performance para recuperar contexto exacto.
- **TUI Elegante:** Interfaz de terminal inmersiva y fluida (basada en `bubbletea` y `lipgloss`) para explorar tu memoria.
- **Protocolo MCP Completo:** Exposición limpia de 17 herramientas a través de `stdio` compatibles con el estándar Model Context Protocol.
- **Motor de datos sdd-memory:** Deduplicación de 3 ramas (revisión por `topic_key`, dedup por hash dentro de una ventana temporal, inserción), borrado lógico (*soft-delete*) y tabla propia de prompts. Contrato de campos compatible con sdd-memory (`sync_id`, `tool_name`, `duplicate_count`, etc.).

---

## 🚀 Instalación

Elige el método que prefieras:

### 1. Instalación Rápida (Recomendada)

**Linux/macOS:**
```bash
curl -sSL https://raw.githubusercontent.com/KevG1t/sdd-memory/master/install.sh | bash
```

**Windows (PowerShell):**
```powershell
irm https://raw.githubusercontent.com/KevG1t/sdd-memory/master/install.ps1 | iex
```

### 2. Instalación con npm/npx

**Ejecución directa (sin instalación):**
```bash
npx sdd-memory-kevg1t
```

**Instalación global:**
```bash
npm install -g sdd-memory-kevg1t
```

**Nota:** Los paquetes npm se publican automáticamente con cada release. Puedes usar `npx` para siempre ejecutar la última versión sin necesidad de instalación.

### 3. Descarga Manual

Descarga el binario para tu plataforma desde la [página de releases](https://github.com/KevG1t/sdd-memory/releases/latest):

- **Linux**: `sdd-memory-linux`
- **macOS**: `sdd-memory-macos` 
- **Windows**: `sdd-memory-windows.exe`

Luego colócalo en tu PATH.

### 4. Instalación con Go

Si tienes Go instalado:

```bash
go install github.com/KevG1t/sdd-memory/cmd/sdd-memory@latest
```

*(Esto colocará el binario en tu carpeta `GOPATH/bin`, asegúrate de tenerla en tu variable de entorno PATH).*

### Actualización
- **Instalación rápida**: Vuelve a ejecutar el script de instalación
- **npm**: `npm update -g sdd-memory-kevg1t`
- **Go install**: `go install github.com/KevG1t/sdd-memory/cmd/sdd-memory@latest`

### Desinstalación

Tienes varias opciones para desinstalar SDD Memory:

#### 1. Desinstalación Automática (Recomendada)

**Linux/macOS:**
```bash
curl -sSL https://raw.githubusercontent.com/KevG1t/sdd-memory/master/uninstall.sh | bash
```

**Windows (PowerShell):**
```powershell
irm https://raw.githubusercontent.com/KevG1t/sdd-memory/master/uninstall.ps1 | iex
```

#### 2. Desinstalación Manual

Si prefieres hacerlo manualmente:
1. **Elimina el binario:**
   ```bash
   rm ~/.local/bin/sdd-memory        # Linux/macOS
   # O desde donde lo hayas instalado
   ```
2. **Elimina los datos (opcional):**
   ```bash
   rm -rf ~/.sdd-memory              # Tu base de datos local
   ```

#### 3. Solo eliminar datos
Si solo quieres limpiar la base de datos pero mantener el programa:
```bash
rm -rf ~/.sdd-memory
```

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

#### Cómo configurarlo en Cursor?

1. **Localiza tu archivo de configuración MCP:**
   - **Linux/macOS**: `~/.cursor/mcp_config.json`
   - **Windows**: `%APPDATA%\Cursor\mcp_config.json`

2. **Añade sdd-memory a la configuración:**
   ```json
   {
     "mcpServers": {
       "sdd-memory": {
         "command": "sdd-memory",
         "args": ["mcp"]
       }
     }
   }
   ```

3. **Alternativamente, usando la UI de Cursor:**
   - Ve a `Cursor Settings` > `Features` > `MCP`
   - Añade un nuevo servidor:
     - **Type**: `command`
     - **Name**: `sdd-memory`
     - **Command**: `sdd-memory mcp` (o la ruta absoluta si no lo tienes en tu PATH)

#### Configuración para otros IDEs

**Claude Code:**
Añade a tu archivo de configuración MCP:
```json
{
  "sdd-memory": {
    "command": "sdd-memory",
    "args": ["mcp"]
  }
}
```

**Otros agentes MCP:**
La configuración es similar, consulta la documentación específica de tu IDE para la ubicación exacta del archivo de configuración MCP.

---

## ☁️ Sincronización en la Nube (Cloud Sync)

`sdd-memory` cuenta con un motor propio para sincronizar tu memoria local con un servidor central. Esto es súper útil si querés tener tu memoria de IA respaldada de forma segura o compartida entre varias computadoras.

El sistema es muy sencillo de entender y se divide en dos partes: **El Servidor** (la bóveda central) y **El Cliente** (tu máquina local de todos los días).

### 1. Levantar el Servidor (Tu Nube Privada)
Si querés levantar tu propia nube en un VPS, necesitás una base de datos PostgreSQL. (Podés usar el archivo `docker-compose.yml` que viene en el proyecto para levantarla con un solo comando).

Una vez que tengas la base de datos, levantá el servidor inyectando estas credenciales por seguridad:

```bash
# Variables de conexión a PostgreSQL
export PGHOST="localhost"
export PGPORT="5432"
export PGUSER="sdd_user"
export PGPASSWORD="sdd_password"
export PGDATABASE="sdd_cloud"

# Tus credenciales de seguridad de SDD Memory
export SDD_CLOUD_SECRET="tu-contraseña-maestra"
export SDD_ALLOWED_PROJECTS="KevG1t/mi-proyecto"

# Levantar el router HTTP
sdd-memory cloud serve
```

### 2. Usar el Cliente (Tu máquina local)
En tu computadora de desarrollo, tu base SQLite local siempre es la dueña de la verdad. Para engancharla con tu nuevo servidor, seguí estos tres pasos súper fáciles:

1. **Configurar la conexión**:
   Decile a tu compu dónde vive tu nube y cuál es la clave de acceso.
   ```bash
   export SDD_CLOUD_SECRET="tu-contraseña-maestra"
   sdd-memory cloud config --server http://tuservidor.com:18080
   ```

2. **Autorizar tu proyecto (Enroll)**:
   Para que no sincronices proyectos por accidente, tenés que enrolar el directorio actual en la nube.
   ```bash
   sdd-memory cloud enroll KevG1t/mi-proyecto
   ```

3. **La Sincronización (Sync)**:
   Cuando termines de trabajar y quieras hacer *backup* o traer cambios de tu otra máquina, simplemente ejecutá:
   ```bash
   sdd-memory cloud sync KevG1t/mi-proyecto
   ```
   *¿Qué hace esto por detrás?* Primero baja inteligentemente los datos nuevos desde el servidor y luego sube tus cambios locales usando un sistema de reintentos automáticos por si se te corta el WiFi a la mitad de la carga.

---

## 🛠️ Herramientas MCP Expuestas

El modo servidor expone **17 herramientas** vitales para que los agentes administren la información sin necesidad de intervención manual. Para la referencia completa de parámetros y respuestas, ver [`docs/mcp-tools.md`](docs/mcp-tools.md):

- **Búsqueda y Recuperación:**
  - `mem_search`: Busca conocimiento histórico usando el motor FTS5.
  - `mem_context`: Recupera activamente las sesiones y observaciones más recientes para poner al agente en contexto al inicio de una tarea.
  - `mem_get_observation`: Recupera el detalle de una memoria específica por su ID.
- **Gestión de Sesiones:**
  - `mem_session_start` / `mem_session_end`: Las sesiones se guardan en su propia tabla para hacer seguimiento del trabajo en curso.
  - `mem_session_summary`: Guarda resúmenes detallados al finalizar una iteración.
- **Escritura y Modificación:**
  - `mem_save`: Guarda o actualiza una observación. Aplica las 3 ramas de dedup (revisión por `topic_key`, dedup por hash, inserción).
  - `mem_capture_passive`: Captura aprendizajes de contexto pasivamente.
  - `mem_update`: Corrige detalles específicos de una observación existente en SQL.
  - `mem_delete`: Borrado lógico (*soft-delete*) de una observación; con `hard: true` la elimina físicamente.
  - `mem_save_prompt`: Registra un prompt del usuario en la tabla `user_prompts`.
- **Utilidades del Sistema:**
  - `mem_current_project`: Detecta y retorna automáticamente tu directorio actual de trabajo.
  - `mem_stats`: Métricas de la memoria (sesiones, observaciones, prompts, proyectos).
  - `mem_doctor`: Diagnósticos de estado de la base de datos.
  - `mem_suggest_topic_key`: Utilidad para sanitizar títulos de observaciones.
  - `mem_judge` / `mem_compare`: Aliases de compatibilidad para flujos lógicos avanzados.
