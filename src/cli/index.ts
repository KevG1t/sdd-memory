import * as p from '@clack/prompts';
import { LocalStore } from '../core/store/LocalStore.js';
import { SyncManager } from '../core/sync/SyncManager.js';
import { CloudClient } from '../core/cloud/CloudClient.js';
import * as path from 'path';

async function main() {
  p.intro('SDD Memory CLI');

  const defaultDir = path.join(process.cwd(), '.sdd-memory');
  const dbPath = process.env.DB_PATH || path.join(defaultDir, 'local.db');
  const syncDir = process.env.SYNC_DIR || path.join(defaultDir, 'sync_chunks');
  const store = new LocalStore(dbPath);

  // Auto-initialize silently to prevent SQLite missing table errors
  await store.init();

  // Initial clear for the app feel
  console.clear();

  while (true) {
    const action = await p.select({
      message: 'What would you like to do?',
      options: [
        { value: 'status', label: 'View Sync Status' },
        { value: 'sync', label: 'Trigger Cloud Sync' },
        { value: 'init', label: 'Force Initialize Database' },
        { value: 'exit', label: 'Exit' }
      ]
    });

    if (p.isCancel(action) || action === 'exit') {
      p.outro('Goodbye!');
      store.close();
      process.exit(0);
    }

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
        try {
          await syncManager.pushToCloud(client);
          s.stop('Sync completed successfully.');
        } catch (err: any) {
          s.stop(`Cloud sync failed: ${err.message}`);
        }
      }
    } else if (action === 'status') {
      const pending = await store.getPendingMutations();
      p.note(`There are ${pending.length} pending mutations waiting to be synced.`, 'Status');
    }

    // TUI Experience: Wait for user to read, then clear and re-render
    await p.select({
      message: 'Press Enter to continue',
      options: [{ value: 'back', label: 'Return to main menu' }]
    });
    console.clear();
  }
}

import { fileURLToPath } from 'url';

main().catch(console.error);
