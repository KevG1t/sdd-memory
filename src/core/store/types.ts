export interface Observation {
  id: string;
  project: string;
  scope: string;
  topic: string;
  content: string;
  revisionCount?: number;
  createdAt: string;
  updatedAt: string;
}

export interface SyncMutation {
  id: number;
  tableName: string;
  recordId: string;
  operation: 'upsert' | 'delete';
  payload?: any;
  timestamp: string;
  status: 'pending' | 'chunked' | 'synced';
}

export interface SyncManifest {
  version: number;
  chunks: { filename: string; hash: string; createdAt: string; recordCount: number; }[];
  lastSyncTimestamp: string;
}
