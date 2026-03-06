## ADDED Requirements

### Requirement: Settings Space Administration Surface
The system SHALL expose a Space administration surface in `Settings > Spaces` for authenticated users who can read spaces.

#### Scenario: Read-capable user views Space settings
- **WHEN** an authenticated user with `space.read` permission opens `Settings > Spaces`
- **THEN** the screen SHALL list the spaces the user can access
- **THEN** each row SHALL show the current Space name and quota state

#### Scenario: User without space.read cannot manage spaces
- **WHEN** an authenticated user without `space.read` permission opens `Settings > Spaces`
- **THEN** the screen SHALL render the existing no-permission state instead of Space administration controls

### Requirement: Space Name Update From Settings
The system SHALL allow users with `space.write` permission to update a Space name from `Settings > Spaces` without recreating the Space.

#### Scenario: Successful Space rename
- **WHEN** a user with `space.write` permission updates a Space name to a valid unused value from `Settings > Spaces`
- **THEN** the backend SHALL persist the new Space name
- **THEN** the settings list, sidebar, and selected Space state SHALL reflect the new name without a full page reload

#### Scenario: Duplicate or invalid Space name is rejected
- **WHEN** a user submits an empty, invalid, or duplicate Space name
- **THEN** the system SHALL reject the request with a validation error
- **THEN** the previously stored Space name SHALL remain unchanged

### Requirement: Settings Quota Management Remains Available
The system SHALL keep quota management in `Settings > Spaces` as part of the same administration surface.

#### Scenario: Write-capable user updates quota from settings
- **WHEN** a user with `space.write` permission changes a Space quota from `Settings > Spaces`
- **THEN** the system SHALL persist the quota change using the existing quota policy
- **THEN** the updated usage and quota state SHALL be visible after refresh of the settings data

#### Scenario: Read-only user sees quota but cannot edit
- **WHEN** a user with `space.read` permission but without `space.write` permission opens `Settings > Spaces`
- **THEN** the screen SHALL show current quota information
- **THEN** quota editing controls SHALL be disabled or hidden

### Requirement: Sidebar Creation Flow Remains The Primary Quick-Create Entry
The system SHALL preserve the existing sidebar-based Space creation flow while introducing settings-based administration.

#### Scenario: Write-capable user still creates Space from sidebar
- **WHEN** an authenticated user with `space.write` permission opens the main sidebar
- **THEN** the Space creation entry SHALL remain available from the sidebar
- **THEN** creating a Space from the sidebar SHALL continue to add it to the shared Space state used by settings and browsing surfaces
