import path from 'path';
import { fileURLToPath } from 'url';
import fsExtra from 'fs-extra';

const scriptDir = path.dirname(fileURLToPath(import.meta.url));
const backendDir = path.resolve(scriptDir, '..');
const frontendDistPath = path.resolve(backendDir, '..', 'frontend', 'dist');
const backendWebDistPath = path.resolve(backendDir, 'dist', 'web');

if (!fsExtra.existsSync(frontendDistPath)) {
  console.error(`[Build] Frontend build not found at ${frontendDistPath}. Please build the frontend first.`);
  process.exit(1);
}

fsExtra.emptyDirSync(backendWebDistPath);
fsExtra.copySync(frontendDistPath, backendWebDistPath);

console.log(`[Build] Frontend assets copied to ${backendWebDistPath}`);
