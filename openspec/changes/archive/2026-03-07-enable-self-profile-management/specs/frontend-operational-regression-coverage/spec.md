## ADDED Requirements

### Requirement: Profile Settings Regression Coverage
The system SHALL provide frontend regression tests for the self-service profile settings flow.

#### Scenario: Writable user can save nickname changes
- **WHEN** the profile settings screen renders for a user with `profile.write`
- **THEN** the frontend test suite SHALL verify that nickname updates call the profile API and refresh session state on success

#### Scenario: Password validation blocks invalid self-service submission
- **WHEN** the user enters a new password without current password or with mismatched confirmation
- **THEN** the frontend test suite SHALL verify that the profile API is not called and the validation feedback path executes

#### Scenario: Read-only user cannot edit profile
- **WHEN** the profile settings screen renders for a user without `profile.write`
- **THEN** the frontend test suite SHALL verify that edit controls are disabled or hidden and the save action is unavailable
