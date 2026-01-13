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

---

## 타입(Type) 정의 규칙

TypeScript의 `type`과 `interface`를 일관성 있게 사용하기 위해 다음 규칙을 따른다.

- **`interface`는 객체의 형태(Shape)를 정의하는 데 사용한다.**
    - API 응답 데이터, DB 데이터 모델 등 명확한 구조를 가진 객체에 우선적으로 사용한다.
    - 선언 병합(Declaration Merging)이 필요할 경우 사용한다.
    - **예시:**
        ```typescript
        // features/browse/types.ts
        export interface FileNode {
          name: string;
          path: string;
          isDir: boolean;
        }
        ```

- **`type`은 타입에 별칭(Alias)을 붙이는 데 사용한다.**
    - `string | number` 와 같은 유니언(Union), `A & B` 와 같은 인터섹션(Intersection) 등 여러 타입을 조합하여 새로운 타입을 만들 때 사용한다.
    - 객체 형태를 정의할 수도 있지만, 주로 특정 UI 컴포넌트에 맞춘 데이터 형식이거나 확장 가능성이 없는 경우에 제한적으로 사용한다.
    - **예시:**
        ```typescript
        // features/browse/types.ts
        export type TreeDataNode = {
          title: string;
          key: string;
          isLeaf: boolean;
          children?: TreeDataNode[];
        };

        // features/user/types.ts
        export type UserRole = 'admin' | 'editor' | 'viewer';
        ```