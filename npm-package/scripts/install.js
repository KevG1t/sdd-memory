#!/usr/bin/env node

const fs = require('fs');
const path = require('path');
const https = require('https');
const { detectPlatform, getSimpleBinaryName } = require('./detect-platform');

const REPO_OWNER = 'KevG1t';
const REPO_NAME = 'sdd-memory';
const GITHUB_API_BASE = 'https://api.github.com';
const GITHUB_RELEASES_BASE = 'https://github.com';

/**
 * Obtiene información del último release desde GitHub API
 */
async function getLatestRelease() {
  return new Promise((resolve, reject) => {
    const url = `${GITHUB_API_BASE}/repos/${REPO_OWNER}/${REPO_NAME}/releases/latest`;
    
    https.get(url, { 
      headers: { 'User-Agent': 'sdd-memory-npm-installer' } 
    }, (res) => {
      let data = '';
      
      res.on('data', chunk => data += chunk);
      res.on('end', () => {
        try {
          const release = JSON.parse(data);
          resolve(release);
        } catch (err) {
          reject(new Error(`Failed to parse GitHub API response: ${err.message}`));
        }
      });
    }).on('error', reject);
  });
}

/**
 * Descarga un archivo desde una URL
 */
async function downloadFile(url, outputPath) {
  return new Promise((resolve, reject) => {
    const file = fs.createWriteStream(outputPath);
    
    https.get(url, (res) => {
      if (res.statusCode !== 200) {
        reject(new Error(`Download failed with status ${res.statusCode}`));
        return;
      }
      
      res.pipe(file);
      
      file.on('finish', () => {
        file.close();
        resolve();
      });
      
      file.on('error', reject);
    }).on('error', reject);
  });
}

/**
 * Hace ejecutable un archivo (Unix/Linux/macOS)
 */
function makeExecutable(filePath) {
  try {
    fs.chmodSync(filePath, 0o755);
  } catch (err) {
    console.warn(`Warning: Could not make ${filePath} executable:`, err.message);
  }
}

/**
 * Instala el binario de sdd-memory
 */
async function install() {
  try {
    console.log('🔍 Detecting platform...');
    const platformInfo = detectPlatform();
    console.log(`📱 Platform: ${platformInfo.platform}-${platformInfo.arch}`);
    console.log(`📦 Binary: ${platformInfo.binaryName}`);
    
    console.log('🌐 Fetching latest release info...');
    const release = await getLatestRelease();
    console.log(`🏷️  Latest version: ${release.tag_name}`);
    
    // Buscar el asset correcto
    const asset = release.assets.find(a => 
      a.name === platformInfo.binaryName || 
      a.name === getSimpleBinaryName(platformInfo.platform)
    );
    
    if (!asset) {
      throw new Error(`Binary not found for ${platformInfo.platform}-${platformInfo.arch}`);
    }
    
    // Crear directorio de destino
    const binDir = path.join(__dirname, '..', 'dist');
    if (!fs.existsSync(binDir)) {
      fs.mkdirSync(binDir, { recursive: true });
    }
    
    const binaryPath = path.join(binDir, platformInfo.isWindows ? 'sdd-memory.exe' : 'sdd-memory');
    
    console.log(`⬇️  Downloading ${asset.name}...`);
    await downloadFile(asset.browser_download_url, binaryPath);
    
    // Hacer ejecutable en sistemas Unix
    if (!platformInfo.isWindows) {
      makeExecutable(binaryPath);
    }
    
    console.log(`✅ sdd-memory installed successfully at: ${binaryPath}`);
    console.log('');
    console.log('🚀 You can now run:');
    console.log('   npx @kevg1t/sdd-memory');
    console.log('   npx @kevg1t/sdd-memory mcp');
    console.log('');
    
  } catch (error) {
    console.error('❌ Installation failed:', error.message);
    console.error('');
    console.error('💡 Alternative installation methods:');
    console.error('   curl -sSL https://raw.githubusercontent.com/KevG1t/sdd-memory/master/install.sh | bash');
    console.error('   irm https://raw.githubusercontent.com/KevG1t/sdd-memory/master/install.ps1 | iex');
    process.exit(1);
  }
}

// Ejecutar instalación solo si es llamado directamente
if (require.main === module) {
  install();
}

module.exports = { install };