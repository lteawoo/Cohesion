# Homebrew Tap Bootstrap

This directory keeps the Cohesion Homebrew formula source before it is published to a dedicated tap repository.

## Current Target

- Tap repository name: `lteawoo/homebrew-cohesion`
- Install command: `brew install lteawoo/cohesion/cohesion`
- Update command: `brew upgrade cohesion`
- Production config/data/secrets: `~/.cohesion`

## Files

- `Formula/cohesion.rb`: formula ready to copy into the tap repository

## Refresh Formula

Render the formula from the latest GitHub release metadata:

```bash
pnpm release:homebrew-formula
```

Render a specific tag:

```bash
pnpm release:homebrew-formula --tag v0.5.18
```

## Publish Checklist

1. Create the `homebrew-cohesion` repository.
2. Copy `Formula/cohesion.rb` into `homebrew-cohesion/Formula/cohesion.rb`.
3. Commit and push the updated tap repository after every release.
4. Add a tap README with install and service instructions.
5. Verify:
   - `brew install lteawoo/cohesion/cohesion`
   - `brew services start cohesion`
   - `brew upgrade cohesion`

Homebrew only upgrades to a new Cohesion release after the tap repository's `Formula/cohesion.rb` is updated and pushed.
