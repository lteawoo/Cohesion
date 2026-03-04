# External Transition Evidence (v0.0.0-bootstrap)

## Metadata

- Date (UTC): 2026-03-04
- External repo: `https://github.com/lteawoo/smb-core.git`
- External module: `github.com/lteawoo/smb-core`
- Cohesion branch context: local transition workspace

## A. External Repo Bootstrap Validation

Executed in `/Users/twlee/projects/smb-core`:

```bash
go mod tidy
go test ./...
./scripts/check-smb-publication-gates.sh
```

Result:
- `go test ./...` pass
- boundary/compat/integration/publication gates pass

## B. Cohesion Transition Validation (external module wiring)

Executed in `apps/backend`:

```bash
./scripts/check-smb-publication-gates.sh
go test ./...
```

Result:
- `smbcore-boundary` pass (external module target)
- `smb-compat-baseline` pass
- `smb-publication-gates` pass
- `go test ./...` pass

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

- Current Cohesion wiring is transition mode with pinned external dependency and local replace for bootstrap.
- Before production cutover, remove local replace and pin RC/stable tag.
