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

## 다음 작업 (Next Steps)
- Space 상대 경로 표시 (Breadcrumb UX 개선).
- 파일 업로드 기능 (Drag & Drop) 구현.
- 파일 우클릭 메뉴(Context Menu) 추가 (삭제, 이름 변경 등).
- 이미지/텍스트 파일 미리보기 기능 검토.