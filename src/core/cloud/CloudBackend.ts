import { Client } from 'pg';

export class CloudBackend {
  private client: Client;

  constructor(pgConnectionString: string) {
    this.client = new Client({ connectionString: pgConnectionString });
  }

  public async init(): Promise<void> {
    await this.client.connect();
    await this.client.query(`
      CREATE TABLE IF NOT EXISTS organizations (
        id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
        name TEXT NOT NULL,
        policies JSONB
      );

      CREATE TABLE IF NOT EXISTS projects (
        id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
        org_id UUID REFERENCES organizations(id),
        name TEXT NOT NULL
      );

      CREATE TABLE IF NOT EXISTS observations (
        id TEXT PRIMARY KEY,
        project_id UUID REFERENCES projects(id),
        scope TEXT NOT NULL,
        topic TEXT NOT NULL,
        content TEXT NOT NULL,
        created_at TIMESTAMPTZ NOT NULL,
        updated_at TIMESTAMPTZ NOT NULL
      );
    `);
  }

  public async close(): Promise<void> {
    await this.client.end();
  }

  public async processChunk(chunkContent: string): Promise<void> {
    const lines = chunkContent.split('\n').filter(l => l.trim() !== '');
    
    // We run the whole chunk in a transaction to ensure atomic processing
    await this.client.query('BEGIN');
    try {
      for (const line of lines) {
        const mutation = JSON.parse(line);
        if (mutation.tableName === 'observations') {
          if (mutation.operation === 'upsert') {
            const p = mutation.payload;
            // The spec maps 'project' string from sqlite to 'project_id' in postgres, but our 
            // chunk comes from SQLite where 'project' is just a string.
            // In a real scenario we'd resolve project name to project_id. 
            // For simplicity in this demo, let's assume we can map the project string or just allow the UUID to be looked up.
            // Since the project schema uses UUID and SQLite uses a string 'sdd-memory', we need a lookup or to store it differently.
            // Let's create the project if it doesn't exist, assigning it to a default org.
            
            // First ensure default org
            const orgRes = await this.client.query("INSERT INTO organizations (name, policies) VALUES ('Default Org', '{}') ON CONFLICT DO NOTHING RETURNING id");
            let orgId;
            if (orgRes.rowCount === 0) {
              const res = await this.client.query("SELECT id FROM organizations LIMIT 1");
              orgId = res.rows[0].id;
            } else {
              orgId = orgRes.rows[0].id;
            }

            // Ensure project exists and get UUID
            let projRes = await this.client.query("SELECT id FROM projects WHERE name = $1", [p.project]);
            let projectId;
            if (projRes.rowCount === 0) {
              projRes = await this.client.query("INSERT INTO projects (org_id, name) VALUES ($1, $2) RETURNING id", [orgId, p.project]);
            }
            projectId = projRes.rows[0].id;

            await this.client.query(`
              INSERT INTO observations (id, project_id, scope, topic, content, created_at, updated_at)
              VALUES ($1, $2, $3, $4, $5, $6, $7)
              ON CONFLICT (id) DO UPDATE SET
                project_id = EXCLUDED.project_id,
                scope = EXCLUDED.scope,
                topic = EXCLUDED.topic,
                content = EXCLUDED.content,
                updated_at = EXCLUDED.updated_at
            `, [p.id, projectId, p.scope, p.topic, p.content, p.createdAt, p.updatedAt]);
            
          } else if (mutation.operation === 'delete') {
            await this.client.query('DELETE FROM observations WHERE id = $1', [mutation.recordId]);
          }
        }
      }
      await this.client.query('COMMIT');
    } catch (e) {
      await this.client.query('ROLLBACK');
      throw e;
    }
  }

  public async getChunksSince(timestamp: string): Promise<string[]> {
    return [];
  }
}
