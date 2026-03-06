## 1. Index Storage And Bootstrap

- [x] 1.1 Add search index schema/migration and a repository abstraction in the backend.
- [x] 1.2 Implement startup bootstrap/repair flow so existing spaces are indexed before indexed search is used.

## 2. Incremental Sync And Query Path

- [x] 2.1 Wire successful file and space mutations to index updates or dirty-space marking logic.
- [x] 2.2 Refactor `/api/search/files` to query the index while preserving current filtering, ranking, `limit`, and `hasMore` semantics.
- [x] 2.3 Add dirty-space recovery logic that reindexes affected spaces after synchronization failures.

## 3. Verification And Rollout

- [x] 3.1 Add backend tests for bootstrap, mutation sync, dirty recovery, and search contract preservation.
- [x] 3.2 Validate indexed search behavior against representative multi-space fixtures and compare it with the current walk-based semantics.
- [x] 3.3 Update `docs/ai-context` with the proposal, issue, and rollout status for this work.
