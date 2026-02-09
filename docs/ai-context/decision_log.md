# 의사결정 로그 (Decision Log)

## 아키텍처 (Architecture)
... (기존 내용 유지) ...

## 파일 시스템 브라우징
### 시스템 디렉토리 필터링 (2026-02-03)
... (기존 내용 유지) ...

### GUI 탐색기 설계 (2026-02-03)
- **결정**: Google Drive와 유사한 사용자 경험을 위해 리스트 대신 `Table`과 `Breadcrumb`을 조합한 탐색기 구조 채택.
- **이유**: 단순 리스트보다 많은 정보(크기, 수정일)를 한눈에 보여줄 수 있으며, 복잡한 경로 탐색 시 Breadcrumb이 필수적임.
- **기술 선택**: Ant Design `Table`의 `onRow` 속성을 이용한 더블 클릭 진입 처리, `filepath.Clean` 기반의 경로 상태 관리.

## 데이터 모델
### FileInfo 확장 (2026-02-03)
- **결정**: `FileInfo` 구조체에 `Size` (int64) 및 `ModTime` (time.Time) 추가.
- **이유**: 파일 탐색기의 기본 기능인 '상세 보기' 및 '정렬' 기능을 지원하기 위함.

## Space와 파일 브라우저 연동
### Space 선택 시 자동 경로 이동 (2026-02-04)
- **결정**: Space 메뉴 클릭 시 해당 Space의 `space_path`를 파일 브라우저에 전달하여 자동 이동.
- **이유**: 사용자가 Space를 클릭했을 때 루트 디렉토리가 아닌 Space의 실제 경로에서 탐색을 시작해야 직관적임.
- **구현 방식**:
  - `MainLayout`에서 `selectedSpace`, `selectedPath` 상태 관리 및 `Outlet` context로 전달.
  - `FileExplorer`가 `useOutletContext`로 경로 정보 및 변경 핸들러를 받음.
  - `FolderTree`에 `rootPath` prop 추가: Space 경로가 있으면 해당 경로부터 트리 시작, 없으면 기존처럼 base directories 로드.
- **수정 파일**: `MainLayout/index.tsx`, `MainSider.tsx`, `FileExplorer.tsx`, `FolderTree.tsx`

### UI 레이아웃 통합 (2026-02-04)
- **결정**: `FolderTree`를 `FileExplorer`에서 분리하여 좌측 사이드바(`MainSider`)로 이동.
- **이유**: Space 목록과 FolderTree를 분리할 필요 없이, 좌측 사이드바에서 Space 선택 → FolderTree 탐색이 자연스러운 흐름.
- **레이아웃 변경**:
  - **이전**: 좌측 사이드바(Space 목록) / 중간(FolderTree) / 우측(FolderContent)
  - **이후**: 좌측 사이드바(Space 목록 + FolderTree) / 우측(FolderContent 전체)
- **장점**: 화면 공간 효율적 사용, 직관적인 계층 구조 표시.

### FolderTree 기본 동작 변경 (2026-02-04)
- **결정**: Space가 선택되지 않은 상태에서는 FolderTree가 아무것도 표시하지 않음.
- **이유**: 시스템 루트 디렉토리를 기본으로 표시할 필요 없음. Space 중심의 워크플로우를 강조.
- **구현**: `rootPath`가 없으면 빈 상태 메시지("Space를 선택하세요") 표시, base directories 자동 로드 제거.

### Space와 FolderTree 완전 통합 (2026-02-04)
- **결정**: Space 메뉴와 FolderTree를 하나의 통합된 트리 구조로 병합.
- **이유**:
  - Space 목록과 폴더 탐색을 별도로 보여줄 필요 없음.
  - 단일 계층 구조로 UX 단순화 및 일관성 향상.
  - 모든 Space를 한눈에 보면서 각 Space 내부를 탐색 가능.
- **구현**:
  - `FolderTree`에 `spaces` prop 추가.
  - Spaces를 `space-{id}` 형태의 key를 가진 루트 노드로 생성.
  - Space 노드 확장 시 `space_path`로부터 하위 디렉토리 lazy loading.
  - `MainSider`에서 Menu 제거, Spaces 헤더 + FolderTree만 표시.
  - Space 선택 시 `space_path` 반환하여 FolderContent 업데이트.
- **특수 케이스**: `showBaseDirectories` 플래그로 모달에서는 시스템 디렉토리 탐색 가능.

## 개발 프로세스
### Serena MCP 필수 사용 (2026-02-04)
- **결정**: 모든 코드 탐색 및 수정 작업에서 Serena MCP 툴을 필수로 사용.
- **이유**:
  - 토큰 효율성: 파일 전체가 아닌 필요한 심볼만 읽어 토큰 절약.
  - 정확성: 심볼 단위 수정으로 실수 방지.
  - 일관성: 모든 AI 모델이 동일한 방식으로 코드 작업.
- **금지**: `Read`/`Edit` 툴로 코드 파일 읽기/수정.
- **필수**: `get_symbols_overview` → `find_symbol` → `replace_symbol_body` 워크플로우.
- **문서화**: `CLAUDE.md`에 상세 가이드 추가.

### Playwright MCP 브라우저 테스트 필수화 (2026-02-04)
- **결정**: UI 수정 시 Playwright MCP로 브라우저 직접 테스트 필수.
- **이유**: 스크린샷으로 UI 직접 확인하여 디자인 적합성 판단.
- **절차**: 수정 → 테스트 → 스크린샷 → 브라우저 종료.
- **로그 위치**: `.playwright-mcp/` (gitignore 처리됨).

### 실행 환경 문서화 (2026-02-04)
- **결정**: AI 에이전트를 위한 실행 환경 정보를 별도 문서로 분리.
- **이유**:
  - 서버 실행, 빌드, 포트 등 실행 관련 정보가 분산되어 있음.
  - AI가 매번 프로젝트 구조를 탐색하는 비효율 방지.
  - 환경 설정 변경 시 한 곳만 수정하면 됨.
- **구현**:
  - `docs/AGENTS.md` 생성: 프로젝트 구조, 서버 실행, 빌드, 포트, 테스트 등 통합.
  - CLAUDE.md, GEMINI.md에 AGENTS.md 읽기 지시 추가.

