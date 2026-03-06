## 1. Backend Membership Contract

- [x] 1.1 Add `GET /api/spaces/{id}/members` and `PUT /api/spaces/{id}/members` routes, request/response types, and supporting service/store methods.
- [x] 1.2 Implement permission gating, payload normalization, and `space.members.replace` audit recording for space-centric membership updates.
- [x] 1.3 Add backend tests for member listing, replacement, validation failures, and denied access.

## 2. Frontend Space Settings

- [x] 2.1 Add API client methods and local state for loading and saving space membership data.
- [x] 2.2 Extend `Settings > Spaces` with a member list and edit controls gated by the read/write membership contract.
- [x] 2.3 Refresh the relevant space/account views after save so account-centric and space-centric permission screens stay in sync.

## 3. Verification And Context

- [x] 3.1 Add frontend regression coverage for read-only visibility, successful save, and validation/denied states.
- [x] 3.2 Perform manual UI verification with screenshots for member listing, permission editing, and hidden controls.
- [x] 3.3 Update `docs/ai-context` with the proposal, issue, and rollout status for this feature.
