import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { LocalStore } from './LocalStore';
import * as fs from 'fs';
import * as path from 'path';

describe('LocalStore', () => {
  let store: LocalStore;
  const dbPath = path.join(__dirname, 'test.db');

  beforeEach(async () => {
    if (fs.existsSync(dbPath)) {
      fs.unlinkSync(dbPath);
    }
    store = new LocalStore(dbPath);
    await store.init();
  });

  afterEach(() => {
    store.close();
    if (fs.existsSync(dbPath)) {
      fs.unlinkSync(dbPath);
    }
  });

  it('should save and retrieve an observation', async () => {
    const obs = {
      id: 'obs-1',
      project: 'test-project',
      scope: 'test-scope',
      topic: 'test-topic',
      content: 'Hello world',
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString()
    };
    await store.saveObservation(obs);

    const retrieved = await store.getObservation('obs-1');
    expect(retrieved).toEqual({ ...obs, revisionCount: 1 });
  });

  it('should update existing observation and fts', async () => {
    const obs = {
      id: 'obs-2',
      project: 'test-project',
      scope: 'test-scope',
      topic: 'test-topic',
      content: 'Initial content',
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString()
    };
    await store.saveObservation(obs);

    const updatedObs = { ...obs, content: 'Updated content' };
    await store.saveObservation(updatedObs);

    const retrieved = await store.getObservation('obs-2');
    expect(retrieved?.content).toBe('Updated content');
  });

  it('should delete observation', async () => {
    const obs = {
      id: 'obs-3',
      project: 'test-project',
      scope: 'test-scope',
      topic: 'test-topic',
      content: 'To be deleted',
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString()
    };
    await store.saveObservation(obs);
    await store.deleteObservation('obs-3');
    
    const retrieved = await store.getObservation('obs-3');
    expect(retrieved).toBeNull();
  });

  it('should track mutations for sync', async () => {
    const obs = {
      id: 'obs-4',
      project: 'test-project',
      scope: 'test-scope',
      topic: 'test-topic',
      content: 'Mutation test',
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString()
    };
    await store.saveObservation(obs);
    let mutations = await store.getPendingMutations();
    expect(mutations.length).toBe(1);
    expect(mutations[0].operation).toBe('upsert');
    expect(mutations[0].recordId).toBe('obs-4');

    await store.deleteObservation('obs-4');
    mutations = await store.getPendingMutations();
    expect(mutations.length).toBe(2);
    expect(mutations[1].operation).toBe('delete');
    expect(mutations[1].recordId).toBe('obs-4');
  });

  it('should search using FTS5', async () => {
    const obs1 = {
      id: 'obs-5',
      project: 'search-project',
      scope: 'search-scope',
      topic: 'search-topic',
      content: 'The quick brown fox jumps over the lazy dog',
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString()
    };
    const obs2 = {
      id: 'obs-6',
      project: 'search-project',
      scope: 'search-scope',
      topic: 'search-topic',
      content: 'A fast brown dog',
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString()
    };
    await store.saveObservation(obs1);
    await store.saveObservation(obs2);

    const results = await store.searchObservations('fox');
    expect(results.length).toBe(1);
    expect(results[0].id).toBe('obs-5');

    const projectFiltered = await store.searchObservations('dog', 'search-project');
    expect(projectFiltered.length).toBe(2);
  });
});
