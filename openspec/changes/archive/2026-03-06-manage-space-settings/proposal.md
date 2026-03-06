## Why

Space metadata is partially modeled in the product, but operators still manage spaces through scattered entry points and cannot update core settings after creation. This creates avoidable rework and inconsistent operational UX right where space administration is already the next product priority.

## What Changes

- Consolidate Space administration around `Settings > Spaces` while keeping sidebar creation as the fast entry point.
- Add editable Space management for operator-facing fields that matter now: Space name and quota.
- Keep existing `space.write` authorization semantics and apply them to Space management actions without introducing a new permission split.
- Remove description editing from the near-term scope so the first iteration stays focused on operationally critical controls.
- Ensure Space updates propagate cleanly to the settings screen, sidebar, and selected Space state without requiring a full page reload.

## Capabilities

### New Capabilities
- `space-settings-management`: Manage Space name and quota from Settings while preserving sidebar-based Space creation.

### Modified Capabilities
- None.

## Impact

- Frontend Space management UI in settings and sidebar-adjacent state flows
- Frontend Space store synchronization for create/update/delete refresh behavior
- Backend Space API surface for editable Space metadata beyond quota-only updates
- Space administration UX and related regression test coverage