### Space 상대 경로 Breadcrumb (2026-02-04)
- **결정**: Space 선택 시 Breadcrumb을 절대 경로 대신 Space 기준 상대 경로로 표시.
- **이유**:
  - 사용자가 Space 내에서의 위치를 직관적으로 파악 가능.
  - 긴 절대 경로(/Users/...) 대신 간결한 경로 표시로 UX 개선.
  - Space 중심 워크플로우 강화.
- **구현**:
  - FolderTree의 onSelect 콜백에 Space 정보 추가.
  - MainLayout에서 selectedSpace state 관리.
  - FolderContent에서 Space 경로를 기준으로 상대 경로 계산.
  - 예: `/Users/twlee/Downloads/folder1` → `Downloads / folder1`
- **수정 파일**: FolderTree.tsx, MainLayout/index.tsx, MainSider.tsx, FileExplorer.tsx, FolderContent.tsx

### 문서 구조 통합 (2026-02-04)
- **결정**: `docs/master_rule.md`를 모든 규칙의 단일 소스로 통합.
- **이유**:
  - 규칙 분산(CLAUDE.md, GEMINI.md, master_rule.md)으로 인한 혼란 방지.
  - AI 모델 간 일관성 확보: 모든 모델이 동일한 규칙 참조.
  - 유지보수 단순화: 한 곳만 수정하면 됨.
- **변경 사항**:
  - `GEMINI.md`: "master_rule.md를 먼저 읽기"만 남김.
  - `CLAUDE.md`: `.claude/CLAUDE.md`로 이동 (Claude Code CLI 전용).
  - `master_rule.md`: Serena MCP, Playwright, 디자인, 커밋 규칙 모두 포함.

### Space 삭제 기능 Context Menu 구현 (2026-02-04)
- **결정**: Space 노드에 Context Menu 방식의 삭제 기능 구현.
- **이유**:
  - 직관적인 UX: 우클릭 대신 "..." 버튼 클릭으로 메뉴 표시.
  - 일관성: Ant Design Tree와 Dropdown 컴포넌트 활용.
  - 안전성: Modal.confirm으로 삭제 확인 절차 추가.
- **구현**:
  - FolderTree: `titleRender`로 Space 노드에만 Dropdown 추가.
  - MainSider: `useDeleteSpace` 훅으로 DELETE API 호출.
  - 삭제 후 `refetch()`로 트리 자동 갱신.
  - 성공/실패 시 message 컴포넌트로 사용자에게 피드백.
- **백엔드**: DELETE `/api/spaces/:id` 엔드포인트는 이미 구현되어 있었음.

### 파일 표시 버그 수정 (2026-02-04)
- **문제**: FolderContent에서 폴더만 표시되고 파일이 표시되지 않는 버그 발견.
- **원인 분석**:
  - `browse_handler.go`의 `handleBrowse` 함수에서 `ListDirectory(true, targetPath)` 호출.
  - `isOnlyDir=true` 파라미터로 인해 `browse/service.go`의 `ListDirectory` 함수가 파일을 필터링.
  - 92-94라인: `if isOnlyDir && !entry.IsDir() { continue }` 로직으로 파일 제외.
- **결정**: `isOnlyDir` 파라미터를 `false`로 변경하여 파일과 폴더 모두 반환.
- **이유**:
  - 파일 탐색기의 핵심 기능은 파일과 폴더를 모두 보여주는 것.
  - FolderContent는 이미 파일과 폴더를 구분하여 표시하는 UI가 구현되어 있음.
  - 왼쪽 FolderTree는 폴더만 표시하고, 오른쪽 FolderContent는 파일과 폴더 모두 표시하는 것이 직관적.
- **수정 파일**: `apps/backend/internal/browse/handler/browse_handler.go:51`
- **결과**: Space 선택 시 폴더와 파일이 모두 정상 표시됨.

### 파일 다운로드 기능 구현 (2026-02-04)
- **결정**: 파일 클릭 시 브라우저의 기본 다운로드 기능을 사용하여 파일을 다운로드.
- **이유**:
  - 단순하고 직관적인 UX: 파일 이름을 클릭하면 바로 다운로드.
  - 브라우저 내장 다운로드 관리 활용: 사용자가 익숙한 방식.
  - 폴더와 파일 동작 구분: 폴더는 클릭 시 이동, 파일은 클릭 시 다운로드.
- **구현**:
  - **백엔드**: `handleDownload` 함수 추가.
    - `/api/browse/download?path=<파일경로>` 엔드포인트.
    - `Content-Disposition: attachment` 헤더로 다운로드 강제.
    - 보안: 디렉토리 다운로드 방지, 파일 존재 여부 및 권한 검증.
    - `io.Copy`로 파일 스트리밍.
  - **프론트엔드**: FolderContent의 render 함수 수정.
    - 폴더: `<a onClick={...}>` (기존 동작 유지)
    - 파일: `<a href="/api/browse/download?path=..." download>`
    - 조건부 렌더링으로 폴더와 파일 구분.
- **대안 검토**:
  - fetch API로 다운로드: 복잡하고 추가 코드 필요, 브라우저 기본 기능보다 장점 없음.
  - 우클릭 메뉴로만 다운로드: 덜 직관적, 추가 클릭 필요.
- **수정 파일**:
  - `apps/backend/internal/browse/handler/browse_handler.go` (handleDownload, RegisterRoutes)
  - `apps/frontend/src/features/browse/components/FolderContent.tsx` (render 함수)
- **결과**: 파일 클릭 시 다운로드 정상 작동, README.md 테스트 완료.

### Space 경로 보안 강화 (2026-02-04)
- **문제**: Space로 할당되지 않은 시스템 경로에도 자유롭게 접근 가능한 보안 취약점.
- **요구사항**: Space 경로만 접근 허용하되, Space 생성 시 시스템 탐색은 가능해야 함.
- **결정**: 백엔드에서 경로 검증 수행, Space 생성 모드는 별도 플래그로 구분.
- **이유**:
  - 보안: 사용자가 의도적으로 할당한 Space 외부 경로는 접근 불가.
  - 유연성: Space 생성 시에는 시스템 전체를 탐색할 수 있어야 폴더 선택 가능.
  - 서버 측 검증: 클라이언트 우회 방지, 신뢰할 수 있는 검증.
