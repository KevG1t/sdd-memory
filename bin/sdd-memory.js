#!/usr/bin/env node

import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
import * as path from 'node:path';
import * as fs from 'node:fs';

const args = process.argv.slice(2);
const command = args[0];

// Dynamic import paths depending on the module
// We map tui to cli
const cmdMap = {
  cli: '../dist/cli/index.js',
  tui: '../dist/cli/index.js',
  mcp: '../dist/mcp/index.js',
};

async function run() {
  if (!command || !cmdMap[command]) {
    console.error(`Usage: sdd-memory <cli|tui|mcp>`);
    process.exit(1);
  }

  const modulePath = cmdMap[command];
  const absolutePath = path.join(path.dirname(fileURLToPath(import.meta.url)), modulePath);

  if (!fs.existsSync(absolutePath)) {
    console.error(`Module not found: ${absolutePath}`);
    console.error(`Did you forget to build the project?`);
    process.exit(1);
  }

  // Use dynamic import to execute the script
  await import(new URL('file://' + absolutePath).href);
}

run().catch((err) => {
  console.error('Execution failed:', err);
  process.exit(1);
});
