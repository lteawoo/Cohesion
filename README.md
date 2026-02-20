# Cohesion

Cohesion은 쉽고 단순하게 내 PC나 서버의 파일을 관리하고 공유하기 위한 self-hosted 파일 서비스입니다.

## 소개

이 프로젝트는 다음 목표를 기준으로 개발됩니다.

- 설치와 실행 절차를 단순하게 유지
- 브라우저에서 바로 파일을 탐색하고 관리
- WebDAV, SFTP 등 기존 클라이언트와 연동
- 계정/역할/권한 기반으로 접근 제어

## 핵심 기능

- Space 기반 가상 루트
- 파일 탐색기 Grid/Table 뷰
- 업로드, 다운로드, 복사, 이동, 삭제, 이름 변경, 폴더 생성
- 다중 선택 및 다중 다운로드(ZIP)
- 이미지 썸네일, 확장자 기반 파일 아이콘
- WebDAV Basic Auth + Space 권한 연동
- SFTP 서버(옵션) + 계정 인증/권한 연동
- JWT 쿠키 인증 및 초기 관리자 Setup 플로우
- 계정/역할/권한(RBAC) 관리 UI

## 지원 프로토콜

- WEB: `http://<host>:<port>` (UI + API)
- WebDAV: `http://<host>:<port>/dav`
- SFTP: `<host>:<sftp_port>` (활성화 시)

## 기술 스택

- Backend: Go (`net/http`, SQLite, WebDAV, SFTP)
- Frontend: React 19, Vite, Ant Design, Zustand
- Monorepo: Turborepo, pnpm workspace
- 배포 빌드: GoReleaser

## 빠른 시작

### 요구사항

- Node.js `>= 24`
- pnpm `>= 10.24.0`
- Go `1.25.7`

### 설치

```bash
pnpm install
```

### 개발 실행

```bash
pnpm dev
```

- Frontend: `http://localhost:5173`
- Backend: `http://localhost:3000`
- SFTP 기본 포트: `2222` (활성화 시)

## Build

### 워크스페이스 전체 빌드

```bash
pnpm build
```

### 개별 빌드

```bash
# frontend
pnpm -C apps/frontend build

# backend
cd apps/backend && go build -o cohesion
```

### 배포 빌드(아티팩트)

```bash
pnpm release:check
pnpm release:snapshot
```

## 환경 변수

- `COHESION_JWT_SECRET`
  - 운영 환경에서 32자 이상 권장
  - 미지정 시 로컬 시크릿 파일에 랜덤 값 생성
- `COHESION_JWT_SECRET_FILE` (선택)
- `COHESION_ADMIN_USER`, `COHESION_ADMIN_PASSWORD`, `COHESION_ADMIN_NICKNAME` (선택)
  - `COHESION_ADMIN_USER`/`COHESION_ADMIN_PASSWORD`는 함께 지정해야 함
- `COHESION_SFTP_HOST_KEY_FILE` (선택)

## 보안 권장사항

- 기본 통신은 HTTP입니다. 인터넷 공개 시 리버스 프록시(Caddy/Nginx) + TLS 구성을 권장합니다.
- WebDAV는 Basic Auth와 Space 권한 체크를 함께 적용합니다.

## 라이선스

이 프로젝트는 `GNU Affero General Public License v3.0 (AGPL-3.0-only)`를 따릅니다.

- 전문: `LICENSE`
- 원문 참고: https://www.gnu.org/licenses/agpl-3.0.html
