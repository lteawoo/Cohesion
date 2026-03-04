# smbcore v1 Publication Guide

## 1) v1 Public API Surface

`apps/backend/pkg/smbcore`에서 아래 항목을 v1 호환 계약으로 취급한다.

- 타입/상수:
  - `Dialect`, `Dialect210|300|302|311`
  - `Permission`, `PermissionRead|Write|Manage`
  - `RolloutPhase`, `RolloutPhaseReadOnly|WriteSafe|WriteFull`
  - `DenyReasonReadonlyPhaseDenied|PermissionDenied|PathBoundary`
  - `ErrPermissionDenied|ErrPathBoundary|ErrReadonlyPhaseDenied`
  - `ErrRuntimeNotImplemented|ErrRuntimeDependenciesMissing|ErrInvalidDialectBounds`
- 인터페이스:
  - `Authenticator`, `Authorizer`, `FileSystem`, `Telemetry`, `Runtime`
- 구조체/생성자:
  - `DirEntry`, `Event`, `Config`, `Engine`, `NewEngine(...)`
- 메서드:
  - `Config.Validate()`
  - `Engine.HandleConn(...)`
  - `Engine.Supports(...)`
  - `Engine.IsReadOnly()`
  - `Engine.Phase()`
  - `Engine.CheckUsability(...)`

다음은 v1 호환 계약에서 제외한다.

- `pkg/smbcore`의 비공개 심볼(소문자 시작 함수/구조체/상수)
- 테스트 전용 헬퍼/테스트 빌드 경로

## 2) Semantic Versioning Policy

- 첫 공개 안정 버전은 `v1.0.0`으로 발행한다.
- 프리릴리즈는 `v1.0.0-rc.N` 형식을 사용한다.
- `PATCH` (`v1.0.x`):
  - 공개 API 시그니처/의미를 깨지 않는 버그 수정
  - 동작 안정성 개선, 문서 수정
- `MINOR` (`v1.x.0`):
  - 하위 호환을 유지하는 공개 API 추가
  - opt-in 기능 확장
- `MAJOR` (`v2.0.0+`):
  - 공개 API 제거/시그니처 변경/호환 불가 의미 변경
  - 기본 동작 계약(예: deny taxonomy, readiness 계약)의 비호환 변경

브레이킹 변경은 릴리즈 노트에 반드시 아래를 포함한다.

- 변경된 API 목록
- 영향 범위
- 이전 버전 대비 마이그레이션 절차

## 3) Release Notes / Migration Minimum

모든 릴리즈 노트는 최소 아래 섹션을 포함한다.

- `Summary`
- `Compatibility`
- `Changes`
- `Migration` (필요 시)
- `Rollback` (이전 버전 복귀 절차)

브레이킹 릴리즈의 `Migration`에는 아래가 필수다.

- 기존 호출 예시 -> 신규 호출 예시
- 설정/정책 차이
- 검증 명령(게이트 스크립트 포함)

## 4) Publication Gates and Evidence Format

발행 승인 전 아래 명령이 모두 통과해야 한다.

```bash
cd apps/backend && ./scripts/check-smb-publication-gates.sh
```

게이트 구성:

- `check-smbcore-boundary.sh`: 경계 누수(import) 검증
- `check-smb-compat-baseline.sh`: dialect/phase/taxonomy/readiness baseline
- `go test -tags integration ./internal/smb -run TestSMB`: SMB integration smoke

증빙은 `docs/releases/smbcore/<version>/evidence.md`에 보관한다.
포맷은 [evidence-template.md](/Users/twlee/projects/Cohesion/docs/releases/smbcore/evidence-template.md) 기준을 사용한다.

## 5) Cohesion External Module Transition Procedure

1. 전환 브랜치 생성 (`chore/smbcore-external-<version>`).
2. import 경로를 in-repo 경로에서 외부 모듈 경로로 일괄 전환:
   - `taeu.kr/cohesion/pkg/smbcore` -> `github.com/lteawoo/smb-core`
3. 버전 고정:
   - `cd apps/backend && go get github.com/lteawoo/smb-core@<version>`
   - `cd apps/backend && go mod tidy`
4. 게이트 검증:
   - `cd apps/backend && ./scripts/check-smb-publication-gates.sh`
   - `cd apps/backend && go test ./...`
5. 전환 증빙을 `docs/releases/smbcore/<version>/evidence.md`에 기록.

## 6) Deterministic Rollback Procedure

1. import 경로를 이전 in-repo 경로로 복구:
   - `github.com/lteawoo/smb-core` -> `taeu.kr/cohesion/pkg/smbcore`
2. `go.mod`/`go.sum`을 이전 known-good 커밋 기준으로 복구.
3. 검증 재실행:
   - `cd apps/backend && ./scripts/check-smb-publication-gates.sh`
   - `cd apps/backend && go test ./...`
4. rollback 증빙을 동일 evidence 문서에 추가 기록.

## 7) Parity Verification Checklist (Post-Transition / Post-Rollback)

- [ ] boundary guard 통과
- [ ] compatibility baseline 통과
- [ ] integration smoke 통과
- [ ] `go test ./...` 통과
- [ ] status/readiness 주요 시나리오 회귀 없음 확인
- [ ] deny taxonomy(`readonly_phase_denied`, `permission_denied`, `path_boundary_violation`) 회귀 없음 확인

## 8) Tag and Version Policy

- RC 태그: `smbcore-v1.0.0-rc.N`
- 안정 태그: `smbcore-v1.0.0`
- 이후 릴리즈:
  - patch: `smbcore-v1.0.X`
  - minor: `smbcore-v1.X.0`
- major 전환 시:
  - breaking migration 가이드 필수
  - 최소 1회 RC 라운드 필수

## 9) Publication Readiness Checklist

- [ ] public API surface 최신화
- [ ] semver 분류 검토 완료
- [ ] 게이트 스크립트 통과 증빙 첨부
- [ ] migration/rollback 문서 점검 완료
- [ ] release notes 초안 작성 완료
- [ ] evidence 문서 링크가 릴리즈 노트에 연결됨
