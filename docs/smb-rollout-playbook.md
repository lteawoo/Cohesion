# SMB Rollout Playbook

## 목적

SMB를 기존 WEB/WebDAV/FTP/SFTP 운영 패턴과 일관되게 단계 도입하기 위한 로컬 운영 가이드입니다.

## 현재 단계 (중요)

- 본 문서는 `deliver-smb-readwrite-phased-rollout` 변경 기준으로 SMB를 **단계형(readonly -> write-safe -> write-full)**으로 운영하기 위한 절차를 다룹니다.
- `/api/status`의 SMB 상태는 포트 오픈 여부가 아니라 **프로토콜 준비 상태**를 기준으로 해석합니다.

### 단계별 범위

- `readonly`:
  - 제공: 접속(negotiate/session/tree), 디렉터리 조회(list), 파일 읽기(read)
  - 거부: write/create/rename/delete/mkdir/truncate
- `write-safe`:
  - 추가 제공: create/write/truncate
  - 거부 유지: rename/delete/mkdir
- `write-full`:
  - 추가 제공 목표: rename/delete/mkdir 포함 full write 연산
- 공통 권한 기준:
  - read 계열: Space `read`
  - write 계열: Space `write`
  - manage 계열: Space `manage`

### SMB 상태 해석 기준

- `healthy`: 바인드 + 프로토콜 런타임 준비가 완료되어 핸드셰이크 가능한 상태
- `unhealthy`: SMB는 활성화되어 있으나 bind/accept/session 중 일부 준비가 실패한 상태
- `unavailable`: `smb_enabled=false` 등으로 의도적으로 비활성화된 상태

### SMB 오류 분류(운영 메시지 taxonomy)

- `disabled`: 설정상 비활성화
- `bind_not_ready`: 포트 바인드 준비 실패 또는 미완료
- `accept_failed`: 연결 수락 단계 오류
- `runtime_not_ready`: 바인드는 됐지만 프로토콜 세션 처리 준비 미완료
- `runtime_error`: 런타임 내부 오류

### 상태 채널 분리 원칙

- 사용자 채널(상태 팝오버):
  - SMB는 `됨/안됨` 이진 상태로만 표시한다.
  - 운영 진단 필드(`reason`, `stage`, `bindReady`, `runtimeReady`, phase metadata)는 노출하지 않는다.
- 운영 채널(`/api/status`, `app.log`):
  - 진단 필드(`reason`, `stage`, taxonomy)는 유지한다.
  - 장애 분석은 운영 채널 기준으로 수행한다.

### 연산 허용/거부 매트릭스

| 연산 | readonly | write-safe | write-full | 비고 |
| --- | --- | --- | --- | --- |
| negotiate/session/tree | 허용 | 허용 | 허용 | 인증 성공 시 진행 |
| directory list | 허용 | 허용 | 허용 | Space `read` 이상 필요 |
| file read | 허용 | 허용 | 허용 | Space `read` 이상 필요 |
| create/write/truncate | 거부 | 허용 | 허용 | Space `write` 이상 필요 |
| rename/delete/mkdir | 거부 | 거부 | 허용(목표) | Space `manage` 이상 필요 |

### 거부 사유 taxonomy

- `readonly_phase_denied`: 현재 rollout phase에서 허용되지 않은 연산 시도
- `permission_denied`: Space 권한 부족
- `path_boundary_violation`: Space 경계/경로 이탈 시도
- `auth_failed`: 계정 인증 실패

### 권장 로그 필드

- 공통: `service=smb`, `port`, `endpoint_mode`, `min_version`, `max_version`
- 실패 시: `stage(bind|accept|session|stop)`, `reason`, `remote_addr(optional)`
- 민감정보(패스워드/토큰/credential material)는 기록하지 않음

### Secret Boundary 정책

- JWT 토큰 서명 키와 SMB material 암호화 키는 분리 운영한다.
  - JWT: `COHESION_JWT_SECRET` (또는 `COHESION_JWT_SECRET_FILE`)
  - SMB material: `COHESION_SMB_MATERIAL_KEY` (또는 `COHESION_SMB_MATERIAL_KEY_FILE`)
- startup prewarm에서 JWT/SMB/SFTP 키를 선확보한다(서비스 활성화 여부와 무관).
- key source 우선순위는 `env > persisted file > generated once`로 통일한다.
- 기존 SMB credential 데이터가 있는 상태에서 key가 없으면 자동 생성하지 않고 복구 필요 오류로 처리한다.
- SMB material 복호화는 active SMB key 단일 소스만 사용한다(legacy fallback/decrypt migration 미지원).

## 기본 정책

