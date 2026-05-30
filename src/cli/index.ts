import * as p from '@clack/prompts';
import { LocalStore } from '../core/store/LocalStore.js';
import { SyncManager } from '../core/sync/SyncManager.js';
import { CloudClient } from '../core/cloud/CloudClient.js';
import * as path from 'path';

async function main() {
  p.intro('SDD Memory CLI');

  const action = await p.select({
    message: 'What would you like to do?',
    options: [
      { value: 'init', label: 'Initialize Database' },
      { value: 'sync', label: 'Trigger Cloud Sync' },
      { value: 'status', label: 'View Sync Status' }
    ]
  });

  if (p.isCancel(action)) {
    p.cancel('Operation cancelled.');
    process.exit(0);
  }

  const dbPath = process.env.DB_PATH || path.join(process.cwd(), 'local.db');
  const syncDir = process.env.SYNC_DIR || path.join(process.cwd(), 'sync_chunks');
  const store = new LocalStore(dbPath);

  // Auto-initialize silently to prevent SQLite missing table errors
  await store.init();

  if (action === 'init') {
    const s = p.spinner();
    s.start('Initializing database...');
    await store.init();
    s.stop('Database initialized at ' + dbPath);
  } else if (action === 'sync') {
    const s = p.spinner();
    s.start('Generating chunk...');
    const syncManager = new SyncManager(store, syncDir);
    const chunkPath = await syncManager.generateChunk();
    if (!chunkPath) {
      s.stop('No pending mutations to sync.');
    } else {
      s.message('Chunk generated at ' + chunkPath + '. Pushing to cloud...');
      const client = new CloudClient(process.env.CLOUD_ENDPOINT || 'http://localhost', process.env.CLOUD_API_KEY || '');
      await syncManager.pushToCloud(client);
      s.stop('Sync completed successfully.');
    }
  } else if (action === 'status') {
    const pending = await store.getPendingMutations();
    p.note(`There are ${pending.length} pending mutations waiting to be synced.`, 'Status');
  }

  store.close();
  p.outro('Done!');
}

import { fileURLToPath } from 'url';

main().catch(console.error);
