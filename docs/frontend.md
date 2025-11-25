# frontend

## 폴더 구조
```
src/
├── assets/              # 이미지, 폰트, 글로벌 CSS
├── components/          # "프로젝트 전체"에서 쓰이는 공용 UI (버튼, 모달, 입력창)
│   ├── ui/              # (선택) MUI나 AntD 같은 라이브러리 래핑 컴포넌트
│   └── Layout/          # 전체 레이아웃 (Header, Sidebar, Main)
├── features/            # 기능별 모듈화
│   ├── auth/            # 로그인, 회원가입 관련 (LoginForm, authSlice...)
│   ├── file-browser/    # 파일 탐색기 관련 (가장 중요)
│   │   ├── components/  # FileList, FileIcon, UploadButton
│   │   ├── hooks/       # useFileUpload, useFolderNavigation
│   │   ├── types/       # FileNode, Folder 구조 타입 정의
│   │   └── index.ts     # 외부로 노출할 컴포넌트만 export
│   └── settings/        # 설정 관련 기능
├── hooks/               # 전역적으로 쓰이는 훅
├── lib/                 # 서드파티 라이브러리 설정
├── pages/               # 실제 라우팅되는 페이지 (각 feature들을 조합하는 곳)
├── store/               # 전역 상태 관리
├── types/               # 전역 타입 정의
├── utils/               # 순수 자바스크립트 유틸 함수 (날짜 포맷, 파일 용량 변환)
├── App.tsx
└── main.tsx
```