- 기본 비활성(`smb_enabled: false`)
- 엔드포인트/포트 정책
  - endpoint mode는 `direct`로 시스템 고정
  - SMB 포트는 `smb_port` 설정값 사용(미지정 시 기본 `445`)
- SMB 버전 정책
  - 협상 범위는 `2.1 ~ 3.1.1`로 시스템 고정
- SMB1 미지원
- Space 권한은 Cohesion 계정 권한(`read/write/manage`)을 기준으로 해석

## 엔드포인트 모드

- 현재 스펙은 `direct` 단일 모드입니다.
- 설정:
  - `smb_enabled: true`
  - `smb_port: <원하는 포트>` (미지정 시 `445`)
- 의미:
  - 프로세스가 설정된 SMB 포트를 직접 바인딩합니다.
- 주의:
  - OS 권한/방화벽/기존 SMB 서비스 점유 상태를 사전 확인해야 합니다.

## 검증 매트릭스 분리

### A) 코드 레벨 사전 검증 (integration-tag smoke)

- 목적: 런타임 기본 경로(connect/list/read)가 빌드 환경에서 회귀 없이 유지되는지 빠르게 확인
- 명령:
  - `cd apps/backend && go test -tags integration ./internal/smb -run TestSMBReadOnlySmoke_ConnectListRead`
- 산출물:
  - 테스트 로그(성공/실패), 실행 시점 commit SHA

### B) 실클라이언트 호환 검증 (native 3OS smoke)

- 목적: Linux/macOS/Windows 네이티브 SMB 클라이언트 상호운용성 확인
- 시나리오(각 OS 공통):
  1. SMB 공유 연결(negotiate/session/tree)
  2. 디렉터리 조회(list)
  3. 파일 읽기(read)
  4. write/create/rename/delete 시도 시 거부 확인(read-only)
- 증빙:
  - OS별 실행 로그/스크린샷 또는 터미널 출력
  - 사용 클라이언트 정보(버전), 대상 포트(`smb_port`)
  - 실패 시 `/api/status`의 `smb` reason/message, 서버 로그의 stage/reason

## 체크리스트

1. Settings > 서버에서 SMB 활성화, SMB 포트, rollout phase를 저장
2. 서버 재시작 후 `/api/status`에서 `smb`의 `rolloutPhase/policySource/bindReady/runtimeReady/message/reason`을 확인
3. 로컬 클라이언트에서 negotiate/auth/list/read 시나리오를 확인
4. `write-safe` 이상에서는 create/write/truncate 허용 여부를 검증
5. `readonly` 또는 `write-safe`에서 manage(rename/delete/mkdir) 연산이 `readonly_phase_denied`로 거부되는지 확인
6. Space 권한(`read/write/manage`) 기준 허용/거부 시나리오를 검증

## 롤백 절차

1. `server.smb_rollout_phase: readonly`로 저장
2. 서버 재시작
3. `/api/status`에서 다음을 확인:
   - `smb.rolloutPhase=readonly`
   - `smb.bindReady=true` (바인드 정상 시)
   - write/manage 연산 시 `readonly_phase_denied` 거부
4. 필요 시 `smb_enabled: false`로 추가 회귀하고 재시작
5. 기존 WEB/WebDAV/FTP/SFTP 경로 정상 동작 재확인
6. secret boundary 변경 직후 SMB 인증 실패가 증가하면:
   - `COHESION_SMB_MATERIAL_KEY`를 직전 운영 값으로 되돌리고 재시작
   - 필요 시 사용자 로그인/비밀번호 변경으로 SMB material 재준비를 유도

## 장애 시 대응

- direct 모드에서 기동 실패: 설정된 `smb_port` 점유/권한/방화벽 정책을 점검
- `/api/status`가 `runtime_not_ready`면 SMB 런타임 준비 상태를 먼저 복구하고 재시작
- write/manage 요청 거부는 rollout phase 기준 정상 동작일 수 있으므로 현재 `rolloutPhase`와 기대 시나리오를 우선 대조
- 권한 이상: 계정/Space 권한 매핑(`CanAccessSpaceByID`)과 사용자 권한 데이터 재검증

### SMB 로그 점검 예시

```bash
# SMB telemetry/런타임 이벤트 확인(필드 순서 무관)
rg "service=smb" logs/app.log | rg "stage=" | rg "reason="

# 정책 거부 taxonomy 확인(필드 순서 무관)
rg "service=smb" logs/app.log | rg "reason=(readonly_phase_denied|permission_denied|path_boundary_violation)"

# 런타임 세션 오류 확인
rg "service=smb" logs/app.log | rg "reason=runtime_error"
```