- **구현**:
  - **백엔드**:
    - Handler에 spaceService 주입 (Space 목록 조회).
    - `isPathAllowed(path)`: 요청 경로가 Space의 하위 경로인지 확인.
      - Space 목록을 가져와서 각 space_path와 비교.
      - `strings.HasPrefix`로 하위 경로 판별.
      - Space가 없으면 모든 경로 허용 (하위 호환성).
    - `handleBrowse`, `handleDownload`에서 경로 검증 수행.
    - `system=true` 쿼리 파라미터로 시스템 탐색 모드 지원.
      - systemMode일 때는 경로 검증 스킵.
  - **프론트엔드**:
    - `useBrowseApi`: `fetchDirectoryContents(path, systemMode=false)` 파라미터 추가.
    - `FolderTree`: `showBaseDirectories`일 때 `systemMode=true` 전달.
- **대안 검토**:
  - 별도 API 엔드포인트 (`/api/browse/system`): 중복 코드, 유지보수 복잡도 증가.
  - 클라이언트 검증: 보안 취약, 우회 가능.
  - Space가 없을 때만 전체 허용: 현재 채택, 단순하고 효과적.
- **보안 고려사항**:
  - Space가 없으면 모든 경로 허용: 초기 사용자 경험 향상, 첫 Space 생성 전까지는 제한 없음.
  - 경로 정규화: `filepath.Clean`으로 `..` 등 우회 시도 방지.
  - 403 에러: 명확한 에러 메시지로 권한 부족 알림.
- **수정 파일**:
  - `apps/backend/internal/browse/handler/browse_handler.go` (isPathAllowed, 검증 로직)
  - `apps/backend/main.go` (spaceService 주입)
  - `apps/frontend/src/features/browse/hooks/useBrowseApi.ts` (systemMode 파라미터)
  - `apps/frontend/src/features/browse/components/FolderTree.tsx` (systemMode 전달)
- **결과**:
  - Space 경로만 접근 가능, 외부 경로는 403 에러.
  - Space 생성 모달에서 시스템 전체 탐색 정상 작동.

### 파일/폴더 우클릭 컨텍스트 메뉴 구현 (2026-02-05)
- **결정**: 파일과 폴더에 우클릭 컨텍스트 메뉴를 추가하고 상위 폴더 버튼 제거.
- **이유**:
  - 현대적인 파일 관리자 UX: 우클릭으로 다양한 작업 접근.
  - UI 단순화: Breadcrumb만으로 충분히 탐색 가능, 별도 버튼 불필요.
  - 파일/폴더 타입별 차별화: 파일과 폴더에 맞는 작업 제공.
- **구현**:
  - **컨텍스트 메뉴 구조**:
    - 파일: "다운로드", "이름 변경", "삭제"
    - 폴더: "이름 변경", "삭제" (다운로드 제외)
  - **상위 폴더 버튼 제거**: Breadcrumb의 상위 경로 클릭으로 대체.
  - **컨텍스트 메뉴 상태 관리**:
    - 마우스 위치(x, y)와 대상 레코드 저장.
    - 메뉴 외부 클릭 시 자동 닫힘 (useEffect + document.addEventListener).
  - **Table onRow 이벤트**: `onContextMenu` 핸들러로 우클릭 감지.
  - **Ant Design Menu**: position: fixed로 마우스 위치에 표시.
- **수정 파일**:
  - `apps/frontend/src/features/browse/components/FolderContent.tsx`
- **결과**: 상위 폴더 버튼 제거, 파일/폴더별 컨텍스트 메뉴 정상 작동.

### 파일/폴더 이름 변경 및 삭제 기능 구현 (2026-02-05)
- **결정**: 백엔드 API와 프론트엔드 모달로 이름 변경 및 삭제 기능 완전 구현.
- **이유**:
  - 파일 관리 기본 기능: 사용자가 파일/폴더를 직접 관리할 수 있어야 함.
  - 안전한 삭제: Modal.confirm으로 실수 방지.
  - 즉시 반영: 작업 후 목록 자동 새로고침으로 UX 향상.
- **구현**:
  - **백엔드 API**:
    - `POST /api/browse/rename`: 파일/폴더 이름 변경.
      - 요청: `{ oldPath, newName }`
      - 보안: 기존 경로와 새 경로 모두 Space 내부인지 검증.
      - `os.Rename` 사용.
    - `POST /api/browse/delete`: 파일/폴더 삭제.
      - 요청: `{ path }`
      - 보안: Space 내부 경로인지 검증.
      - 폴더는 `os.RemoveAll` (하위 파일 포함), 파일은 `os.Remove`.
  - **프론트엔드**:
    - **이름 변경 모달**:
      - 현재 이름을 기본값으로 표시.
      - Input에 Enter 키 입력 시 변경 실행.
      - 성공 시 `message.success`, 실패 시 `message.error`.
      - 작업 후 `fetchDirectoryContents`로 목록 새로고침.
    - **삭제 확인 모달**:
      - `Modal.confirm` 사용.
      - 폴더 삭제 시 "하위 파일 포함 삭제" 경고.
      - 확인 후 DELETE API 호출.
      - 성공 시 목록 자동 새로고침.
    - **컨텍스트 메뉴 onClick**:
      - "이름 변경": 모달 열기, 현재 이름을 state에 설정.
      - "삭제": 확인 모달 표시.
- **보안 고려사항**:
  - 이름 변경 시 새 경로도 Space 내부인지 검증: 경로 탈출 방지.
  - Space 외부 경로 접근 시 403 에러.
  - `filepath.Clean`으로 경로 정규화.
- **수정 파일**:
  - `apps/backend/internal/browse/handler/browse_handler.go` (handleRename, handleDelete)
  - `apps/frontend/src/features/browse/components/FolderContent.tsx` (모달, API 호출)
- **결과**: 이름 변경 및 삭제 모두 정상 작동, 목록 자동 새로고침 확인.

### 다운로드 우클릭 전용 변경 (2026-02-05)
- **결정**: 파일 이름 클릭 시 다운로드 제거, 우클릭 메뉴로만 다운로드 가능.
- **이유**:
  - 일관성: 모든 파일 작업을 우클릭 메뉴로 통일.
  - 실수 방지: 의도치 않은 다운로드 방지.
  - 폴더와 일관된 UX: 폴더는 클릭 시 이동, 파일은 클릭 무반응.
