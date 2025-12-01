import { spawn } from 'child_process';
import path from 'path';
import os from 'os';
import process from 'process';
import fsExtra from 'fs-extra';

// 1. 경로 설정
const currentDir = process.cwd();
const goCachePath = path.join(currentDir, 'tmp', 'gocache');
const frontendDistPath = path.join(currentDir, '..', 'frontend', 'dist');

// 2. 환경변수 주입
const env = {
  ...process.env,
  GOCACHE: goCachePath
};

console.log(`[Build] Starting Go Build...`);
console.log(`[Build] Current Directory: ${currentDir}`);
console.log(`[Build] OS: ${os.platform()}`);
console.log(`[Build] GOCACHE: ${goCachePath}`);

// 3. Frontend 빌드 결과물 복사
if (!fsExtra.existsSync(frontendDistPath)) {
  console.error(`[Build] Frontend build not found at ${frontendDistPath}. Please build the frontend first.`);
  process.exit(1);
}
fsExtra.mkdirpSync(path.join(currentDir, 'dist', 'web'));
fsExtra.copySync(frontendDistPath, path.join(currentDir, 'dist', 'web'));

// 4. Go Build 실행
// -o dist/main.exe : 결과물을 dist 폴더에 main.exe로 저장
const build = spawn('go build -tags=production -ldflags "-X main.goEnv=production" -o dist/main.exe .', {
  env: env,
  stdio: 'inherit',
  shell: true
});

// 5. 종료 코드 처리
build.on('close', (code) => {
  if (code === 0) {
    console.log(`[Build] Success! Output: dist/main.exe`);
  } else {
    console.error(`[Build] Failed with code ${code}`);
  }
  process.exit(code);
});