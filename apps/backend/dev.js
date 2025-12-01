import { spawn } from 'child_process';
import path from 'path';
import os from 'os';
import process from 'process';

// 1. 현재 절대 경로 가져오기
const currentDir = process.cwd();

// 2. 캐시 디렉토리 설정 (프로젝트 내부 tmp/gocache)
const goCachePath = path.join(currentDir, 'tmp', 'gocache');

// 3. 환경변수 병합
const env = { 
  ...process.env, 
  GOCACHE: goCachePath,
};

console.log(`[Dev] Starting Air...`);
console.log(`[Dev] OS: ${os.platform()}`);
console.log(`[Dev] GOCACHE: ${goCachePath}`);

// 4. Air 실행
const air = spawn('air', { 
  env: env, 
  stdio: 'inherit',
  shell: true
});

air.on('close', (code) => {
  console.log(`[Dev] Air process exited with code ${code}`);
});