## 1. Backend contract

- [x] 1.1 `/api/search/files` 응답을 `items`, `limit`, `hasMore` 객체로 확장한다.
- [x] 1.2 limit 초과 일치 여부를 감지해 `hasMore`를 계산하고 기존 정렬 규칙을 유지한다.
- [x] 1.3 검색 핸들러 테스트를 새 응답 계약과 truncation 시나리오에 맞게 보강한다.

## 2. Frontend search state

- [x] 2.1 검색 API 래퍼와 타입을 새 응답 계약으로 갱신한다.
- [x] 2.2 검색 페이지 훅에서 반환 개수, 현재 limit, hasMore, 더 보기 액션을 관리한다.
- [x] 2.3 헤더 검색은 새 응답 계약을 사용하면서 기존 debounce/선택 흐름을 유지한다.

## 3. Search UI

- [x] 3.1 검색 페이지에 결과 수, 정렬 기준, truncation 안내를 추가한다.
- [x] 3.2 검색 결과에 `spaceName`과 `parentPath` 문맥을 노출한다.
- [x] 3.3 `hasMore=true`일 때 검색 페이지에서 `더 보기`로 limit를 늘려 재조회한다.

## 4. Verification

- [x] 4.1 헤더 검색과 검색 페이지 회귀 테스트를 보강한다.
- [x] 4.2 `pnpm --dir apps/frontend typecheck`, `pnpm --dir apps/frontend test`, `cd apps/backend && go test ./...`로 검증한다.
- [x] 4.3 작업 내용과 결정 사항을 AI 컨텍스트 문서에 반영한다.
