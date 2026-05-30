import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { CloudBackend } from './CloudBackend';

// Mock pg module
vi.mock('pg', () => {
  const Client = vi.fn();
  Client.prototype.connect = vi.fn();
  Client.prototype.end = vi.fn();
  Client.prototype.query = vi.fn();
  return { Client };
});

import { Client } from 'pg';

describe('CloudBackend', () => {
  let backend: CloudBackend;
  let queryMock: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    vi.clearAllMocks();
    queryMock = vi.fn().mockResolvedValue({ rowCount: 1, rows: [{ id: 'mocked-uuid' }] });
    (Client.prototype.query as any) = queryMock;
    
    backend = new CloudBackend('postgres://mock');
  });

  it('should initialize tables', async () => {
    await backend.init();
    expect(Client.prototype.connect).toHaveBeenCalled();
    expect(queryMock).toHaveBeenCalledTimes(1);
    expect(queryMock.mock.calls[0][0]).toContain('CREATE TABLE IF NOT EXISTS');
  });

  it('should process a chunk atomically with idempotency', async () => {
    const chunk = `{"tableName":"observations","recordId":"obs-1","operation":"upsert","payload":{"id":"obs-1","project":"test","scope":"s","topic":"t","content":"c","createdAt":"2023","updatedAt":"2023"}}
{"tableName":"observations","recordId":"obs-2","operation":"delete"}`;
    
    await backend.processChunk(chunk);

    // Should begin and commit transaction
    expect(queryMock).toHaveBeenCalledWith('BEGIN');
    expect(queryMock).toHaveBeenCalledWith('COMMIT');

    // Should insert/upsert orgs and projects, then upsert observation
    const upsertObsCalls = queryMock.mock.calls.filter(call => 
      call[0].includes('INSERT INTO observations') && call[0].includes('ON CONFLICT')
    );
    expect(upsertObsCalls.length).toBe(1);
    expect(upsertObsCalls[0][1][0]).toBe('obs-1');

    // Should delete observation
    const deleteObsCalls = queryMock.mock.calls.filter(call => 
      call[0].includes('DELETE FROM observations')
    );
    expect(deleteObsCalls.length).toBe(1);
    expect(deleteObsCalls[0][1][0]).toBe('obs-2');
  });

  it('should rollback on failure', async () => {
    const chunk = `{"tableName":"observations","recordId":"obs-1","operation":"upsert","payload":{"id":"obs-1","project":"test"}}`;
    
    queryMock.mockImplementationOnce((q) => Promise.resolve()) // BEGIN
             .mockImplementationOnce((q) => Promise.reject(new Error('DB Error'))); // Org INSERT fails

    await expect(backend.processChunk(chunk)).rejects.toThrow('DB Error');
    expect(queryMock).toHaveBeenCalledWith('ROLLBACK');
  });
});
