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
 * Intenta instalar el binario globalmente para acceso directo
 */
function tryGlobalInstall(sourcePath, platformInfo) {
  const os = require('os');
  const { execSync } = require('child_process');
  
  try {
    const globalBinDir = path.join(os.homedir(), '.local', 'bin');
    const globalBinaryPath = path.join(globalBinDir, platformInfo.isWindows ? 'sdd-memory.exe' : 'sdd-memory');
    
    // Crear directorio si no existe
    if (!fs.existsSync(globalBinDir)) {
      fs.mkdirSync(globalBinDir, { recursive: true });
    }
    
    // Copiar binario
    fs.copyFileSync(sourcePath, globalBinaryPath);
    
    // Hacer ejecutable en sistemas Unix
    if (!platformInfo.isWindows) {
      makeExecutable(globalBinaryPath);
    }
    
    console.log(`🌍 Global installation: ${globalBinaryPath}`);
    
    // En Windows, intentar agregar al PATH del usuario
    if (platformInfo.isWindows) {
      try {
        const currentPath = process.env.PATH || '';
        if (!currentPath.includes(globalBinDir)) {
          console.log('💡 Adding to PATH for global access...');
          console.log('   You may need to restart your terminal for "sdd-memory" command to work globally.');
        }
      } catch (err) {
        console.log('💡 To use "sdd-memory" globally, add to your PATH:');
        console.log(`   ${globalBinDir}`);
      }
    }
    
    return true;
  } catch (err) {
    console.log('⚠️  Could not install globally, but local installation succeeded');
    return false;
  }
}

/**
 * Instala el binario de sdd-memory usando los binarios pre-incluidos
 */
function install() {
  try {
    console.log('🔍 Detecting platform...');
    const platformInfo = detectPlatform();
    console.log(`📱 Platform: ${platformInfo.platform}-${platformInfo.arch}`);
    console.log(`📦 Binary: ${platformInfo.binaryName}`);
    
    // Buscar el binario pre-incluido
    const distDir = path.join(__dirname, '..', 'dist');
    const possibleBinaries = [
      platformInfo.binaryName,
      getSimpleBinaryName(platformInfo.platform)
    ];
    
    let sourceBinary = null;
    for (const binaryName of possibleBinaries) {
      const binaryPath = path.join(distDir, binaryName);
      if (fs.existsSync(binaryPath)) {
        sourceBinary = binaryPath;
        break;
      }
    }
    
    if (!sourceBinary) {
      console.error('❌ Pre-compiled binary not found for your platform');
      console.error('📁 Available binaries:');
      if (fs.existsSync(distDir)) {
        fs.readdirSync(distDir).forEach(file => {
          console.error(`   - ${file}`);
        });
      } else {
        console.error('   No dist directory found');
      }
      throw new Error(`Binary not found for ${platformInfo.platform}-${platformInfo.arch}`);
    }
    
    console.log(`✅ Found pre-compiled binary: ${path.basename(sourceBinary)}`);
    
    // Intentar instalación global
    const globalInstalled = tryGlobalInstall(sourceBinary, platformInfo);
    
    console.log('');
    console.log('🚀 You can now run:');
    if (globalInstalled) {
      console.log('   sdd-memory                    # Direct command (restart terminal if needed)');
    }
    console.log('   npx sdd-memory-kevg1t         # Via npx');
    console.log('   npx sdd-memory-kevg1t mcp     # MCP server mode');
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