- **구현**:
  - 파일 이름을 `<a>` 태그에서 `<span>` 태그로 변경.
  - 다운로드는 컨텍스트 메뉴의 "다운로드" 항목으로만 가능.
  - 파일 아이콘과 이름은 시각적으로만 표시, 클릭 이벤트 없음.
- **대안 검토**:
  - 파일 클릭 시 미리보기: 아직 미리보기 기능 미구현, 추후 고려.
  - 더블 클릭으로 다운로드: 폴더 더블 클릭(이동)과 혼동 가능.
- **수정 파일**:
  - `apps/frontend/src/features/browse/components/FolderContent.tsx` (render 함수)
- **결과**: 파일 클릭 시 다운로드 안 됨, 우클릭 메뉴로만 다운로드 가능.

### 파일 업로드 기능 구현 (2026-02-05)
- **결정**: Drag & Drop 방식의 파일 업로드 기능 구현, 중복 파일 덮어쓰기 확인 포함.
- **이유**:
  - 직관적인 UX: 파일을 드래그하거나 클릭하여 쉽게 업로드.
  - 현대적인 파일 관리자 표준: Google Drive, Dropbox 등 주요 서비스에서 사용.
  - Space 기반 워크플로우와 자연스럽게 통합.
  - 안전한 덮어쓰기: 사용자 확인 후 기존 파일 덮어쓰기 가능.
- **구현**:
  - **백엔드**:
    - `POST /api/browse/upload` 엔드포인트 추가.
    - multipart/form-data로 파일 수신 (최대 32MB).
    - `targetPath` 파라미터로 업로드 대상 디렉토리 지정.
    - `overwrite` 파라미터 (선택): `true`일 때 기존 파일 덮어쓰기.
    - Space 경로 검증: `isPathAllowed`로 대상 경로가 Space 내부인지 확인.
    - 파일 중복 처리:
      - `overwrite=false` (기본값): 동일 이름 파일 존재 시 409 Conflict 에러.
      - `overwrite=true`: 기존 파일 덮어쓰기 (`os.Create`가 자동으로 덮어씀).
    - `os.Create` + `io.Copy`로 파일 저장.
    - 실패 시 부분 저장된 파일 정리 (`os.Remove`).
  - **프론트엔드**:
    - Ant Design `Upload.Dragger` 컴포넌트 사용.
    - `customRequest`로 커스텀 업로드 로직 구현.
    - `performUpload` 함수로 업로드 로직 분리 (재사용 가능).
    - 409 에러 처리:
      - `Modal.confirm`으로 덮어쓰기 확인 모달 표시.
      - 확인 시 `overwrite=true`로 재업로드.
      - 취소 시 업로드 중단.
    - FormData로 파일, targetPath, overwrite 전송.
    - 업로드 성공 시 `message.success` 표시 및 목록 자동 새로고침.
    - `showUploadList: false`로 기본 업로드 목록 숨김 (즉시 새로고침으로 대체).
    - FolderContent 상단 Breadcrumb 아래 배치.
- **보안 고려사항**:
  - Space 경로 검증: Space 외부 경로 업로드 차단 (403 에러).
  - 파일 크기 제한: 32MB (서버 메모리 보호).
  - 덮어쓰기 확인: 사용자 명시적 확인 없이는 덮어쓰기 불가.
- **대안 검토**:
  - 별도 업로드 버튼: Drag & Drop보다 덜 직관적.
  - 무제한 파일 크기: 서버 메모리 고갈 위험.
  - 중복 파일 자동 덮어쓰기: 데이터 손실 위험, 사용자 확인 필수.
  - 중복 파일 자동 이름 변경: 사용자가 예상하지 못한 파일명, 혼란 야기.
- **수정 파일**:
  - `apps/backend/internal/browse/handler/browse_handler.go` (handleUpload, RegisterRoutes)
  - `apps/frontend/src/features/browse/components/FolderContent.tsx` (Upload.Dragger, 덮어쓰기 확인 모달 추가)
- **결과**: Drag & Drop 파일 업로드 정상 작동, 중복 파일 덮어쓰기 확인 모달 정상 작동.

### 뷰 전환 기능 구현 (2026-02-05)
- **결정**: 테이블 뷰와 그리드(앨범) 뷰를 전환할 수 있는 기능 구현.
- **이유**:
  - 사용자 선호도: 일부 사용자는 상세 정보를 선호하고, 일부는 시각적 탐색을 선호.
  - 파일 타입별 최적화: 이미지는 그리드가, 문서는 테이블이 더 적합.
  - 현대적인 파일 관리자 표준: Finder, Google Drive, Dropbox 모두 뷰 전환 기능 제공.
  - 유연성: 사용자가 상황에 따라 적합한 뷰 선택 가능.
- **구현**:
  - **뷰 전환 UI**:
    - Button.Group으로 두 버튼 묶음.
    - Breadcrumb 우측에 배치 (justify-space-between).
    - 활성 버튼은 `type="primary"`로 강조.
    - 아이콘: UnorderedListOutlined (테이블), AppstoreOutlined (그리드).
  - **테이블 뷰** (기존):
    - Ant Design Table 컴포넌트.
    - 파일명, 수정일, 크기 정보 표시.
    - 정렬, 더블 클릭 이동, 우클릭 메뉴 지원.
  - **그리드 뷰** (신규):
    - Ant Design Row/Col + Card 컴포넌트.
    - 반응형 그리드: xs=12 (2열), sm=8 (3열), md=6 (4열), lg=4 (6열), xl=3 (8열).
    - 카드 내용:
      - 큰 아이콘 (48px): 폴더(노란색), 파일(회색).
      - 파일 이름 (12px, word-break).
      - 파일 크기 (11px, 파일만).
    - 더블 클릭 이동, 우클릭 메뉴 기능 유지.
    - 중앙 정렬, 카드 호버 효과.
  - **상태 관리**:
    - `viewMode` state ('table' | 'grid').
    - 조건부 렌더링으로 뷰 전환.
    - 기본값: 'table'.
