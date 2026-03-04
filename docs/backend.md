# backend

## 폴더 구조
```
apps/backend/
├── .air.toml               # Live-reloading 설정
├── .gitignore
├── build.js                # 프로덕션 빌드 스크립트
├── dev.js                  # 개발 서버 실행 스크립트
├── embed_dev.go
├── embed_prod.go
├── go.mod
├── go.sum
├── main.go
├── package.json            # pnpm & turbo 모노레포 구성용
│
├── config/                 # 환경별 설정 파일
│   ├── config.dev.yaml
│   └── config.prod.yaml
│
├── internal/
│   │
│   ├── config/             # 설정 로딩 및 관리
│   │   ├── config_model.go
│   │   └── handler.go
│   │
│   ├── webdav/             # WebDAV 서버
│   │   └── server.go       #    - WebDAV 요청을 받아 핵심 서비스 호출
│   │
│   ├── sftp/               # SFTP 서버
│   │   └── service.go      #    - SFTP 요청을 받아 핵심 서비스 호출
│   │
│   ├── file/               # 파일 도메인 로직
│   │   ├── api.go
│   │   ├── file.go
│   │   └── service.go
│   │
│   ├── share/              # 폴더 공유 도메인 직
│   │   ├── api.go
│   │   ├── share.go
│   │   └── service.go
│   │
│   ├── user/               # 유저 도메인 로직
│   │   ├── api.go
│   │   ├── user.go
│   │   └── service.go
│   │
│   ├── storage/            # 실제 파일 저장소 (추상화 계층)
│   │   ├── interface.go
│   │   ├── filesystem.go
│   │   └── s3.go
│   │
│   ├── platform/           # 공통 기반 기술
│   │   ├── database/
│   │   │   └── db.go       # DB 초기화
│   │   └── auth/           #    - 인증/인가 로직
│   │
│   └── spa/                # SPA 프론트엔드 서빙 핸들러
│       └── handler.go
```

## 실행 구조

백엔드 앱은 개발 시와 프로덕션(서비스) 시의 빌드 및 실행 방식에 차이가 있음. 설정 관리는 `Viper` 라이브러리를 사용함.

### 공통: `Viper`로 설정 파일 읽기

어떤 환경이든 앱 시작 시 `internal/config/handler.go`의 `SetConfig(goEnv)` 함수가 먼저 호출됨.

1.  이 함수는 `goEnv` 값('development' 또는 'production')에 따라 `config.dev.yaml` 또는 `config.prod.yaml` 파일을 읽도록 `viper`를 세팅해야 함.
2.  `viper`는 `config/` 폴더 안에서 설정 파일을 찾아 읽음.
3.  읽어온 설정(서버 포트, DB 정보 등)은 `Conf` 전역 변수에 저장됨. 이를 통해 앱 어디서든 `config.Conf.Server.Port`처럼 쉽게 설정 값을 사용 가능함.

---

### 1. 개발 환경에서 실행

1.  **시작**: `dev.js` 스크립트가 `air` 라이브 리로더를 실행함. `air`는 코드 변경 시 자동으로 재빌드 및 재시작을 수행함.
2.  **빌드**: `air`는 `.air.toml` 설정에 따라 `go build -o ./tmp/main.exe .` 명령으로 앱을 컴파일함.
    -   `goEnv` 변수는 코드 상의 기본값인 `"development"`를 유지함.
    -   프론트엔드 파일은 바이너리에 포함되지 않음.
3.  **실행**: 컴파일된 `tmp/main.exe`를 실행함.
4.  **서버 동작**:
    -   `SetConfig("development")`가 호출되어 `viper`가 `config.dev.yaml` 파일을 로드함.
    -   개발용 설정(포트, DB 정보)으로 서버가 동작함.
    -   서버는 API 요청(`_API 경로`)만 처리. 프론트엔드 UI는 `vite` 개발 서버가 별도로 담당함.

---

### 2. 프로덕션 환경에서 실행

