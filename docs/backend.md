# backend

## 목적

백엔드는 Cohesion의 API 서버이자 파일 공유 런타임이다. 스페이스/파일 관리, 인증/권한, 감사 로그, 서버 설정, 상태 조회와 함께 WebDAV, FTP, SFTP 런타임을 제공한다.

## 기술 스택

- Go
- 표준 `net/http` 기반 라우팅
- SQLite
- pnpm workspace 스크립트는 빌드/개발 실행 보조용

## 현재 구조

```text
apps/backend/
├── config/                       # 환경별 설정 파일
├── main.go                       # 서버 조립, 서비스 부팅
├── build.js                      # 프론트 번들 포함 프로덕션 빌드
├── dev.js                        # 개발용 실행 보조
├── embed_dev.go
├── embed_prod.go
└── internal/
    ├── account/                  # 사용자/역할/권한 관리
    ├── audit/                    # 감사 로그 저장/조회/export/cleanup
    ├── auth/                     # 로그인, 세션, 권한 매핑
    ├── browse/                   # 시스템 디렉터리 탐색
    │   └── handler/              # /api/browse*
    ├── config/                   # /api/config
    ├── ftp/                      # FTP 런타임
    ├── platform/
    │   ├── database/             # DB 초기화/마이그레이션
    │   ├── logging/              # 운영 로그 포맷
    │   └── web/                  # 공용 HTTP 에러/핸들러 어댑터
    ├── sftp/                     # SFTP 런타임
    ├── spa/                      # SPA 정적 파일 서빙
    ├── space/                    # 스페이스, 파일 작업, 쿼터, 검색, 휴지통
    │   ├── handler/              # file dispatcher + upload/download/mutation/archive handlers
    │   └── store/
    ├── status/                   # /api/status
    ├── system/                   # 재시작/업데이트
    └── webdav/                   # WebDAV 런타임 및 핸들러
```

## 현재 지원 표면

- HTTP API
- WebDAV
- FTP
- SFTP

문서와 운영 가이드는 위 표면만 기준으로 유지한다. 제거되었거나 현재 저장소에 없는 프로토콜/스토리지 추상화는 활성 구조로 취급하지 않는다.

## 주요 책임 경계

- `account`
  - 사용자 CRUD
  - 역할/권한 정의
  - 사용자별 스페이스 권한 저장
- `auth`
  - 로그인/세션 사용자 조회
  - 요청 권한 매핑
  - denied 감사 정책 연결
- `space`
  - 스페이스 생성/삭제/이름 변경
  - 브라우징, 업로드, 이동/복사, 다운로드, 휴지통
  - 쿼터 계산
  - 파일 검색
- `audit`
  - 감사 이벤트 저장
  - 조회/export/cleanup API
- `config`, `system`, `status`
  - 서버 설정
  - 재시작/업데이트
  - 런타임 상태 조회
- `webdav`, `ftp`, `sftp`
  - 공유 프로토콜 런타임

## Space file handler 경계

- `internal/space/handler/file_handler.go`
  - `/api/spaces/{id}/files/{action}` dispatcher만 유지한다.
  - denied audit와 search-index dirty marking 같은 cross-cutting 후처리를 연결한다.
- `internal/space/handler/file_upload_handler.go`
  - multipart upload staging, conflict policy, quota reservation/finalize를 담당한다.
- `internal/space/handler/file_download_handler.go`
  - direct download, download ticket, multi-download ticket, ZIP streaming을 담당한다.
- `internal/space/handler/file_mutation_handler.go`
  - rename, create-folder, move/copy, trash lifecycle를 담당한다.
- `internal/space/handler/file_handler_shared.go`
  - path validation, quota invalidation, audit helper, search-index dirty marking, trash helper 같은 공통 로직만 둔다.
- `archive_download_job.go`, `download_ticket.go`
  - archive job/ticket 계약은 유지하고 file action handlers가 이를 조합한다.

## 실행 명령

