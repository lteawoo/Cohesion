## Context

The current product already stores `space_name`, `quota_bytes`, and additional metadata fields on the Space model, but operational management is fragmented. Sidebar creation remains the only practical creation entry, settings currently focus on quota-only editing, and there is no post-create rename flow. This change needs coordination across frontend settings UI, shared Space store state, and backend Space APIs.

## Goals / Non-Goals

**Goals:**
- Turn `Settings > Spaces` into the primary administration surface for existing Spaces.
- Support Space rename without requiring delete-and-recreate workflows.
- Keep quota management in the same settings surface.
- Preserve sidebar creation as the fast create entry.
- Keep authorization simple by reusing `space.read` and `space.write`.
- Ensure create/update actions refresh shared Space state consistently across sidebar, settings, and selected Space context.

**Non-Goals:**
- Editing `space_desc`, `icon`, `space_category`, or `space_path` in this change.
- Introducing new Space permissions.
- Replacing the sidebar creation flow with settings-based creation.
- Performing filesystem path migration for existing Spaces.

## Decisions

### Decision: Keep sidebar creation and move existing-Space management into settings
- Rationale: Sidebar creation is already discoverable and covered by existing behavior. The missing operational gap is post-create administration, not creation entry itself.
- Alternatives considered:
  - Move creation fully into settings: rejected because it removes a fast, already-working entry point.
  - Leave administration split between sidebar and settings: rejected because rename/quota ownership would remain unclear.

### Decision: Add a dedicated metadata update endpoint for `/api/spaces/{id}` and keep quota updates on `/api/spaces/{id}/quota`
- Rationale: Name update and quota update already have different validation and failure modes. Keeping quota on its existing endpoint minimizes regression risk and lets the settings UI compose two focused operations.
- Alternatives considered:
  - Replace everything with a single generic Space update endpoint including quota: rejected for this iteration because it broadens the change surface and increases ambiguity around validation and backward compatibility.
  - Keep rename as a frontend-only state patch: rejected because Space name is persisted backend state.

### Decision: Restrict editable fields in this iteration to `space_name`
- Rationale: The agreed scope is to solve the highest-value operational gap first. Supporting name update now delivers immediate value without dragging in lower-priority metadata editing.
- Alternatives considered:
  - Also edit description/icon/category: rejected because those fields are not currently essential to the operator workflow and would expand UI/API/test scope.
  - Allow `space_path` updates: rejected because it implies filesystem migration and rollback complexity.

### Decision: Reconcile shared Space state by Space ID after mutations
- Rationale: Rename changes the display label used across the sidebar and the selected Space object, but identity should stay stable by ID. After create/update/quota mutations, the frontend should refresh the Space list and remap `selectedSpace` by ID to avoid stale labels.
- Alternatives considered:
  - Patch only the local screen state: rejected because sidebar and browse state can drift.
  - Force full page reload after mutation: rejected because it degrades UX and is unnecessary.

### Decision: Reuse `space.write` for rename and quota mutations
- Rationale: Existing permissions already distinguish read vs write administration. Adding a new permission split now would require RBAC, UI, and test churn without clear product value.
- Alternatives considered:
  - Add `space.manage` or rename-specific permissions: rejected for scope and migration cost.

## Risks / Trade-offs

- [Selected Space object becomes stale after rename] -> Refetch Space list after mutation and reconcile `selectedSpace` by ID.
- [Split endpoints mean name and quota are not atomically updated together] -> Keep UI save actions separate and communicate success/failure per field group.
- [Inline or row-based editing adds per-row async complexity] -> Use row-scoped draft/loading state instead of a shared global mutation state.
- [Existing tests may not cover shared-state refresh paths] -> Add focused frontend tests for settings-to-sidebar synchronization and backend tests for rename validation.

## Migration Plan

1. Add backend Space rename support on `/api/spaces/{id}` with validation for empty and duplicate names.
2. Extend frontend Space store with update flow and selected Space reconciliation.
3. Expand `Settings > Spaces` to show Space name management alongside existing quota controls.
4. Verify sidebar creation remains unchanged and that shared state refreshes after create/update/quota mutations.
5. Rollback path: revert the new rename endpoint and settings UI changes; quota endpoint and sidebar creation remain on existing behavior.

## Open Questions

- None for the current agreed scope. Description editing, additional metadata editing, and path changes are intentionally deferred.
