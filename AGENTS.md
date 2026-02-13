**필수적으로 따라야함**
1. docs/master_rule_v2.md (작업 규칙)

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
pnpm dev
```
- 프론트엔드: http://localhost:5173
- 백엔드: http://localhost:3000
- **Hot Reload**: 코드 변경 시 자동 반영
  - 프론트엔드: Vite HMR (즉시 반영)
  - 백엔드: Air를 통한 자동 재빌드 및 재시작

### 개발 서버 동작

**자동 반영 (Hot Reload)**
- 파일 저장 시 자동으로 변경사항이 반영됩니다.
- **프론트엔드**: Vite의 HMR(Hot Module Replacement)로 페이지 새로고침 없이 즉시 반영.
- **백엔드**: Air를 통해 Go 파일 변경 감지 → 자동 재빌드 → 서버 재시작.
- 수동 재시작 불필요: 코드를 수정하고 저장하기만 하면 됩니다.

**주의사항**
- 백엔드 재시작 시 잠깐 서버 연결이 끊길 수 있습니다 (1-2초).
- 환경 변수 변경 시에는 수동으로 서버를 재시작해야 합니다.

### 개별 실행

**프론트엔드만:**
```bash
cd apps/frontend
pnpm dev
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
pnpm build
```

### 프론트엔드 빌드
```bash
cd apps/frontend
pnpm build
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

### 백그라운드 실행
일반적으로 개발 서버는 터미널에서 직접 실행하는 것을 권장합니다.
Hot Reload 로그를 실시간으로 확인할 수 있어 디버깅에 유리합니다.

필요한 경우 백그라운드로 실행:
```bash
# 전체 개발 서버
pnpm dev > /tmp/cohesion-dev.log 2>&1 &

# 백엔드만 (권장하지 않음 - Air의 Hot Reload 로그를 확인할 수 없음)
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
pnpm clean
pnpm install
pnpm build
```

### 데이터베이스 리셋
```bash
cd apps/backend
rm cohesion.db
# 서버 재시작하면 자동 재생성
```

## 패키지 관리

- **루트**: pnpm (Turbo)
- **프론트엔드**: pnpm
- **백엔드**: go mod

### 의존성 추가

**프론트엔드:**
```bash
cd apps/frontend
pnpm add <package>
```

**백엔드:**
```bash
cd apps/backend
go get <package>
```

**루트 워크스페이스:**
```bash
# 루트 디렉토리에서
pnpm add -w <package>
```
