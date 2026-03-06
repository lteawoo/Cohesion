## ADDED Requirements

### Requirement: Login Flow Regression Coverage
The system SHALL provide frontend regression tests for the login screen covering loading, authenticated redirect, setup-required flow, and login submission validation.

#### Scenario: Authenticated user bypasses login screen
- **WHEN** the login page renders for a user who already has an authenticated session
- **THEN** the frontend test suite SHALL verify that the screen redirects to the requested path or `/`

#### Scenario: Setup-required path is exercised
- **WHEN** setup status reports that bootstrap is required
- **THEN** the frontend test suite SHALL verify that the setup form renders and successful bootstrap logs the user in

#### Scenario: Invalid login submission is rejected locally
- **WHEN** the user submits an invalid username or missing password
- **THEN** the frontend test suite SHALL verify that the login API is not called and the validation message path executes

### Requirement: Settings Navigation Permission Coverage
The system SHALL provide frontend regression tests for permission-based section visibility and section switching in the settings page.

#### Scenario: Permission-gated sections are hidden
- **WHEN** the settings page renders for a user without permission for server, space, or account management sections
- **THEN** the frontend test suite SHALL verify that those menu entries are not rendered

#### Scenario: Available settings sections can be selected
- **WHEN** the settings page renders for a user with the required permissions
- **THEN** the frontend test suite SHALL verify that clicking a section renders the matching settings content

### Requirement: Header Search Interaction Coverage
The system SHALL provide frontend regression tests for the header search interaction in the main layout.

#### Scenario: Header search debounces and loads suggestions
- **WHEN** the user types a query of at least two characters and spaces are connected
- **THEN** the frontend test suite SHALL verify that the search request is debounced and the resulting suggestions are rendered

#### Scenario: Header search routes to full search results
- **WHEN** the user submits a valid header search query
- **THEN** the frontend test suite SHALL verify that navigation moves to `/search?q=...`

#### Scenario: Header search selection opens the matching location
- **WHEN** the user selects a suggestion that belongs to a connected space
- **THEN** the frontend test suite SHALL verify that browse state is updated and navigation returns to the browse page

### Requirement: Server Settings Critical Branch Coverage
The system SHALL provide frontend regression tests for the critical save and restart branches of the server settings screen.

#### Scenario: Invalid server config blocks save
- **WHEN** the settings screen contains an invalid port configuration
- **THEN** the frontend test suite SHALL verify that save and restart actions are blocked by validation behavior

#### Scenario: Save and restart paths surface success or failure
- **WHEN** save or restart requests succeed or fail
- **THEN** the frontend test suite SHALL verify that the corresponding success or error feedback path executes