```bash
# 전체 개발 서버
pnpm dev

# 백엔드만 개발 실행
cd apps/backend && go run .

# 백엔드 테스트
cd apps/backend && go test ./...

# 백엔드 빌드
cd apps/backend && pnpm build

# 릴리즈 설정 점검
pnpm release:check
```

## 설정/데이터 메모

- 환경 설정 파일: `apps/backend/config/config.dev.yaml`, `apps/backend/config/config.prod.yaml`
- 데이터베이스: SQLite
- 필수 secret bootstrap:
  - JWT secret은 `COHESION_JWT_SECRET` 또는 `COHESION_JWT_SECRET_FILE`로 공급한다.
  - 개발 환경은 secret file이 없으면 자동 생성할 수 있다.
  - 프로덕션은 env/file 중 하나로 secret을 제공해야 하고, 최종 secret 길이가 32자 미만이면 부팅이 실패한다.
  - SFTP host key는 `COHESION_SFTP_HOST_KEY_FILE` 우선, 그 외에는 사용자 config 또는 실행 파일 기준 `data/` 경로에서 prewarm 한다.
- 주요 런타임 설정:
  - HTTP 포트
  - WebDAV on/off
  - FTP on/off 및 포트
  - SFTP on/off 및 포트
  - 감사 로그 보존 일수

## Browse API 경계

- `/api/browse/base-directories`, `/api/browse?path=...`
  - 시스템 디렉터리 탐색 전용
  - Space 생성 흐름에서 사용
  - `space.write` 권한 필요
- `/api/spaces/validate-root`
  - Space 생성 직전 root 경로 검증 전용
  - `valid`, `not_found`, `not_directory`, `permission_denied` 결과를 구조화해 반환
  - `space.write` 권한 필요
- `/api/spaces/{id}/browse?path=...`
  - 선택한 스페이스 내부 탐색 전용
  - 스페이스 권한 기준으로 접근 제어

`/api/browse`를 스페이스 내부 탐색 대체 경로로 쓰지 않는다.

`POST /api/spaces`는 `ValidateSpaceRoot`와 동일한 검증을 다시 수행해 preflight를 우회한 broken Space 생성을 막는다. 이미 생성된 Space는 이후 OS 권한이 사라져도 레코드를 유지하고 browse 시점에 현재 접근 오류를 surface한다.

## 운영 로그

- 로그 파일은 항상 실행 바이너리 기준 `logs/` 아래에 생성된다.
- 운영 로그: `<executable-dir>/logs/app.log`
- 접근 로그: `<executable-dir>/logs/access.log`
- 업데이터 로그: `<executable-dir>/logs/updater.log`
- 기본 Air 개발 경로에서는 실행 파일이 `apps/backend/tmp/main.exe`이므로 로그 위치는 `apps/backend/tmp/logs/*`이다.

운영 상태/부팅/경고/오류는 운영 로그에서 보고, 요청 단위 추적은 접근 로그에서 본다.

```bash
# 라이프사이클 이벤트 추적
rg "event=(boot\\.|config\\.loaded|db\\.ready|service\\.ready|server\\.)" apps/backend/tmp/logs/app.log

# 오류 확인
rg "level=(ERROR|FATAL)" apps/backend/tmp/logs/app.log

# 접근 로그 확인
tail -f apps/backend/tmp/logs/access.log
```

## 검증 기준

백엔드 변경 후 기본 검증 경로:

```bash
cd apps/backend && go test ./...
pnpm release:check
```

프론트와 계약이 바뀌는 작업이면 프론트 타입체크/테스트도 함께 확인한다.

## 문서 유지보수 체크리스트

- `internal/*`의 안정적인 패키지 경계나 주요 책임이 바뀌면 `docs/backend.md`를 같이 갱신한다.
- 지원 프로토콜, 설정 필드, 로그 경로, 운영 명령이 바뀌면 "현재 지원 표면", "실행 명령", "설정/데이터 메모"를 다시 맞춘다.
- `/api/browse*`, `/api/spaces/*`, `/api/status`, `/api/config` 같은 운영 핵심 경계가 바뀌면 관련 API 설명을 검토한다.