1.  **시작**: `build.js` 스크립트를 실행해야 함.
2.  **프론트엔드 통합**: `frontend/dist`의 빌드 결과물을 `backend/dist/web`으로 복사함.
3.  **빌드**: `go build` 시 아래의 특정 플래그를 사용해야 함.
    -   `-ldflags "-X main.goEnv=production"`: 이 플래그로 `goEnv` 변수 값을 `"production"`으로 강제 변경하여 컴파일함.
    -   `-tags=production`: 이 태그로 `embed_prod.go`를 빌드에 포함시켜, `dist/web` 폴더의 프론트엔드 파일들을 바이너리 안에 내장시킴.
4.  **결과물**: API 서버와 프론트엔드 UI가 통합된 단일 실행 파일 `dist/main.exe`가 생성됨.
5.  **서버 동작** (`dist/main.exe` 실행 시):
    -   `SetConfig("production")`이 호출되어 `viper`가 `config.prod.yaml` 파일을 로드함.
    -   프로덕션용 설정으로 서버가 동작함.
    -   이 서버 하나가 API 요청(`_API 경로`)과 웹 UI(`/` 경로)를 모두 처리하는 독립 앱으로 기능함.

---

### 3. JWT/SMB Secret Boundary

- JWT 서명 키와 SMB material 암호화 키를 분리 운영함.
  - JWT: `COHESION_JWT_SECRET` 또는 `COHESION_JWT_SECRET_FILE`
  - SMB material: `COHESION_SMB_MATERIAL_KEY`
- 프로덕션에서는 JWT 비밀이 누락되면 자동 생성하지 않고 기동 실패로 처리함.
- 프로덕션에서 SMB가 활성화(`smb_enabled=true`)된 경우 `COHESION_SMB_MATERIAL_KEY` 누락 시 기동 실패로 처리함.
- 개발/테스트에서는 SMB material 키 미설정 시 개발용 fallback 키를 허용함.
- legacy SMB material 호환을 위해 복호화 시 JWT 기반 legacy key를 제한적으로 시도하고, 성공 시 현재 SMB key 기준으로 재암호화함.

---

## 운영 로그 가이드 (Operational Logging)

### 로그 채널

- 운영 로그(Operational): 서비스 상태/라이프사이클/경고/오류
- 접근 로그(Access): HTTP 요청/응답 메타데이터

### 로그 파일 위치

백엔드 실행파일 기준 `logs/` 디렉토리에 기록됨.

- `logs/app.log`: 운영 로그 전체
- `logs/access.log`: HTTP access 로그 전용
- `logs/updater.log`: 업데이터 실행 로그

### 터미널 출력 정책

- 터미널에는 운영 상태 확인에 필요한 이벤트만 출력됨.
- `INFO`: 필수 라이프사이클 이벤트(`boot.*`, `config.loaded`, `db.ready`, `service.ready`, `server.*`)
- `WARN/ERROR/FATAL`: 항상 출력
- `http.access` 이벤트는 터미널에 출력하지 않고 `logs/access.log`로만 기록

### 로그 형식

- 터미널(운영 로그): log4j2 유사 패턴 형식
  - `<ts> <LEVEL> [<component>] <event> - <msg> <extras>`
  - 예시:
    - `2026-03-03T01:34:32+09:00 INFO [server] server.ready - server ready port=38080`
    - `2026-03-03T01:34:33+09:00 INFO [server] server.shutdown_signal - shutdown requested source=signal`
- 파일(`logs/app.log`, `logs/access.log`, `logs/updater.log`): 영문 `key=value` 단일 라인 형식 유지
  - 공통 핵심 필드: `ts`, `level`, `event`, `component`, `msg`
  - Access 추가 필드(선택): `query`
  - 예시:
    - `ts=2026-03-03T01:34:32+09:00 level=INFO event=server.ready component=server msg=\"server ready\" port=38080`
    - `ts=2026-03-03T01:34:33+09:00 level=INFO event=http.access component=access msg=\"http request served\" method=GET path=/api/search query=\"q=hello&sort=asc\" status=200`

### 트러블슈팅 명령

```bash
# 운영 라이프사이클 이벤트 추적 (파일 기준)
rg "event=(boot\\.|config\\.loaded|db\\.ready|service\\.ready|server\\.)" logs/app.log

# 오류/치명 로그 확인 (파일 기준)
rg "level=(ERROR|FATAL)" logs/app.log

# Access 로그 실시간 확인
tail -f logs/access.log

# 특정 API 요청 필터링 (예: restart)
rg "event=http\\.access .*path=/api/system/restart" logs/access.log

# updater 런타임 오류 확인
rg "event=error\\.updater\\." logs/updater.log
```

