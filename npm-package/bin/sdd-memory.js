#!/usr/bin/env node

const fs = require('fs');
const path = require('path');
const { spawn } = require('child_process');
const { detectPlatform } = require('../scripts/detect-platform');

/**
 * Encuentra la ruta del binario instalado
 */
function findBinaryPath() {
  const platformInfo = detectPlatform();
  console.log('🔍 [DEBUG] Platform detection:', platformInfo);
  
  // Determinar el nombre del binario según la plataforma
  let binaryName;
  if (platformInfo.isWindows) {
    binaryName = platformInfo.isArm64 ? 'sdd-memory-windows-arm64.exe' : 'sdd-memory-windows-amd64.exe';
  } else if (platformInfo.isDarwin) {
    binaryName = platformInfo.isArm64 ? 'sdd-memory-darwin-arm64' : 'sdd-memory-darwin-amd64';
  } else if (platformInfo.isLinux) {
    binaryName = platformInfo.isArm64 ? 'sdd-memory-linux-arm64' : 'sdd-memory-linux-amd64';
  } else {
    binaryName = 'sdd-memory';
  }
  
  console.log('🎯 [DEBUG] Target binary name:', binaryName);
  
  // Buscar en el directorio dist del paquete npm
  const distPath = path.join(__dirname, '..', 'dist', binaryName);
  console.log('📂 [DEBUG] Checking dist path:', distPath);
  console.log('✅ [DEBUG] Dist path exists:', fs.existsSync(distPath));
  
  if (fs.existsSync(distPath)) {
    console.log('🚀 [DEBUG] Using specific binary:', distPath);
    return distPath;
  }
  
  // Fallback: intentar con el binario genérico
  const genericPath = path.join(__dirname, '..', 'dist', 'sdd-memory');
  console.log('📂 [DEBUG] Fallback generic path:', genericPath);
  console.log('✅ [DEBUG] Generic path exists:', fs.existsSync(genericPath));
  
  if (fs.existsSync(genericPath)) {
    console.log('🚀 [DEBUG] Using generic binary:', genericPath);
    return genericPath;
  }
  
  // Windows fallback: intentar con sdd-memory.exe
  if (platformInfo.isWindows) {
    const windowsGenericPath = path.join(__dirname, '..', 'dist', 'sdd-memory.exe');
    console.log('📂 [DEBUG] Windows fallback path:', windowsGenericPath);
    console.log('✅ [DEBUG] Windows fallback exists:', fs.existsSync(windowsGenericPath));
    
    if (fs.existsSync(windowsGenericPath)) {
      console.log('🚀 [DEBUG] Using Windows generic binary:', windowsGenericPath);
      return windowsGenericPath;
    }
  }
  
  // Último fallback: buscar en PATH del sistema
  console.log('⚠️  [DEBUG] Using system PATH fallback: sdd-memory');
  return 'sdd-memory';
}

/**
 * Ejecuta el binario real de sdd-memory
 */
function executeBinary() {
  try {
    const binaryPath = findBinaryPath();
    const args = process.argv.slice(2); // Remover 'node' y script path
    
    console.log('🎯 [DEBUG] Final binary path:', binaryPath);
    console.log('📋 [DEBUG] Arguments:', args);
    console.log('🖥️  [DEBUG] Process platform:', process.platform);
    console.log('🔧 [DEBUG] Node arch:', process.arch);
    
    // Verificar si el archivo existe y es ejecutable
    if (binaryPath !== 'sdd-memory') {
      try {
        const stats = fs.statSync(binaryPath);
        console.log('📊 [DEBUG] Binary file stats:', {
          size: stats.size,
          isFile: stats.isFile(),
          mode: stats.mode.toString(8)
        });
      } catch (err) {
        console.log('❌ [DEBUG] Cannot stat binary file:', err.message);
      }
    }
    
    // Spawn el proceso hijo con stdio inherit para pasar entrada/salida
    console.log('🚀 [DEBUG] About to spawn process...');
    const child = spawn(binaryPath, args, {
      stdio: 'inherit',
      shell: false
    });
    
    // Manejar señales del sistema
    process.on('SIGINT', () => child.kill('SIGINT'));
    process.on('SIGTERM', () => child.kill('SIGTERM'));
    
    // Propagar el código de salida
    child.on('exit', (code, signal) => {
      if (signal) {
        process.kill(process.pid, signal);
      } else {
        process.exit(code || 0);
      }
    });
    
    // Manejar errores del proceso hijo
    child.on('error', (err) => {
      console.log('💥 [DEBUG] Child process error:', {
        code: err.code,
        message: err.message,
        errno: err.errno,
        syscall: err.syscall,
        path: err.path
      });
      
      if (err.code === 'ENOENT') {
        console.error('❌ sdd-memory binary not found!');
        console.error('');
        console.error('💡 Try reinstalling:');
        console.error('   npm uninstall sdd-memory-kevg1t');
        console.error('   npm install sdd-memory-kevg1t');
        console.error('');
        console.error('🔄 Or use alternative installation:');
        console.error('   curl -sSL https://raw.githubusercontent.com/KevG1t/sdd-memory/master/install.sh | bash');
        process.exit(1);
      } else {
        console.error('❌ Error executing sdd-memory:', err.message);
        process.exit(1);
      }
    });
    
  } catch (error) {
    console.error('❌ Failed to execute sdd-memory:', error.message);
    process.exit(1);
  }
}

// Ejecutar el binario
executeBinary();