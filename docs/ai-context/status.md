# 프로젝트 상태 (Status)

## 현재 진행 상황
- 서비스 범위 정리 완료:
  - 백엔드/프론트에서 현재 지원 프로토콜만 유지
  - 불필요한 런타임 경로 및 관련 의존성 정리
- 배포 파이프라인 정리 진행:
  - CI/Release 워크플로우에서 제거된 게이트 호출 제거
- 제품 백로그 정리 완료:
  - GitHub Issue 등록
  - #197 스페이스 관리 메타데이터 편집과 설정 화면 통합
  - #198 파일 검색 결과 품질과 성능 고도화
  - #199 감사 로그 운영 기능 보강
  - #200 내 프로필 수정과 비밀번호 변경 지원
  - #201 프론트엔드 핵심 운영 경로 회귀 테스트 보강
- OpenSpec 변경안 준비 완료:
  - `space-settings-management` 메인 spec 동기화 완료
  - `manage-space-settings` change archive 완료
- #197 스페이스 설정 관리 구현 완료:
  - 백엔드 `PATCH /api/spaces/{id}` rename 지원 추가
  - `Settings > Spaces`에서 이름 변경 + 기존 쿼터 관리 통합
  - shared Space list refresh와 browse selectedSpace ID 재동기화 적용
  - `space.write` 유지, 사이드바 생성 진입 유지, 설명 편집은 제외
  - 후속 조정 완료:
    - `space_desc` 제거 및 기존 SQLite 마이그레이션 추가
    - `Settings > Spaces` 설명을 `스페이스를 관리합니다.`로 통일
    - 이름/쿼터 수정과 저장/삭제를 한 테이블의 행 단위 액션으로 재구성
    - 기존 분리 섹션 기준 번역/테스트/잔존 코드 제거
    - 설정 상세 하단 잘림은 `settings-section` 하단 패딩 추가로 보정
  - 검증 완료:
    - `cd apps/backend && go test ./...`
    - `pnpm --dir apps/frontend typecheck`
    - `pnpm --dir apps/frontend test`
    - `pnpm release:check`
    - 수동 UI 확인: rename, quota update, read-only access, sidebar create entry
    - 수동 UI 확인: row action table screenshot
- #201 프론트엔드 핵심 운영 경로 회귀 테스트 보강 구현 완료:
  - `Login` 테스트 추가:
    - 로딩 상태
    - 인증된 사용자 리다이렉트
    - 초기 setup 플로우
    - 로그인 성공/유효성 검증
  - `SettingsPage` 테스트 추가:
    - 권한별 섹션 메뉴 노출/비노출
    - 허용된 섹션 클릭 시 콘텐츠 전환
  - `MainLayout` 헤더 검색 테스트 추가:
    - 연결된 스페이스 유무에 따른 검색 입력 제어
    - debounce 후 suggestion 로드
    - 검색 제출 시 `/search` 이동
    - suggestion 선택 시 browse 상태/이동 반영
  - `ServerSettings` 테스트 추가:
    - 설정 로드 실패
    - validation 오류 차단
    - 저장 성공
    - 재시작 confirm 후 reconnect 실패 피드백
  - 검증 완료:
    - `pnpm --dir apps/frontend typecheck`
    - `pnpm --dir apps/frontend test`

## 검증 상태
- 백엔드: `cd apps/backend && go test ./...` 통과
- 프론트엔드: 주요 빌드/타입 검증 경로 정상

## 다음 작업
1. #198 파일 검색 결과 품질과 성능 고도화
2. #200 내 프로필 수정과 비밀번호 변경 지원
3. #199 감사 로그 운영 기능 보강
4. 남은 운영 문서 정합성 점검