### 운영자 확인 순서

- 터미널에서 현재 상태를 빠르게 확인한다. (부팅/재시작/종료, WARN/ERROR/FATAL)
- 상세 원인 분석이나 자동화 필터링은 파일 로그(`app.log`, `access.log`, `updater.log`)에서 수행한다.
- `http.access`는 터미널에 출력되지 않으므로, 요청 단위 추적은 반드시 `logs/access.log`를 사용한다.

## Browse API 역할 경계

- `/api/browse/base-directories`, `/api/browse?path=...`
  - 목적: Space 생성 모달에서 시스템 디렉토리 탐색
  - 권한: `space.write` 필요
- `/api/spaces/{id}/browse?path=...`
  - 목적: 선택된 Space 내부 디렉토리 탐색(일반 파일 브라우저)
  - 권한: Space 멤버십 기반 `read/write` 권한 검증

정책적으로 `/api/browse`는 시스템 탐색 전용이며, Space 내부 탐색 대체 경로로 사용하지 않는다.

## 감사로그 수동 점검 가이드 (실패/denied)

`Settings > Audit Logs`에서 아래 순서로 점검한다.

1. **실패 이벤트 추적 (`result=failure`)**
   - 결과 필터를 `failure`로 설정하고 조회한다.
   - 행을 선택해 상세 metadata의 `reason`/`code`를 확인한다.
   - 예: `config.update` 실패 시 `reason=validation_failed` 또는 `reason=save_failed`.

2. **권한 거부 이벤트 추적 (`result=denied`)**
   - 결과 필터를 `denied`로 설정하고 조회한다.
   - 대상(`target`)과 사용자(`actor`)를 기준으로 반복 거부 패턴을 확인한다.
   - `metadata.reason` / `metadata.code`로 거부 원인을 분류한다.
   - denied 이벤트가 없는 경우는 현재 관측 기간에 정책 대상 거부 이벤트가 발생하지 않은 상태다.

3. **denied 수집 정책 확인 (대상/제외)**

| 구분 | 경로/영역 | 감사로그 |
|------|-----------|----------|
| 포함 | 고위험 read (`GET /api/accounts`, `GET /api/roles`, `GET /api/permissions`, `GET /api/config`) | `result=denied` 기록 |
| 포함 | 계정/권한/역할 변경 (`POST /api/accounts`, `PATCH/DELETE /api/accounts/{id}`, `PUT /api/accounts/{id}/permissions`, `POST /api/roles`, `DELETE /api/roles/{name}`, `PUT /api/roles/{name}/permissions`) | `result=denied` 기록 |
| 포함 | 감사로그 조회 접근 (`/api/audit/logs*`) | `result=denied` 기록 |
| 포함 | 서버 제어 변경 (`PUT /api/config`, `/api/system/restart`, `/api/system/update/start`) | `result=denied` 기록 |
| 포함 | Space/파일 변경 및 다운로드 (`/api/spaces/{id}/files/{download,download-ticket,rename,delete,delete-multiple,create-folder,upload,move,copy,download-multiple,download-multiple-ticket}`, `/api/downloads/{ticket}`) | `result=denied` 기록 |
| 제외 | Space 휴지통 경로 (`/api/spaces/{id}/files/{trash,trash-restore,trash-delete,trash-empty}`) | denied 감사 미기록 (access 로그로 확인) |
| 제외 | 조회성 경로 (`/api/browse*`, `/api/spaces/{id}/browse`, `/api/search/files`) | denied 감사 미기록 (access 로그로 확인) |

4. **API 직접 확인(운영 점검/자동화용)**

```bash
# 실패 이벤트 최근 20건
curl -sS --cookie "cohesion_access=<ACCESS_COOKIE>" \
  "http://localhost:3000/api/audit/logs?page=1&pageSize=20&result=failure"

# denied 이벤트 최근 20건
curl -sS --cookie "cohesion_access=<ACCESS_COOKIE>" \
  "http://localhost:3000/api/audit/logs?page=1&pageSize=20&result=denied"
```
