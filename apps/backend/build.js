import { spawn } from 'child_process';
import path from 'path';
import process from 'process';

// 1. 경로 설정
const currentDir = process.cwd();
const goCachePath = path.join(currentDir, 'tmp', 'gocache');

// 2. 환경변수 주입
const env = {
  ...process.env,
  GOCACHE: goCachePath,
  GO_ENV: 'production'
};

console.log(`[Build] Starting Go Build...`);
console.log(`[Build] GOCACHE: ${goCachePath}`);
console.log(`[Build] GO_ENV: ${env.GO_ENV}`);

// 3. Go Build 실행
// -o dist/main.exe : 결과물을 dist 폴더에 main.exe로 저장
const build = spawn('go build -o dist/main.exe .', {
  env: env,
  stdio: 'inherit',
  shell: true
});

// 4. 종료 코드 처리
build.on('close', (code) => {
  if (code === 0) {
    console.log(`[Build] Success! Output: dist/main.exe`);
  } else {
    console.error(`[Build] Failed with code ${code}`);
  }
  process.exit(code);
});