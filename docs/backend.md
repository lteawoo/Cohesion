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
- 서버 전송 상태:
  - ZIP 작업 상태와 업로드 용량 예약, 임시 파일 경로는 SQLite에 기록한다.
  - ZIP 파일은 프로덕션 `<state-root>/data/transfers/archives`, 개발 `dist/data/transfers/archives`에 보관한다. 프로세스 재기동 뒤에도 보존 기간이 남은 ZIP은 새 다운로드 요청으로 받을 수 있다.
  - 생성 중이던 ZIP 작업은 실패로 표시되며 전송 센터에서 재시도할 수 있다. 중단된 multipart 업로드의 용량 예약과 기록된 임시 파일도 정리한다. 접근할 수 없는 임시 경로는 다음 정리 주기에 다시 확인한다.
  - 이어올리기 원본은 전송 상태 디렉터리의 `uploads/`에 보존한다. 대상 폴더에 완료용 임시 파일을 복사한 뒤 이름을 바꾸므로 게시 시 파일 한 개 분량의 추가 공간이 필요하다. 서버가 재시작되면 세션과 용량 예약을 복원한다.
  - 마지막 성공 청크로부터 24시간이 지나면 세션이 만료된다. 정리 주기는 1분이다. 완료 결과는 24시간 동안 소유자가 다시 조회할 수 있다.

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

## 브라우저 업로드 세션 API

모든 요청은 스페이스 파일 쓰기 권한이 필요하다. 세션 ID는 생성 계정과 스페이스에 귀속된다.

| 요청 | 동작 |
| --- | --- |
| `POST /api/spaces/{spaceId}/files/upload-sessions` | JSON으로 `path`, `filename`, `size`, 선택적 `conflictPolicy`를 보내면 세션 ID와 수신 위치를 반환한다. |
| `GET …/upload-sessions?id={id}` | 수신 위치 또는 완료 결과를 조회한다. |
| `PATCH …/upload-sessions?id={id}` | `Upload-Offset` 헤더와 길이가 정해진 바이너리 본문을 보낸다. 최대 4 MiB이며 위치가 다르면 409를 반환한다. |
| `POST …/upload-sessions?id={id}` | 전체 수신을 확인하고 파일을 게시한다. 대상 이름 충돌 시 같은 세션에 `conflictPolicy=rename\|overwrite\|skip`을 지정해 재시도할 수 있다. |
| `DELETE …/upload-sessions?id={id}` | 수신을 취소하고 임시 파일과 예약을 회수한다. |

## 비로그인 파일 요청

쓰기 권한이 있는 계정은 `/api/spaces/{spaceId}/files/file-requests?folder={path}`에서 자신이 만든 활성 링크를 조회하거나 생성한다. 생성 본문은 `days`(1~365일)와 `byteLimit`(바이트)이다. 같은 경로에 `DELETE`와 `id`를 보내면 링크가 닫힌다.

공개 요청은 `/api/file-drop`에 `X-File-Request`를 보낸다. GET은 제목과 만료 시각을 반환한다. 제출자는 독립적인 24바이트 난수를 base64url로 인코딩해 `X-Submission-Key`에 추가한다. POST는 `filename`과 `size`로 제출을 시작하거나 복구한다. GET은 수신 위치, PATCH는 4 MiB 이하의 청크와 `Upload-Offset`, PUT은 완료, DELETE는 취소에 사용한다.

링크마다 수신한 바이트와 진행 중인 예약량을 합산한다. 동시 제출은 최대 32개, 완료 파일은 링크당 최대 10,000개다. 스페이스 할당량도 적용한다. 완료 시 생성자의 쓰기 권한과 만료·폐쇄 상태를 재확인한다. 최종 파일은 대상 폴더 안의 준비 파일에 하드 링크를 만들어 게시하므로 하드 링크를 지원하는 볼륨이 필요하다.

## 비로그인 다운로드 링크

스페이스 `manage`와 전역 `file.read` 권한을 갖춘 계정이 다운로드 링크를 관리한다. `GET /api/spaces`의 `can_manage_download_links`는 로그인 계정이 해당 스페이스에서 링크를 관리할 수 있는지 나타낸다.

| 요청 | 동작 |
| --- | --- |
| `POST /api/spaces/{id}/files/download-links` | 스페이스 기준 파일 경로 `path`와 유효 일수 `days`(1~365)를 보내면 생성 시점의 사본과 링크를 만든다. |
| `GET /api/spaces/{id}/files/download-links` | 공간의 활성 링크를 조회한다. 선택적으로 `path`로 파일을 지정한다. |
| `DELETE /api/spaces/{id}/files/download-links?id={linkId}` | 관리자가 링크를 닫는다. 원본을 옮기거나 삭제한 뒤에도 사용할 수 있다. |
| `GET /api/shared-file` | `X-Download-Link` 헤더로 공유한 파일의 이름·크기·만료 시각을 조회한다. |
| `POST /api/shared-file` | 폼 본문의 `token`으로 사본을 다운로드한다. 응답은 첨부 파일로 전송한다. |

링크마다 사본 크기만큼 서버 저장 공간이 필요하다. 스페이스에 할당량이 있으면 활성 사본과 생성 중인 사본의 합에도 같은 크기의 별도 한도가 적용된다. 이 한도나 사본 볼륨의 여유 용량을 초과하면 서버가 507을 반환한다. 서버는 사본을 최대 2개까지 동시에 준비하고, 준비 슬롯이 모두 사용 중이면 새 생성 요청에 429를 반환한다. 서버가 재시작되어도 사본과 링크가 유지된다. 새 요청마다 링크 상태와 발급자의 현재 권한을 확인한다. 원본 수정이나 삭제로는 공유가 종료되지 않으므로 공개를 끝낼 때는 링크를 닫는다.

사본은 전송 상태 루트의 `download-links/`에 보관한다. 서버 상태 경로와 스페이스를 별도 디렉터리로 구성한다. 이미 등록된 스페이스가 사본 보관 경로와 겹치면 새 링크 생성 요청은 409를 반환한다. 링크 폐쇄 후 즉시 사본 제거를 시도하며, 기동 시와 1분 주기로 만료된 사본과 정리가 남은 파일을 회수한다.

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
