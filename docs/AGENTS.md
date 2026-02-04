# AI 에이전트를 위한 실행 환경 가이드

## 프로젝트 구조

이 프로젝트는 **Turbo 기반 Monorepo** 구조입니다.

```
Cohesion/
├── apps/
│   ├── backend/     # Go 백엔드 서버
│   └── frontend/    # React + Vite 프론트엔드
├── docs/
│   └── ai-context/  # AI 컨텍스트 파일 (필수!)
└── package.json     # 루트 패키지 (Turbo 설정)
```

## 서버 실행

### 개발 모드 (전체)
```bash
# 루트 디렉토리에서
npm run dev
```
- 프론트엔드: http://localhost:5173
- 백엔드: http://localhost:3000

### 개별 실행

**프론트엔드만:**
```bash
cd apps/frontend
npm run dev
# → http://localhost:5173
```

**백엔드만:**
```bash
cd apps/backend
go run .
# → http://localhost:3000
```

## 빌드

### 전체 빌드
```bash
npm run build
```

### 프론트엔드 빌드
```bash
cd apps/frontend
npm run build
# 결과: apps/frontend/dist/
```

### 백엔드 빌드
```bash
cd apps/backend
go build -o cohesion
# 결과: apps/backend/cohesion
```

## 포트 정보

| 서비스 | 포트 | 설명 |
|--------|------|------|
| 프론트엔드 | 5173 | Vite dev server |
| 백엔드 API | 3000 | Go Fiber server |

## 데이터베이스

- **SQLite** (로컬 파일)
- 위치: `apps/backend/cohesion.db`
- 자동 생성 및 마이그레이션

## 환경 변수

### 백엔드 (apps/backend/.env)
```
ENV=development
DB_PATH=./cohesion.db
PORT=3000
```

### 프론트엔드
- 환경 변수 불필요 (개발 시)
- API 프록시: Vite가 자동으로 `/api` → `http://localhost:3000` 프록시

## 테스트

### Playwright 브라우저 테스트
```bash
# Playwright MCP 툴 사용 (규칙에 따라)
# 1. 서버 시작
# 2. Playwright MCP 브라우저 테스트
# 3. 스크린샷 확인
# 4. 브라우저 종료
```

### 유닛 테스트
```bash
# 프론트엔드
cd apps/frontend
npm run test

# 백엔드
cd apps/backend
go test ./...
```

## 주의사항

### 규칙 준수
1. **항상 docs/master_rule.md를 먼저 읽기**
2. **Serena MCP 필수 사용** (코드 탐색/수정)
3. **Playwright MCP로 UI 테스트 필수**
4. **ai-context 파일 업데이트 필수** (작업 후)

### 디렉토리 이동 시
- 절대 경로 사용 권장: `/Users/twlee/projects/Cohesion/apps/backend`
- 상대 경로 사용 시 주의: 현재 위치 확인 후 이동

### 백그라운드 실행
```bash
# 개발 서버를 백그라운드로 실행할 때
npm run dev > /tmp/cohesion-dev.log 2>&1 &
# 또는
cd /Users/twlee/projects/Cohesion/apps/backend && go run . > /tmp/cohesion-backend.log 2>&1 &
```

## 트러블슈팅

### 포트 충돌
```bash
# 포트 사용 중인 프로세스 확인
lsof -ti:5173  # 프론트엔드
lsof -ti:3000  # 백엔드

# 프로세스 종료
kill -9 $(lsof -ti:5173)
```

### 빌드 에러
```bash
# 클린 빌드
npm run clean
npm install
npm run build
```

### 데이터베이스 리셋
```bash
cd apps/backend
rm cohesion.db
# 서버 재시작하면 자동 재생성
```

## 패키지 관리

- **루트**: npm (Turbo)
- **프론트엔드**: npm
- **백엔드**: go mod

### 의존성 추가

**프론트엔드:**
```bash
cd apps/frontend
npm install <package>
```

**백엔드:**
```bash
cd apps/backend
go get <package>
```
