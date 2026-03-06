# file-search-result-exploration Specification

## Purpose
검색 결과 문맥과 점진적 탐색 계약을 명확히 해 중복 파일명 구분과 추가 결과 탐색을 안정적으로 제공한다.

## Requirements
### Requirement: Search API returns search context metadata
The system SHALL return `/api/search/files` responses as an object that includes the current result items, the applied limit, and whether more matches exist beyond the returned items.

#### Scenario: Search response reports truncation
- **WHEN** an authenticated user requests `/api/search/files` with a query that matches more than the requested limit
- **THEN** the response SHALL include only up to `limit` items
- **AND** the response SHALL include `hasMore=true`
- **AND** the response SHALL echo the applied `limit`

#### Scenario: Search response preserves current filtering and ordering
- **WHEN** an authenticated user requests `/api/search/files`
- **THEN** the response SHALL include only results from readable spaces
- **AND** the returned items SHALL preserve the existing name-match ranking order before space/path tie-breaks

### Requirement: Search results expose enough context for navigation
The system SHALL expose each result with enough context for users to distinguish duplicate names across spaces and folders.

#### Scenario: Header suggestions show result context
- **WHEN** the user opens header search suggestions for a query with matching results
- **THEN** each suggestion SHALL show the result name, the space name, and the parent path when available

#### Scenario: Search page rows show result context
- **WHEN** the user views `/search?q=...` results
- **THEN** each result row SHALL show the space name and parent path alongside the existing file metadata

### Requirement: Search page supports progressive exploration of truncated results
The system SHALL let users understand current result volume and request more results without leaving the search page.

#### Scenario: Search page shows current result summary
- **WHEN** the search page finishes loading results for a valid query
- **THEN** the page SHALL show the number of returned results for the current request

#### Scenario: Search page offers more results when truncated
- **WHEN** the search response includes `hasMore=true`
- **THEN** the page SHALL offer a "더 보기" action that increases the requested result limit and reloads the results

#### Scenario: Search page keeps empty and failure states distinct
- **WHEN** the query is too short, returns no matches, or fails to load
- **THEN** the page SHALL continue to show distinct guidance for minimum query length, empty results, and load failure
