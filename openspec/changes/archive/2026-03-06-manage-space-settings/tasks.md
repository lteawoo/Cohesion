## 1. Backend Space rename support

- [x] 1.1 Add Space service/store update support for `space_name` with empty-value and duplicate-name validation.
- [x] 1.2 Expose a write-gated Space metadata update handler on `/api/spaces/{id}` for rename-only updates in this iteration.
- [x] 1.3 Add backend tests covering successful rename and invalid/duplicate rename failures.

## 2. Frontend shared Space state updates

- [x] 2.1 Extend the Space store with a rename mutation flow that refreshes the shared Space list after success.
- [x] 2.2 Reconcile `selectedSpace` by Space ID after create/rename/quota mutations so sidebar and browse state stay in sync.
- [x] 2.3 Add or update frontend store/component tests for shared Space state refresh behavior.

## 3. Settings and sidebar UX

- [x] 3.1 Expand `Settings > Spaces` to list accessible Spaces with name editing and existing quota controls in one administration surface.
- [x] 3.2 Hide or disable rename/quota editing actions for users without `space.write` while preserving readable quota/name information for `space.read` users.
- [x] 3.3 Verify sidebar Space creation remains available and unchanged for `space.write` users.

## 4. Verification and documentation sync

- [x] 4.1 Run frontend typecheck/tests and backend Go tests for the affected Space management paths.
- [x] 4.2 Perform manual verification for rename, quota update, read-only access, and sidebar creation regression.
- [x] 4.3 Update project context documents (`docs/ai-context/status.md`, `todo.md`, `decision_log.md`) with implementation outcomes if behavior or scope changes during apply.
