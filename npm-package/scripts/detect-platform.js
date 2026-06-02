const os = require('os');

/**
 * Detecta la plataforma y arquitectura actual y mapea a los nombres
 * de binarios usados en los GitHub releases
 */
function detectPlatform() {
  const platform = os.platform();
  const arch = os.arch();
  
  // Mapeo de plataformas Node.js a nombres de binarios
  const platformMap = {
    'linux': {
      'x64': 'sdd-memory-linux-amd64',
      'arm64': 'sdd-memory-linux-arm64'
    },
    'darwin': {
      'x64': 'sdd-memory-darwin-amd64', 
      'arm64': 'sdd-memory-darwin-arm64'
    },
    'win32': {
      'x64': 'sdd-memory-windows-amd64.exe',
      'arm64': 'sdd-memory-windows-arm64.exe'
    }
  };
  
  if (!platformMap[platform]) {
    throw new Error(`Unsupported platform: ${platform}`);
  }
  
  if (!platformMap[platform][arch]) {
    throw new Error(`Unsupported architecture: ${arch} on ${platform}`);
  }
  
  const binaryName = platformMap[platform][arch];
  const isWindows = platform === 'win32';
  const isDarwin = platform === 'darwin';
  const isLinux = platform === 'linux';
  const isArm64 = arch === 'arm64';
  
  return {
    platform,
    arch,
    binaryName,
    isWindows,
    isDarwin,
    isLinux,
    isArm64,
    downloadUrl: null // Se establecerá en install.js
  };
}

/**
 * Obtiene el nombre del binario simple para el alias universal
 */
function getSimpleBinaryName(platform) {
  const simpleMap = {
    'linux': 'sdd-memory-linux',
    'darwin': 'sdd-memory-macos',
    'win32': 'sdd-memory-windows.exe'
  };
  
  return simpleMap[platform];
}

module.exports = {
  detectPlatform,
  getSimpleBinaryName
};