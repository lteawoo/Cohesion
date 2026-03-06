## ADDED Requirements

### Requirement: Authenticated users can manage their own profile
The system SHALL provide a self-service profile management surface for authenticated users with `profile.read` and `profile.write` permissions.

#### Scenario: Profile screen shows current account information
- **WHEN** an authenticated user opens `Settings > 내 프로필`
- **THEN** the screen SHALL show the current username, nickname, and role for the logged-in account
- **AND** the screen SHALL expose editable controls only for fields allowed in self-service management

#### Scenario: User without profile write sees read-only controls
- **WHEN** an authenticated user has `profile.read` but does not have `profile.write`
- **THEN** the profile screen SHALL keep current account information visible
- **AND** nickname/password editing controls SHALL be disabled or hidden

### Requirement: Self-service nickname update keeps session state current
The system SHALL allow the logged-in user to update their nickname without going through administrator account management.

#### Scenario: Successful nickname update
- **WHEN** an authenticated user with `profile.write` submits a valid new nickname from the profile screen
- **THEN** the backend SHALL persist the nickname for that same account
- **AND** the frontend SHALL refresh session state so the updated nickname is visible without re-login

#### Scenario: Empty nickname is rejected
- **WHEN** an authenticated user submits an empty or whitespace-only nickname
- **THEN** the request SHALL be rejected with a validation error
- **AND** the stored nickname SHALL remain unchanged

### Requirement: Self-service password change requires current password confirmation
The system SHALL require current password verification before the logged-in user can change their own password.

#### Scenario: Password change succeeds with valid current password
- **WHEN** an authenticated user with `profile.write` submits `currentPassword` matching their existing password and a valid new password
- **THEN** the backend SHALL update that user's password hash
- **AND** the profile update response SHALL indicate success without exposing password data

#### Scenario: Password change fails when current password is missing or invalid
- **WHEN** an authenticated user submits a new password without a matching valid `currentPassword`
- **THEN** the request SHALL be rejected
- **AND** the stored password hash SHALL remain unchanged

#### Scenario: Profile update can omit password change
- **WHEN** an authenticated user updates only nickname and does not provide a new password
- **THEN** the system SHALL not require `currentPassword`
- **AND** the existing password SHALL remain unchanged

### Requirement: Self-service profile updates do not allow privilege changes
The system SHALL keep self-service profile management scoped to the current account's nickname and password only.

#### Scenario: Self-service update ignores or rejects role mutation
- **WHEN** an authenticated user attempts to change role or username through the self-service profile update path
- **THEN** the system SHALL reject the unsupported fields or ignore them according to the API contract
- **AND** the stored role and username SHALL remain unchanged
