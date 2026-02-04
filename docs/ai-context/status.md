# 프로젝트 상태 (Status)

## 현재 진행 상황
- **기능 구현 완료**: Google Drive 스타일의 파일 탐색 GUI 구현.
    - 백엔드 API 고도화 (파일 크기, 수정일 정보 추가).
    - 프론트엔드 `FolderContent`를 `Table` 기반으로 전면 개편.
    - Breadcrumb 내비게이션 및 폴더 진입/탐색 기능 추가.
- **버그 수정**: 트리 중복 생성 및 확장 불능 문제 해결 상태 유지 확인.
- **Space 연동 완료** (2026-02-04):
    - **통합 트리 구조**: Space 목록과 FolderTree를 하나의 트리로 완전 통합.
    - Spaces가 트리의 루트 노드로 표시, 확장 시 해당 Space의 하위 디렉토리 로드.
    - Space 메뉴 제거, 단일 트리 구조로 단순화.
    - `FolderTree`가 `spaces` prop을 받아 Space 노드 생성 및 lazy loading 처리.
    - `MainSider`: Spaces 헤더 + 버튼, 통합 FolderTree.
    - `FileExplorer` 단순화: FolderContent만 전체 화면으로 표시.
    - 모달에서는 `showBaseDirectories` 플래그로 시스템 디렉토리 탐색.
    - 파일: `MainLayout/index.tsx`, `MainSider.tsx`, `FileExplorer.tsx`, `FolderTree.tsx` 수정.
- **코드 품질 개선** (2026-02-04):
    - Serena MCP 필수 사용 규칙을 `CLAUDE.md`에 추가.
    - 여러 Serena replace_symbol_body 사용 시 발생한 중복 코드 제거.
    - ESLint 에러 수정: 파싱 에러, 사용하지 않는 import, any 타입 등.
    - React 권장 패턴 준수: useEffect에서 setState 제거, 올바른 폼 초기화.
    - 테마 대응: 텍스트 색상이 다크/라이트 모드에 맞춰 자동 변경.

- **문서 구조 개편** (2026-02-04):
    - `CLAUDE.md`를 `.claude/CLAUDE.md`로 이동 (Claude Code CLI 전용 설정).
    - `GEMINI.md` 간소화: "docs/master_rule.md를 먼저 읽기"로 단순화.
    - `docs/master_rule.md`에 모든 규칙 통합:
        - Playwright MCP 브라우저 테스트 절차 추가.
        - 디자인 규칙 (8px 그리드, 일관된 radius 등) 명시.
        - 커밋 메시지 규칙 (한글, 간결체) 추가.
        - Serena MCP 사용 상세 규칙을 GEMINI.md에서 이동.
    - `.gitignore`에 테스트/디버깅 디렉토리 추가 (.playwright-mcp/, .serena 등).

- **문서 구조 개선** (2026-02-04):
    - `docs/AGENTS.md` 생성: 실행 환경 정보 통합 문서.
        - 프로젝트 구조, 서버 실행, 빌드, 포트 정보.
        - 테스트 방법, 트러블슈팅, 패키지 관리.
    - `.claude/CLAUDE.md`, `GEMINI.md` 수정: AGENTS.md 읽기 추가.
- **Space 상대 경로 Breadcrumb 구현 완료** (2026-02-04):
    - FolderTree: Space 선택 시 Space 정보도 함께 전달하도록 수정.
    - MainLayout: selectedSpace state 추가 및 Outlet context에 포함.
    - FileExplorer: OutletContext에 selectedSpace 추가.
    - FolderContent: Space 상대 경로로 Breadcrumb 표시.
        - Space 선택 시: "SpaceName / folder1 / folder2" 형식.
        - Space 미선택 시: 기존 절대 경로 유지.
    - 타입 에러 수정 (FolderContent, DirectorySetupModal).
    - Playwright 브라우저 테스트 완료: Breadcrumb 정상 작동 확인.

- **Space 삭제 기능 구현 완료** (2026-02-04):
    - FolderTree: Space 노드에 Context Menu (Dropdown) 추가.
    - MainSider: useDeleteSpace 훅 사용하여 삭제 로직 구현.
    - 삭제 확인 Modal (Modal.confirm) 추가.
    - 삭제 후 Space 목록 자동 갱신.
    - 성공/실패 메시지 표시 (message.success/error).
    - Playwright 브라우저 테스트 완료: Context Menu, 확인 모달, 삭제 후 트리 갱신 모두 정상 작동 확인.

- **파일 표시 버그 수정 완료** (2026-02-04):
    - 문제: FolderContent에서 폴더만 표시되고 파일이 표시되지 않음.
    - 원인: `browse_handler.go`의 `ListDirectory` 호출 시 `isOnlyDir` 파라미터가 `true`로 설정되어 있어 파일 필터링.
    - 해결: `apps/backend/internal/browse/handler/browse_handler.go:51` 라인 수정.
        - 변경 전: `h.browseService.ListDirectory(true, targetPath)`
        - 변경 후: `h.browseService.ListDirectory(false, targetPath)`
    - 결과: 오른쪽 FolderContent에서 폴더와 파일이 모두 정상 표시.
    - Playwright 브라우저 테스트 완료: 파일 아이콘, 크기, 수정일 모두 정상 표시 확인.

## 다음 작업 (Next Steps)
- 파일 업로드 기능 (Drag & Drop) 구현.
- 파일 우클릭 메뉴(Context Menu) 추가 (삭제, 이름 변경 등).
- 이미지/텍스트 파일 미리보기 기능 검토.