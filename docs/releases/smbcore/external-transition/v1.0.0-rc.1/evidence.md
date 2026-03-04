# External Transition Evidence (v1.0.0-rc.1)

## Metadata

- Date (UTC): 2026-03-04
- External repo: `https://github.com/lteawoo/smb-core.git`
- External module: `github.com/lteawoo/smb-core`
- Resolved version: `v1.0.0-rc.1`

## A. External Repo Validation

Executed in `/Users/twlee/projects/smb-core`:

```bash
go test ./...
./scripts/check-smb-publication-gates.sh
```

Result:
- `go test ./...` pass
- boundary/compat/integration/publication gates pass

## B. Cohesion Transition Validation (pinned external module)

Executed in `apps/backend`:

```bash
./scripts/check-smb-publication-gates.sh
go test ./...
go build ./...
```

Result:
- `smbcore-boundary` pass
- `smb-compat-baseline` pass
- `smb-publication-gates` pass
- `go test ./...` pass
- `go build ./...` pass

## C. Rollback Rehearsal Validation

Executed in sandbox copy `/tmp/cohesion-smbcore-rollback-sandbox`:

```bash
# import/go.mod rollback to in-repo smbcore
./scripts/check-smb-publication-gates.sh
go test ./...
```

Result:
- `smbcore-boundary` pass (in-repo fallback target)
- `smb-compat-baseline` pass
- `smb-publication-gates` pass
- `go test ./...` pass

## D. Notes

- Cohesion `apps/backend/go.mod` pins `github.com/lteawoo/smb-core v1.0.0-rc.1`.
- Committed shared path has no local `replace` entry.
