import { Server } from "@modelcontextprotocol/sdk/server/index.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import {
  CallToolRequestSchema,
  ListToolsRequestSchema,
  Tool,
} from "@modelcontextprotocol/sdk/types.js";
import { LocalStore } from '../core/store/LocalStore.js';
import * as crypto from 'crypto';

const TOOLS: Tool[] = [
  {
    name: "mem_save",
    description: "Save or update a memory observation",
    inputSchema: {
      type: "object",
      properties: {
        project: { type: "string" },
        scope: { type: "string" },
        topic: { type: "string" },
        content: { type: "string" }
      },
      required: ["project", "scope", "topic", "content"]
    }
  },
  {
    name: "mem_search",
    description: "Search memory observations",
    inputSchema: {
      type: "object",
      properties: {
        query: { type: "string" },
        project: { type: "string" }
      },
      required: ["query"]
    }
  },
  {
    name: "mem_get_observation",
    description: "Get a specific memory observation by ID",
    inputSchema: {
      type: "object",
      properties: {
        id: { type: "string" }
      },
      required: ["id"]
    }
  }
];

export async function runMcpServer(dbPath: string) {
  const store = new LocalStore(dbPath);
  await store.init();

  const server = new Server({
    name: "sdd-memory",
    version: "1.0.0"
  }, {
    capabilities: {
      tools: {}
    }
  });

  server.setRequestHandler(ListToolsRequestSchema, async () => {
    return { tools: TOOLS };
  });

  server.setRequestHandler(CallToolRequestSchema, async (request) => {
    try {
      if (request.params.name === "mem_save") {
        const { project, scope, topic, content } = request.params.arguments as any;
        const existing = await store.findByTopicKey(project, scope, topic);
        let id: string;
        let revisionCount = 1;
        let createdAt = new Date().toISOString();

        if (existing) {
          id = existing.id;
          revisionCount = (existing.revisionCount || 1) + 1;
          createdAt = existing.createdAt;
        } else {
          id = `obs-${crypto.randomBytes(8).toString('hex')}`;
        }
        
        await store.saveObservation({
          id,
          project,
          scope,
          topic,
          content,
          revisionCount,
          createdAt,
          updatedAt: new Date().toISOString()
        });

        return {
          content: [{ type: "text", text: `Saved observation ${id}` }]
        };
      }
      
      if (request.params.name === "mem_search") {
        const { query, project } = request.params.arguments as any;
        const results = await store.searchObservations(query, project);
        return {
          content: [{ type: "text", text: JSON.stringify(results, null, 2) }]
        };
      }

      if (request.params.name === "mem_get_observation") {
        const { id } = request.params.arguments as any;
        const result = await store.getObservation(id);
        if (!result) {
          return { content: [{ type: "text", text: "Not found" }], isError: true };
        }
        return {
          content: [{ type: "text", text: JSON.stringify(result, null, 2) }]
        };
      }

      return {
        content: [{ type: "text", text: `Unknown tool: ${request.params.name}` }],
        isError: true
      };
    } catch (e: any) {
      return {
        content: [{ type: "text", text: `Error: ${e.message}` }],
        isError: true
      };
    }
  });

  const transport = new StdioServerTransport();
  await server.connect(transport);
}

import { fileURLToPath } from 'url';
import * as path from 'path';
import * as os from 'os';

const defaultDir = path.join(os.homedir(), '.sdd-memory');
const dbPath = process.env.DB_PATH || path.join(defaultDir, 'local.db');
runMcpServer(dbPath).catch(console.error);
