# SDD Memory (Lite)

Un motor de persistencia *Local-First* ultrarrápido y minimalista, diseñado específicamente para dotar de memoria a largo plazo a agentes de IA (como `sdd-orchestrator`), basado en la arquitectura de sincronización por *chunks* de [Engram](https://github.com/KevG1t/engram).

## Características Arquitectónicas

- **Local-First (SQLite):** Persistencia inmediata y ultrarrápida usando `better-sqlite3`.
- **Búsqueda FTS5:** Motor de búsqueda full-text nativo de alta performance para recuperar el contexto exacto.
- **Protocolo MCP Nativo:** Exposición limpia de herramientas a través de `stdio` para agentes inteligentes.
- **SyncManager Libre de Conflictos:** Detección de deltas locales, compresión de *chunks* vía `zlib` (`.json.gz`), hashes criptográficos y resolución de colisiones por `revision_count`.
- **Cero Grasita:** Sin dependencias pesadas de terceros, UIs complejas, ni evaluadores LLM semánticos que rompan la compatibilidad en Windows. Pura matemática de persistencia.

## Instalación Global

Puedes instalar el binario globalmente de forma directa desde GitHub (requiere Node.js instalado). Como el repositorio es público, la forma más limpia y libre de errores de SSH es usar la URL HTTPS:

```bash
pnpm add -g https://github.com/KevG1t/sdd-memory.git
```
*(También compatible con `npm install -g https://github.com/KevG1t/sdd-memory.git`)*

### Actualización
Para obtener la última versión fresca, simplemente vuelve a ejecutar el comando de instalación o ejecuta:
```bash
pnpm update -g sdd-memory
```

## Uso y Comandos

Una vez instalado, el binario `sdd-memory` estará disponible globalmente en tu terminal. Todos los datos (base de datos y chunks) se guardarán por defecto de forma segura en una carpeta oculta `.sdd-memory` dentro del directorio donde ejecutes el comando.

### 1. Interfaz de Usuario (TUI)
Para inicializar la base de datos a mano, revisar el estado de sincronización o forzar el *push* a la nube, usa el menú interactivo:
```bash
sdd-memory tui
```

### 2. Modo Servidor (MCP)
Este es el comando que tu Orquestador o LLM debe ejecutar para enchufarse al motor de memoria a través de `stdio`:
```bash
sdd-memory mcp
```

## Herramientas MCP Expuestas

El modo servidor expone 3 herramientas vitales para los agentes:

1. **`mem_save`**: Guarda o actualiza una observación. Si el `topic_key` (proyecto, scope, topic) ya existe, incrementa el `revision_count` automáticamente. Si es nuevo, genera un ID criptográfico seguro.
2. **`mem_search`**: Busca conocimiento histórico usando el motor indexado FTS5.
3. **`mem_get_observation`**: Recupera el detalle de una memoria específica por su ID.

## Sincronización en la Nube (Próximos Pasos)

El motor local ya es capaz de empaquetar, comprimir y realizar un **Push** de las memorias nuevas hacia un servidor externo. Para configurar este *endpoint*, crea un archivo `.env` o exporta las siguientes variables en tu entorno:

```env
CLOUD_ENDPOINT=http://mi-servidor-postgres.com/api/sync
CLOUD_API_KEY=tu-secreto-super-seguro
```

> **Nota de Arquitectura:** El backend que recibe estos *chunks* y los impacta en PostgreSQL debe ser desarrollado e implementado en una aplicación de servidor separada.
