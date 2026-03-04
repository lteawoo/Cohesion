# SMBCore External Module Transition Runbook

## Canonical Targets

- External repository: `https://github.com/lteawoo/smb-core.git`
- External module path: `github.com/lteawoo/smb-core`
- Transition working branch: `chore/smbcore-external-v1.0.0-rc.1`

## 1. Transition Procedure (Cohesion -> external module)

1. Create transition branch.
2. Switch imports from `taeu.kr/cohesion/pkg/smbcore` to `github.com/lteawoo/smb-core`.
3. Pin module version in `apps/backend/go.mod` to released tag (`v1.0.0-rc.1` or newer).
4. Do not commit local `replace` wiring in shared branch/CI path.
5. Only use temporary local `replace` for private sandbox rehearsal.

## 2. Parity Gate Commands

```bash
cd apps/backend
./scripts/check-smb-publication-gates.sh
go test ./...
```

Evidence path:
- `docs/releases/smbcore/external-transition/<version>/evidence.md`

## 3. Deterministic Rollback Checkpoint

Known-good rollback checkpoint MUST contain:
- Previous dependency wiring (`taeu.kr/cohesion/pkg/smbcore` imports)
- `apps/backend/go.mod` without `github.com/lteawoo/smb-core` dependency/replace
- Passing gate evidence for rollback snapshot

Restore commands:

```bash
# 1) Revert import wiring to in-repo smbcore package
rg -l 'github.com/lteawoo/smb-core' apps/backend/internal/smb | \
  xargs -I{} perl -0pi -e 's#"github.com/lteawoo/smb-core"#"taeu.kr/cohesion/pkg/smbcore"#g' {}

# 2) Remove external module wiring
cd apps/backend
go mod edit -droprequire=github.com/lteawoo/smb-core
go mod edit -dropreplace=github.com/lteawoo/smb-core
go mod tidy

# 3) Re-run parity gates
./scripts/check-smb-publication-gates.sh
go test ./...
```

## 4. Rollback Rehearsal Rule

Before source-of-truth cutover, maintainers MUST execute at least one rollback rehearsal on a sandbox/worktree and archive evidence.

## 5. Transition Window Exit Criteria

All criteria MUST be satisfied:

- [ ] External repo CI green (`go test ./...`, publication gates)
- [ ] Cohesion transition parity gates green with pinned external module
- [ ] Rollback rehearsal completed and evidence archived
- [ ] First RC tag exists in `smb-core`
- [ ] Release evidence reviewed/approved

When all criteria are met, mark `github.com/lteawoo/smb-core` as SMB core source-of-truth and freeze ad-hoc dual-source edits.
