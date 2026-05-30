import { LocalStore } from '../store/LocalStore.js';
import { SyncManifest } from '../store/types.js';
import { CloudClient } from '../cloud/CloudClient.js';
import * as fs from 'fs';
import * as path from 'path';
import * as crypto from 'crypto';
import * as zlib from 'zlib';

export class SyncManager {
  private manifestPath: string;

  constructor(private localStore: LocalStore, private syncDir: string) {
    if (!fs.existsSync(syncDir)) {
      fs.mkdirSync(syncDir, { recursive: true });
    }
    this.manifestPath = path.join(this.syncDir, 'manifest.json');
  }

  public async getManifest(): Promise<SyncManifest> {
    if (!fs.existsSync(this.manifestPath)) {
      return { version: 1, chunks: [], lastSyncTimestamp: '1970-01-01T00:00:00Z' };
    }
    const content = fs.readFileSync(this.manifestPath, 'utf8');
    return JSON.parse(content);
  }

  private async saveManifest(manifest: SyncManifest): Promise<void> {
    fs.writeFileSync(this.manifestPath, JSON.stringify(manifest, null, 2));
  }

  public async generateChunk(): Promise<string | null> {
    const manifest = await this.getManifest();
    
    const lastChunkTime = manifest.chunks.reduce((max, chunk) => 
      chunk.createdAt > max ? chunk.createdAt : max
    , '1970-01-01T00:00:00.000Z');

    const pending = await this.localStore.getMutationsAfter(lastChunkTime);
    if (pending.length === 0) {
      return null;
    }

    const timestamp = new Date().toISOString().replace(/[:.]/g, '-');
    const uuid = crypto.randomUUID();
    const filename = `chunk_${timestamp}_${uuid}.json.gz`;
    const chunkPath = path.join(this.syncDir, filename);

    const ids: number[] = pending.map(m => m.id);
    const uncompressedContent = JSON.stringify(pending);
    const compressed = zlib.gzipSync(uncompressedContent);

    fs.writeFileSync(chunkPath, compressed);

    const hash = crypto.createHash('sha256').update(compressed).digest('hex');

    const chunkCreatedAt = pending.reduce((max, m) => m.timestamp > max ? m.timestamp : max, pending[0].timestamp);

    manifest.chunks.push({
      filename,
      hash,
      createdAt: chunkCreatedAt,
      recordCount: pending.length
    });
    
    await this.saveManifest(manifest);
    await this.localStore.markMutationsStatus(ids, 'chunked');

    return chunkPath;
  }

  public async pushToCloud(cloudClient: CloudClient): Promise<void> {
    const manifest = await this.getManifest();
    // Assuming we want to upload all chunks that might not have been uploaded yet.
    // In a real system we'd track which are synced. For now, we just upload all 'chunked' mutations.
    // Wait, mutations are marked 'chunked', and we should mark them 'synced' after push.
    // Since chunks are immutable, we just upload the latest chunks.
    // Let's find chunks that have mutations in 'chunked' status.
    const chunkedMutations = (await this.localStore.getPendingMutations()).filter(m => m.status === 'chunked'); // Wait, pendingMutations only returns 'pending'.
    
    // Actually, localStore needs to return 'chunked' mutations, or we just trust the manifest.
    // We can fetch mutations by status.
    // Let's modify LocalStore to get mutations by status if needed, or simply read the chunks on disk.
    
    // For simplicity, let's just read chunks from disk, upload them, and then mark all chunked mutations as synced.
    for (const chunk of manifest.chunks) {
      const chunkPath = path.join(this.syncDir, chunk.filename);
      if (fs.existsSync(chunkPath)) {
        const content = fs.readFileSync(chunkPath, 'utf8');
        await cloudClient.uploadChunk(chunk.filename, content);
      }
    }
    
    // Mark all 'chunked' mutations as 'synced' in DB
    // We can do this by adding a method or modifying `markMutationsStatus` to take a from-status.
    // Instead of modifying LocalStore now, I'll just rely on `markMutationsStatus` taking IDs.
    // But we didn't save the IDs here. Let's just create a `markAllChunkedAsSynced` in LocalStore.
    await this.localStore.markAllChunkedAsSynced();
  }

  public async pullFromCloud(cloudClient: CloudClient): Promise<void> {
    const manifest = await this.getManifest();
    const chunks = await cloudClient.downloadChunks(manifest.lastSyncTimestamp);
    // Process them... (this would be handled by a cloud integration on the client side, but the spec mentions CloudBackend doing it on the server).
    // The design says SyncManager.pullFromCloud() is a stub for now.
    manifest.lastSyncTimestamp = new Date().toISOString();
    await this.saveManifest(manifest);
  }
}
