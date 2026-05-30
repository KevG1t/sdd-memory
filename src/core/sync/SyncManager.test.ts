import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { SyncManager } from './SyncManager';
import { LocalStore } from '../store/LocalStore';
import { CloudClient } from '../cloud/CloudClient';
import * as fs from 'fs';
import * as path from 'path';

describe('SyncManager', () => {
  let localStore: LocalStore;
  let syncManager: SyncManager;
  const dbPath = path.join(__dirname, 'test.db');
  const syncDir = path.join(__dirname, 'sync_dir');

  beforeEach(async () => {
    if (fs.existsSync(dbPath)) fs.unlinkSync(dbPath);
    if (fs.existsSync(syncDir)) fs.rmSync(syncDir, { recursive: true, force: true });
    
    localStore = new LocalStore(dbPath);
    await localStore.init();
    syncManager = new SyncManager(localStore, syncDir);
  });

  afterEach(() => {
    localStore.close();
    if (fs.existsSync(dbPath)) fs.unlinkSync(dbPath);
    if (fs.existsSync(syncDir)) fs.rmSync(syncDir, { recursive: true, force: true });
  });

  it('should generate chunk idempotently when no pending mutations', async () => {
    const chunkPath = await syncManager.generateChunk();
    expect(chunkPath).toBeNull();
    const manifest = await syncManager.getManifest();
    expect(manifest.chunks.length).toBe(0);
  });

  it('should generate chunk and update manifest when pending mutations exist', async () => {
    await localStore.saveObservation({
      id: 'obs-1', project: 'p', scope: 's', topic: 't', content: 'c', createdAt: 'a', updatedAt: 'b'
    });
    const chunkPath = await syncManager.generateChunk();
    expect(chunkPath).not.toBeNull();
    expect(fs.existsSync(chunkPath!)).toBe(true);

    const manifest = await syncManager.getManifest();
    expect(manifest.chunks.length).toBe(1);
    expect(manifest.chunks[0].recordCount).toBe(1);

    const pending = await localStore.getPendingMutations();
    expect(pending.length).toBe(0); // Because they were marked chunked!
  });

  it('should push to cloud and mark as synced', async () => {
    await localStore.saveObservation({
      id: 'obs-2', project: 'p', scope: 's', topic: 't', content: 'c', createdAt: 'a', updatedAt: 'b'
    });
    await syncManager.generateChunk();
    
    const client = new CloudClient('http://localhost', 'secret');
    const uploadSpy = vi.spyOn(client, 'uploadChunk');

    await syncManager.pushToCloud(client);
    expect(uploadSpy).toHaveBeenCalledTimes(1);

    // Verify it's synced (internal DB check)
    const result = (localStore as any).db.prepare("SELECT count(*) as c FROM sync_mutations WHERE status = 'synced'").get();
    expect(result.c).toBe(1);
  });
});
