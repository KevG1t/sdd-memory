import Database from 'better-sqlite3';
import { Observation, SyncMutation } from './types.js';
import * as fs from 'fs';
import * as path from 'path';

export class LocalStore {
  private db: Database.Database;

  constructor(dbPath: string) {
    const dir = path.dirname(dbPath);
    if (!fs.existsSync(dir)) {
      fs.mkdirSync(dir, { recursive: true });
    }
    this.db = new Database(dbPath);
  }

  public close(): void {
    if (this.db) {
      this.db.close();
    }
  }

  public async init(): Promise<void> {
    this.db.exec(`
      CREATE TABLE IF NOT EXISTS observations (
        id TEXT PRIMARY KEY,
        project TEXT NOT NULL,
        scope TEXT NOT NULL,
        topic TEXT NOT NULL,
        content TEXT NOT NULL,
        revision_count INTEGER NOT NULL DEFAULT 1,
        created_at TEXT NOT NULL,
        updated_at TEXT NOT NULL
      );

      CREATE VIRTUAL TABLE IF NOT EXISTS observations_fts USING fts5(
        project, scope, topic, content, content='observations', content_rowid='rowid'
      );

      CREATE TABLE IF NOT EXISTS sync_mutations (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        table_name TEXT NOT NULL,
        record_id TEXT NOT NULL,
        operation TEXT NOT NULL CHECK(operation IN ('upsert', 'delete')),
        payload TEXT,
        timestamp TEXT NOT NULL,
        status TEXT NOT NULL DEFAULT 'pending' CHECK(status IN ('pending', 'chunked', 'synced'))
      );
    `);
  }

  private logMutation(tableName: string, recordId: string, operation: 'upsert' | 'delete', payload?: any): void {
    const stmt = this.db.prepare(
      `INSERT INTO sync_mutations (table_name, record_id, operation, payload, timestamp, status)
       VALUES (?, ?, ?, ?, ?, 'pending')`
    );
    stmt.run(
      tableName,
      recordId,
      operation,
      payload ? JSON.stringify(payload) : null,
      new Date().toISOString()
    );
  }

  public async getObservation(id: string): Promise<Observation | null> {
    const row = this.db.prepare('SELECT * FROM observations WHERE id = ?').get(id) as any;
    if (!row) return null;
    return {
      id: row.id,
      project: row.project,
      scope: row.scope,
      topic: row.topic,
      content: row.content,
      revisionCount: row.revision_count,
      createdAt: row.created_at,
      updatedAt: row.updated_at
    };
  }

  public async saveObservation(obs: Observation): Promise<void> {
    const tx = this.db.transaction(() => {
      const existing = this.db.prepare('SELECT id FROM observations WHERE id = ?').get(obs.id);
      
      const stmt = this.db.prepare(`
        INSERT INTO observations (id, project, scope, topic, content, revision_count, created_at, updated_at)
        VALUES (@id, @project, @scope, @topic, @content, @revisionCount, @createdAt, @updatedAt)
        ON CONFLICT(id) DO UPDATE SET
          project = excluded.project,
          scope = excluded.scope,
          topic = excluded.topic,
          content = excluded.content,
          revision_count = excluded.revision_count,
          updated_at = excluded.updated_at
      `);

      stmt.run({
        id: obs.id,
        project: obs.project,
        scope: obs.scope,
        topic: obs.topic,
        content: obs.content,
        revisionCount: obs.revisionCount ?? 1,
        createdAt: obs.createdAt,
        updatedAt: obs.updatedAt
      });

      // Update FTS table
      if (existing) {
        this.db.prepare('DELETE FROM observations_fts WHERE rowid = (SELECT rowid FROM observations WHERE id = ?)').run(obs.id);
      }
      this.db.prepare(`
        INSERT INTO observations_fts (rowid, project, scope, topic, content)
        VALUES ((SELECT rowid FROM observations WHERE id = ?), ?, ?, ?, ?)
      `).run(obs.id, obs.project, obs.scope, obs.topic, obs.content);

      // Log mutation
      this.logMutation('observations', obs.id, 'upsert', obs);
    });
    tx();
  }

  public async deleteObservation(id: string): Promise<void> {
    const tx = this.db.transaction(() => {
      this.db.prepare('DELETE FROM observations_fts WHERE rowid = (SELECT rowid FROM observations WHERE id = ?)').run(id);
      const res = this.db.prepare('DELETE FROM observations WHERE id = ?').run(id);
      
      if (res.changes > 0) {
        this.logMutation('observations', id, 'delete');
      }
    });
    tx();
  }

  public async searchObservations(query: string, project?: string): Promise<Observation[]> {
    let sql = `
      SELECT o.* FROM observations_fts fts
      JOIN observations o ON o.rowid = fts.rowid
      WHERE observations_fts MATCH ?
    `;
    const params: any[] = [query];

    if (project) {
      sql += ` AND o.project = ?`;
      params.push(project);
    }
    
    sql += ' ORDER BY rank';

    const rows = this.db.prepare(sql).all(...params) as any[];
    return rows.map(row => ({
      id: row.id,
      project: row.project,
      scope: row.scope,
      topic: row.topic,
      content: row.content,
      revisionCount: row.revision_count,
      createdAt: row.created_at,
      updatedAt: row.updated_at
    }));
  }

  public async findByTopicKey(project: string, scope: string, topic: string): Promise<Observation | null> {
    const row = this.db.prepare('SELECT * FROM observations WHERE project = ? AND scope = ? AND topic = ?').get(project, scope, topic) as any;
    if (!row) return null;
    return {
      id: row.id,
      project: row.project,
      scope: row.scope,
      topic: row.topic,
      content: row.content,
      revisionCount: row.revision_count,
      createdAt: row.created_at,
      updatedAt: row.updated_at
    };
  }

  public async getPendingMutations(): Promise<SyncMutation[]> {
    const rows = this.db.prepare("SELECT * FROM sync_mutations WHERE status = 'pending' ORDER BY id ASC").all() as any[];
    return rows.map(row => ({
      id: row.id,
      tableName: row.table_name,
      recordId: row.record_id,
      operation: row.operation,
      payload: row.payload ? JSON.parse(row.payload) : undefined,
      timestamp: row.timestamp,
      status: row.status
    }));
  }

  public async getMutationsAfter(timestamp: string): Promise<SyncMutation[]> {
    const rows = this.db.prepare("SELECT * FROM sync_mutations WHERE timestamp > ? ORDER BY timestamp ASC, id ASC").all(timestamp) as any[];
    return rows.map(row => ({
      id: row.id,
      tableName: row.table_name,
      recordId: row.record_id,
      operation: row.operation,
      payload: row.payload ? JSON.parse(row.payload) : undefined,
      timestamp: row.timestamp,
      status: row.status
    }));
  }

  public async markMutationsStatus(ids: number[], status: 'chunked' | 'synced'): Promise<void> {
    if (ids.length === 0) return;
    const placeholders = ids.map(() => '?').join(',');
    this.db.prepare(`UPDATE sync_mutations SET status = ? WHERE id IN (${placeholders})`).run(status, ...ids);
  }

  public async markAllChunkedAsSynced(): Promise<void> {
    this.db.prepare("UPDATE sync_mutations SET status = 'synced' WHERE status = 'chunked'").run();
  }
}