- **디자인 고려사항**:
  - 8px 그리드 시스템 준수: gutter=[16, 16], padding=16px.
  - 일관된 아이콘 색상: 폴더(#ffca28), 파일(#8c8c8c).
  - Card bodyStyle로 padding 조정 (16px 8px).
  - word-break로 긴 파일명 처리.
- **대안 검토**:
  - 리스트 뷰 추가: 테이블과 유사하여 불필요.
  - 아이콘 크기 조정 슬라이더: 복잡도 증가, 우선순위 낮음.
  - 뷰 모드 localStorage 저장: 현재는 세션별 초기화, 추후 고려.
- **수정 파일**:
  - `apps/frontend/src/features/browse/components/FolderContent.tsx` (뷰 전환 버튼, 그리드 뷰 추가)
- **결과**: 테이블 ↔ 그리드 양방향 전환 정상 작동, 그리드 뷰 반응형 정상 작동.

### 업로드 UI 개선 (2026-02-05)
- **결정**: Upload.Dragger 섹션 제거, 전체 영역 드래그 앤 드롭 지원, 업로드 버튼 추가.
- **이유**:
  - 화면 공간 효율성: 별도 업로드 섹션이 화면 공간을 많이 차지.
  - 현대적인 UX: Google Drive, Dropbox 등은 전체 영역 드래그 지원.
  - 직관성: 파일 탐색기 어디에나 드롭 가능.
  - 일관성: 뷰 전환 버튼과 업로드 버튼을 같은 영역에 배치.
- **구현**:
  - **Upload.Dragger 제거**:
    - 기존 Upload.Dragger 섹션 완전 제거.
    - 파일 목록이 화면을 더 넓게 사용.
  - **전체 영역 드래그 앤 드롭**:
    - 최상위 div에 드래그 이벤트 리스너 추가.
    - `onDragEnter`, `onDragLeave`, `onDragOver`, `onDrop`.
    - `isDragging` state로 드래그 상태 관리.
    - 드래그 중 오버레이 표시:
      - 반투명 파란색 배경 (rgba(24, 144, 255, 0.1)).
      - 파란색 점선 테두리 (2px dashed #1890ff).
      - 중앙 정렬된 아이콘 + 메시지 ("파일을 놓아 업로드").
      - z-index: 999, position: absolute.
  - **업로드 버튼**:
    - 위치: Breadcrumb 우측, 뷰 전환 버튼 옆.
    - Space 컴포넌트로 버튼들 그룹화.
    - UploadOutlined 아이콘 + "업로드" 텍스트.
    - 숨겨진 file input (display: none).
    - 버튼 클릭 시 fileInputRef.current?.click().
  - **업로드 로직 통합**:
    - `handleFileUpload` 함수로 드래그/버튼 업로드 통합.
    - 중복 확인 및 덮어쓰기 모달 기능 유지.
    - performUpload 함수 재사용.
- **디자인 고려사항**:
  - 드래그 오버레이: 파란색(#1890ff) 테마 일관성.
  - 아이콘 크기: 64px (눈에 잘 띄도록).
  - pointerEvents: none (오버레이가 드롭 이벤트 방해 방지).
- **대안 검토**:
  - Upload.Dragger 유지: 화면 공간 낭비, 중복 기능.
  - 작은 드롭 영역 추가: 현대적인 UX와 맞지 않음.
  - 오버레이 없이 드래그만 지원: 시각적 피드백 부족.
- **수정 파일**:
  - `apps/frontend/src/features/browse/components/FolderContent.tsx` (드래그 이벤트, 업로드 버튼 추가)
- **결과**: 전체 영역 드래그 앤 드롭 정상 작동, 업로드 버튼 정상 작동, 화면 공간 효율화.

### Breadcrumb 절대 경로 버그 수정 (2026-02-06)
- **문제**: 하위 폴더(2+ depth) 이동 시 breadcrumb이 절대 경로로 표시되는 버그.
- **원인**:
  - FolderTree에서 Space 노드가 아닌 하위 폴더 선택 시 Space 정보 없이 경로만 전달.
  - FolderContent에서 폴더 더블 클릭 시 `onPathChange(path)` 호출로 경로만 업데이트.
  - `selectedSpace` state가 현재 경로와 동기화되지 않아 breadcrumb 로직이 실패.
  - Breadcrumb 로직: `if (selectedSpace && selectedPath.startsWith(selectedSpace.space_path))`
  - 경로가 다른 Space에 속하거나 selectedSpace가 없으면 절대 경로로 fallback.
- **결정**: `handlePathSelect` 함수에서 Space가 명시되지 않으면 자동으로 찾도록 개선.
- **이유**:
  - 사용자가 어떤 방식으로 탐색하든 (사이드바 클릭, 더블 클릭, breadcrumb 클릭) 일관된 UX 제공.
  - Space 중심 워크플로우 유지: 항상 Space 기준 상대 경로 표시.
  - 코드 중복 방지: FolderTree, FolderContent 모두 수정 불필요.
- **구현**:
  - `MainLayout/index.tsx`의 `handlePathSelect(path, space?)` 수정:
    ```typescript
    if (space) {
      setSelectedSpace(space);
    } else {
      const matchedSpace = spaces?.find(s => path.startsWith(s.space_path));
      setSelectedSpace(matchedSpace);
    }
    ```
  - `path`가 어떤 Space의 하위 경로인지 자동 탐색.
  - 매칭되는 Space를 `selectedSpace`에 설정.
  - 매칭되는 Space가 없으면 `undefined` 설정 (절대 경로 표시).
- **대안 검토**:
  - FolderTree에서 항상 Space 정보 전달: 복잡한 로직, 여러 파일 수정 필요.
  - FolderContent에서 Space 찾기: 중복 로직, MainLayout에서 처리하는 것이 더 효율적.
  - Breadcrumb에서 Space 찾기: 로직이 여러 곳에 분산, 유지보수 어려움.
- **수정 파일**:
  - `apps/frontend/src/components/layout/MainLayout/index.tsx` (handlePathSelect 로직 개선)
- **결과**: 하위 폴더(2+ depth) 탐색 시 breadcrumb이 항상 상대 경로로 정상 표시.

### 그리드 뷰 기본값 변경 (2026-02-06)
- **결정**: 파일 탐색기의 기본 뷰를 테이블에서 그리드(앨범)로 변경.
- **이유**:
  - 사용자 요청: 기본적으로 앨범 형식을 선호.
  - 시각적 탐색 용이성: 그리드 뷰가 파일/폴더 구조를 한눈에 파악하기 쉬움.
  - 현대적인 파일 관리자 트렌드: Finder, Google Drive 등 대부분 그리드 뷰 기본 제공.
  - 이미지/미디어 중심 작업: 그리드 뷰가 파일 미리보기에 더 적합.
- **구현**:
  - `FolderContent.tsx`의 `viewMode` state 초기값 변경.
  - 기존: `useState<'table' | 'grid'>('table')`
  - 변경: `useState<'table' | 'grid'>('grid')`
  - 뷰 전환 기능은 유지: 사용자가 원하면 테이블 뷰로 전환 가능.
- **대안 검토**:
  - localStorage에 사용자 선호 뷰 저장: 추후 고려, 현재는 세션별 초기화.
  - 파일 타입별 자동 뷰 전환: 복잡도 증가, 우선순위 낮음.
  - 그리드 뷰만 제공: 유연성 부족, 사용자 선택권 제거.
- **수정 파일**:
  - `apps/frontend/src/features/browse/components/FolderContent.tsx` (viewMode 기본값 변경)
- **결과**: Space 선택 시 그리드 뷰가 기본으로 표시, 뷰 전환 기능 정상 작동.

### 사이드바 트리 노드 증식 버그 수정 (2026-02-07)
- **문제**: 여러 Space를 열고 닫을 때 하위 폴더 노드가 중복 생성.
- **원인**: 같은 디렉토리를 가리키는 Space들의 하위 노드가 파일 경로를 key로 사용하여 Ant Design Tree key 충돌.
- **결정**: 자식 노드 key에 부모 Space prefix를 붙여 유일성 보장 + `expandedKeys` 명시적 상태 관리.
- **구현**:
  - key 형식: `space-{id}::{filePath}` (Space 하위), 기존 경로 (비-Space).
  - `handleSelect`에서 `::` 구분자로 실제 경로 추출.
  - `expandedKeys` state + `onExpand` 핸들러로 확장 상태 제어.
  - treeData 초기화 시 `loadedKeys`, `expandedKeys` 동시 초기화.
- **수정 파일**: `FolderTree.tsx`

### 서버 상태 표시 개선 (2026-02-07)
- **결정**: Status popover를 텍스트 바로 아래로 이동하고, 호스트/포트/경로 정보 추가.
- **이유**:
  - 기존 popover가 텍스트에서 떨어져 표시되어 연결성이 약함.
  - 사용자가 각 프로토콜의 접근 정보를 한눈에 확인할 수 있어야 함.
- **구현**:
  - 백엔드: `net.InterfaceAddrs()`로 IPv4 주소 수집, `ProtocolStatus`에 `Port`/`Path` 추가.
  - 프론트엔드: `placement="bottomLeft"`, 포트/경로 inline 표시, 호스트 목록 섹션 추가.
- **수정 파일**: `handler.go`, `main.go`, `types.ts`, `ServerStatus.tsx`

### WebDAV 루트 디렉토리 가상 Space 목록 (2026-02-07)
- **결정**: `/dav/` 접근 시 커스텀 `webdav.FileSystem`으로 모든 Space를 가상 폴더로 노출.
- **이유**:
  - WebDAV 클라이언트(Finder 등)가 `/dav/`에 마운트하면 모든 Space를 볼 수 있어야 함.
  - 기존 구현은 `/dav/{spaceName}` 형태만 지원하여 루트 접근 시 400 에러.
- **구현**:
  - `SpaceFS`: `webdav.FileSystem` 인터페이스 구현.
  - 루트(`/`): `Readdir`에서 Space 목록을 가상 디렉토리 FileInfo로 반환.
  - `/{spaceName}/...`: Space의 `SpacePath`로 실제 OS 파일시스템 위임.
  - 루트 Mkdir/RemoveAll/Rename 거부 (Space 관리는 API로만).
  - 경로 탈출 방지: `strings.HasPrefix(realPath, sp.SpacePath)`.
- **대안 검토**:
  - per-space 핸들러 유지 + 루트용 별도 핸들러: 중복 코드, 복잡도 증가.
  - 하나의 SpaceFS로 통합: 단일 핸들러로 루트와 하위 모두 처리, 더 깔끔.
- **수정 파일**: `spacefs.go`(신규), `service.go`, `webdav_handler.go`

### 레이아웃 여백 16px 통일 (2026-02-07)
- **결정**: 모든 레이아웃 영역의 수평 패딩을 `16px` (8-grid 2×8)로 일괄 통일.
- **이유**:
  - Header의 antd 기본 패딩(`0 50px`)이 50px로 8-grid 위반.
  - 사이드바 헤더의 세로 패딩 `12px`가 8-grid 위반.
  - 사이드바(16px), 메인 콘텐츠(24px) 간 수평 패딩 불일치.
  - 시각적으로 "Cohesion" 텍스트와 "Spaces" 라벨이 정렬되지 않음.
- **구현**:
  - Header: `padding: '0 16px'` 명시적 설정 (antd 기본값 오버라이드).
  - 사이드바 헤더: `'12px 16px'` → `'16px'` (상하좌우 균일).
  - 사이드바 트리: `'8px'` → `'8px 16px'` (수평만 통일, 수직은 컴팩트 유지).
  - 메인 콘텐츠: `'24px'` → `'16px'` (수평 패딩 통일).
- **대안 검토**:
  - `24px` 기준 통일: 사이드바에 과도한 여백, 300px 폭에서 비효율적.
  - `8px` 기준 통일: 너무 좁아 답답한 느낌.
- **수정 파일**:
  - `apps/frontend/src/components/layout/MainLayout/index.tsx` (Header padding)
  - `apps/frontend/src/components/layout/MainLayout/MainSider.tsx` (사이드바 헤더, 트리 padding)
  - `apps/frontend/src/features/browse/components/FileExplorer.tsx` (메인 콘텐츠 padding)
- **결과**: 모든 영역 수평 패딩 16px 통일, 다크/라이트 모드 및 그리드/테이블 뷰 모두 정상.

### 컨텍스트 메뉴 UI 개선 — Ant Design Dropdown 전환 (2026-02-08)
- **결정**: 기존 `Menu` + `position: fixed` 인라인 스타일 → Ant Design `Dropdown` 컴포넌트 래핑 공통 `ContextMenu` 생성.
- **이유**:
  - FolderContent, FolderTree에서 동일한 컨텍스트 메뉴 패턴 (상태관리 + document click 리스너) 중복.
  - 화면 경계 처리 없음 (메뉴가 화면 밖으로 잘릴 수 있음).
  - 애니메이션 없이 즉시 출현/사라짐.
  - ESC 키 닫기 미지원.
- **구현**:
  - `src/components/ContextMenu.tsx`: Ant Design `Dropdown` + `trigger={[]}` 제어 모드.
  - 투명 trigger span을 클릭 좌표에 `position: fixed`로 배치, Dropdown이 자동 위치 보정.
  - `useEffect`로 외부 클릭(document click) + ESC 키(keydown) 리스너 등록.
  - FolderContent, FolderTree: `Menu` import 제거, `ContextMenu` 사용, 중복 useEffect 제거.
- **대안 검토**:
  - 커스텀 컴포넌트 (framer-motion): 과도한 개발 비용, Ant Design 일관성 저하.
  - `Dropdown trigger={['contextMenu']}`: Table onRow / Tree onRightClick과 호환 어려움.
- **수정 파일**:
  - `apps/frontend/src/components/ContextMenu.tsx` (신규)
  - `apps/frontend/src/features/browse/components/FolderContent.tsx`
  - `apps/frontend/src/features/browse/components/FolderTree.tsx`
- **결과**: 화면 경계 자동 보정, 페이드 애니메이션, ESC/외부 클릭 닫기, 중복 코드 제거.

### 새 폴더 만들기 기능 구현 (2026-02-08)
- **결정**: 빈 영역 우클릭 시 "새 폴더 만들기" 메뉴를 표시하고 모달로 폴더 생성.
- **이유**:
  - 파일 관리 기본 기능: 사용자가 직접 폴더 구조를 만들 수 있어야 함.
  - 파일 복사/이동 기능(Task 2)의 기반: 빈 영역 컨텍스트 메뉴 인프라 구축.
  - 직관적인 UX: Google Drive, Finder 등 주요 파일 관리자와 일관된 경험.
  - 안전한 생성: 중복 확인 및 특수문자 검증으로 파일시스템 보호.
- **구현**:
  - **백엔드**:
    - `POST /api/browse/create-folder` 엔드포인트 추가.
    - 요청: `{ parentPath, folderName }`.
    - 검증 순서:
      1. POST 메서드 확인
      2. Request body 파싱 및 필드 검증 (빈 값 체크)
      3. `parentPath` Space 경로 검증 (`isPathAllowed`)
      4. 부모 디렉토리 존재 및 디렉토리 여부 확인 (`os.Stat`)
      5. 폴더명 유효성 검증 (`/`, `\`, `:`, `*`, `?`, `"`, `<`, `>`, `|` 금지)
      6. 중복 확인 (동일 이름 폴더 존재 시 409 Conflict)
      7. `os.Mkdir(folderPath, 0755)` 실행
      8. 성공 응답 반환 (path, name 포함)
    - 에러 코드: 400 (잘못된 요청), 403 (Space 외부), 404 (부모 없음), 409 (중복), 500 (OS 오류).
  - **프론트엔드**:
    - **빈 영역 감지**:
      - 기존: `e.currentTarget === e.target` (너무 엄격, Grid View/Table View에서 실패).
      - 개선: `!target.closest('.ant-card') && !target.closest('tr')` (카드나 테이블 행이 아닌 곳).
    - **빈 영역 컨텍스트 메뉴**:
      - `emptyAreaMenu` state로 빈 영역 메뉴 상태 관리.
      - `emptyAreaMenuItems`: "새 폴더 만들기" (FolderOutlined 아이콘).
      - ContextMenu 컴포넌트 재사용.
    - **폴더 생성 모달**:
      - `createFolderModal` state로 모달 상태 및 폴더명 관리.
      - Input autoFocus, Enter 키로 생성, 취소/생성 버튼.
      - `handleCreateFolder`: API 호출 → 성공 시 message.success → 목록 새로고침.
    - **API 호출**:
      - `POST /api/browse/create-folder` with `{ parentPath: selectedPath, folderName }`.
      - 409 에러 시 "폴더 이미 존재" 메시지.
- **대안 검토**:
  - 별도 "새 폴더" 버튼: 화면 공간 낭비, 덜 직관적.
  - 파일/폴더 메뉴에 "새 폴더" 추가: 빈 영역 메뉴가 더 직관적.
  - 빈 영역 더블 클릭으로 생성: 실수로 생성 가능, 명시적 메뉴가 더 안전.
  - 자동 이름 생성 ("새 폴더", "새 폴더 2", ...): 사용자 의도와 다를 수 있음, 직접 입력이 더 명확.
- **수정 파일**:
  - `apps/backend/internal/browse/handler/browse_handler.go` (handleCreateFolder, RegisterRoutes)
  - `apps/frontend/src/features/browse/components/FolderContent.tsx` (빈 영역 메뉴, 모달, 감지 로직)
- **결과**: 빈 영역 우클릭 → 메뉴 → 모달 → 폴더 생성 → 자동 새로고침 흐름 정상 작동.

### TypeScript 빌드 에러 수정 (2026-02-08)
- **문제**: 프론트엔드 빌드 시 TypeScript 타입 에러 발생.
  - `Upload`, `UploadProps` 임포트되었으나 사용되지 않음 (TS6133, TS6196).
  - 이벤트 핸들러 파라미터에 타입 명시 없음 (TS7006, TS7031).
  - 빌드 실패로 런타임에서 이전 코드가 실행되어 null 참조 에러 발생.
- **결정**: 엄격한 타입 체크를 준수하여 모든 파라미터에 명시적 타입 지정.
- **이유**:
  - TypeScript strict mode: 타입 안정성 향상 및 런타임 에러 예방.
  - 빌드 성공: 최신 코드가 브라우저에 반영되어야 버그 수정 효과 확인 가능.
  - 코드 가독성: 명시적 타입은 IDE 자동완성 및 문서화에 도움.
- **구현**:
  - **FolderContent.tsx**:
    - 미사용 import 제거: `Upload`, `UploadProps`.
    - `onContextMenu`: `(e: React.MouseEvent<HTMLElement>) => ...`
    - `onChange` (rename modal): `(e: React.ChangeEvent<HTMLInputElement>) => ...`
    - `onChange` (create folder modal): `(e: React.ChangeEvent<HTMLInputElement>) => ...`
  - **FolderTree.tsx**:
    - `handleExpand`: `(keys: React.Key[]) => ...`
    - `handleRightClick`: `({ event, node }: { event: React.MouseEvent; node: any }) => ...`
- **대안 검토**:
  - `@ts-ignore` 사용: 타입 체크 우회는 런타임 에러 위험 증가, 유지보수성 저하.
  - `any` 타입 사용: 타입 안정성 손실, 버그 탐지 어려움.
  - 타입 명시: 가장 안전하고 명확한 방법.
- **수정 파일**:
  - `apps/frontend/src/features/browse/components/FolderContent.tsx`
  - `apps/frontend/src/features/browse/components/FolderTree.tsx`
- **결과**: 빌드 성공, 브라우저 새로고침 후 정상 동작 (null 참조 에러 해결).

### 파일 브라우저 정렬 기능 구현 (2026-02-09)
- **결정**: 클라이언트 정렬 방식으로 폴더 우선 정렬 구현.
- **이유**:
  - 현재 폴더 내 파일만 정렬하므로 클라이언트 정렬로 충분 (수백 개 파일도 빠름).
  - 백엔드 수정 불필요, 프론트엔드만 수정하여 빠르게 구현.
  - useMemo로 최적화하여 불필요한 재정렬 방지.
- **구현**:
  - **정렬 상태**: `sortBy` (name/modTime/size), `sortOrder` (ascend/descend).
  - **정렬 로직**:
    1. 폴더 우선: `a.isDir !== b.isDir ? (a.isDir ? -1 : 1)`
    2. sortBy 기준 정렬: localeCompare (이름), getTime (수정일), 숫자 비교 (크기)
    3. sortOrder 적용: ascend는 그대로, descend는 결과 반전
  - **그리드 뷰**: Select 드롭다운으로 6가지 정렬 옵션 제공.
  - **테이블 뷰**: Ant Design Table의 onChange 핸들러로 정렬 상태 업데이트.
  - **뷰 전환**: sortConfig state를 공유하여 뷰 전환 시 정렬 유지.
- **대안 검토**:
  - 서버 정렬: 불필요한 복잡도, 클라이언트 정렬로 충분.
  - 정렬 없이 백엔드 순서대로 표시: 사용자 경험 저하, 파일 관리 어려움.
  - 폴더와 파일 섞어서 정렬: 직관성 떨어짐, 대부분의 파일 관리자가 폴더 우선 채택.
- **기본 정렬**: 폴더 우선 + 이름 오름차순 (가장 직관적).
- **수정 파일**:
  - `apps/frontend/src/features/browse/components/FolderContent.tsx`
- **결과**: 모든 정렬 옵션 정상 작동, 뷰 전환 시 정렬 유지 확인.

### 드래그 앤 드롭 파일 이동 기능 (2026-02-09)
- **결정**: Google Drive 스타일의 드래그 앤 드롭으로 파일/폴더를 다른 폴더로 이동하는 기능 구현.
- **이유**:
  - 직관적인 UX: 사용자가 익숙한 드래그 앤 드롭 인터페이스 제공.
  - 작업 효율성: 우클릭 메뉴보다 빠른 파일 이동.
  - 다중 이동 지원: 선택된 여러 파일을 한 번에 이동 가능.
- **구현**:
  - **드래그 소스**:
    - 모든 Card와 Table Row에 `draggable={true}` 속성 추가.
    - `onDragStart`: 선택되지 않은 항목 드래그 시 자동 선택, dataTransfer에 경로 목록 저장.
    - 데이터 타입: `application/json` with `{type: 'cohesion-internal', paths: [...]}`
  - **드롭 타겟**:
    - 폴더에만 드롭 가능, 파일에는 불가.
    - `onDragOver`: 폴더에 호버 시 `dragOverFolder` 상태로 시각적 피드백 (파란 테두리/배경).
    - `onDrop`: 폴더에 드롭 시 해당 폴더로 이동, 자기 자신 이동 방지.
    - 빈 영역 드롭: 현재 폴더에 이동 (같은 폴더면 무시).
  - **외부 파일 업로드와 구분**:
    - `dataTransfer.files.length > 0`: 외부 파일 업로드.
    - `dataTransfer.getData('application/json')`: 내부 파일 이동.
    - 외부 파일 드래그 시에만 `isDragging` 상태 활성화 (오버레이 표시).
  - **텍스트 선택 방지**:
    - Card와 Table Row에 `userSelect: 'none'` CSS 적용.
    - 드래그 중 텍스트 선택 안 됨.
  - **시각적 피드백**:
    - 드롭 가능한 폴더: 파란 테두리 (그리드: 2px dashed, 테이블: 배경색).
    - 선택된 항목: 파란 테두리 + 배경색.
- **제약사항**:
  - 자기 자신으로 이동 방지 (message.warning 표시).
  - Space 외부 이동 방지 (기존 move API 검증 활용).
  - 부모 폴더를 자식 폴더로 이동 방지 (백엔드 검증).
- **기존 기능과의 호환성**:
  - 기존 우클릭 메뉴 이동/복사 기능 유지.
  - 외부 파일 업로드 드래그 앤 드롭 정상 작동.
  - 다중 선택 기능과 완벽 통합.
- **대안 검토**:
  - 우클릭만 사용: 작업 효율성 떨어짐, Google Drive 등 경쟁 제품 대비 UX 열등.
  - 별도 버튼으로 이동: 단계가 많아 불편함.
  - 드래그 앤 드롭: 가장 직관적이고 효율적.
- **수정 파일**:
  - `apps/frontend/src/features/browse/components/FolderContent.tsx`
- **결과**: 단일/다중 파일 드래그 이동 성공, 외부 파일 업로드와 충돌 없음, 텍스트 선택 방지 확인.