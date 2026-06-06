# Fork tracking — intarweb/RetroSaveManager

This is a soft fork of [`joeblack2k/RetroSaveManager`](https://github.com/joeblack2k/RetroSaveManager). We track upstream HEAD daily and publish a HEAD-tracked image to `ghcr.io/intarweb/retrosavemanager` for home-infra resilience (decoupled from upstream merge timing and Docker Hub rate limits).

> All forks we manage with the sync-upstream + from-source-build pattern live under the [`intarweb`](https://github.com/intarweb) GitHub org. See the `ghcr-fork-mirror` skill for the canonical recipe.

## Branch model — Model B (canonical)

| Branch | Purpose | Source of truth |
|---|---|---|
| `main` | Upstream-clean tracking. Rebased daily onto `upstream/main`. | upstream + sync CI commits |
| `intarweb-dev` | Deploy track. Auto-regenerated daily as `main + cherry-picks of every open PR from intarweb to upstream`. **This is what publishes `:latest`.** | us (auto-regen) |
| `feat/*`, `fix/*` | Individual patch branches we PR upstream | us |

`intarweb-dev` is regenerated FROM SCRATCH on every sync-upstream run. **If a patch should ride `:latest`, it MUST have an open PR (draft is fine) from intarweb to upstream** — auto-regen only sees PRs in upstream's open-PR list. Branches without a PR are NOT picked up.

## Upstream sync

| Property | Value |
|---|---|
| Upstream | [`joeblack2k/RetroSaveManager`](https://github.com/joeblack2k/RetroSaveManager) |
| Upstream branch tracked | `main` |
| Sync cadence | Daily 06:45 UTC + manual `workflow_dispatch` |
| Sync mechanism | `git rebase upstream/main` on `main`, then auto-regen `intarweb-dev = main + cherry-picks of open PRs` |
| Sync workflow | [`.github/workflows/sync-upstream.yml`](.github/workflows/sync-upstream.yml) |

## Build pipeline

| Property | Value |
|---|---|
| Image | `ghcr.io/intarweb/retrosavemanager` |
| `:latest` source | push to `intarweb-dev` (build-from-source via upstream's adapted `publish-ghcr.yml`) |
| `:sha-<long>` | Every build on `main` and `intarweb-dev` |
| Tagged releases (`v*`) | Built when upstream cuts release tags |
| Multi-arch | linux/amd64, linux/arm64 |
| Build workflow | [`.github/workflows/publish-ghcr.yml`](.github/workflows/publish-ghcr.yml) (customized — fires on push to `main` and `intarweb-dev`; `:latest` only tagged on `intarweb-dev`) |

## Local patches we carry on `intarweb-dev` (vs `upstream/main`)

| Commit | Subject | Status |
|---|---|---|
| (cherry-picked at sync time) | `fix(helper-auth): rebind app-password on stale device ID instead of returning 409` | Open PR [#1](https://github.com/joeblack2k/RetroSaveManager/pull/1) (auto-regen-managed) |

## ⚠️ Unsent patches — NOT currently on `intarweb-dev`

The following auth-mode commits exist on the fork (preserved on the `feat/enforce-auth-mode` branch) but are **NOT currently being applied** to `intarweb-dev` because they have no open PR upstream. Per the auto-regen policy, only commits backed by an open PR are cherry-picked into `:latest`.

To restore these into `:latest`, open draft PR(s) upstream from `intarweb:feat/enforce-auth-mode` (or split into smaller branches and PR each). Once a draft PR exists, the next `sync-upstream` run picks the commits up automatically.

| SHA | Subject | Where it lives now |
|---|---|---|
| `2d8cf04` | `feat(auth): enforce AUTH_MODE=enabled on all non-public endpoints` | `origin/feat/enforce-auth-mode` |
| `8d26dcd` | `fix(auth): pass through helper-identity requests so auto-enroll bootstrap works` | `origin/feat/enforce-auth-mode` |
| `a9ffce2` | `feat(auth): TRUST_REMOTE_USER_HEADER toggle, path-prefix coverage, spoof-resistance, SSE+static tests` | `origin/feat/enforce-auth-mode` (tip) |

The previously-existing CI commit `2bbbe04` (`ci: build :latest from dev branch`) is obsoleted by the new `publish-ghcr.yml` customization on `intarweb-dev` and is intentionally NOT preserved.

## How to consume

```yaml
# docker-compose.yml
services:
  retrosavemanager:
    image: ghcr.io/intarweb/retrosavemanager:latest
```

Pin to a specific build:

```yaml
    image: ghcr.io/intarweb/retrosavemanager:sha-<long-sha>
```

## Maintenance recipes

**Manually re-sync onto upstream + regenerate intarweb-dev:**
```bash
gh workflow run "Sync from upstream + auto-regen intarweb-dev" --repo intarweb/RetroSaveManager
```

**Force a fresh build of `:latest` without an upstream change:**
```bash
gh workflow run "Publish GHCR Images" --repo intarweb/RetroSaveManager --ref intarweb-dev
```

**Promote one of the unsent auth patches to ride `:latest`:**
```bash
# Option A — open a draft PR for the existing branch:
gh pr create --repo joeblack2k/RetroSaveManager \
  --base main --head intarweb:feat/enforce-auth-mode --draft \
  --title "WIP: AUTH_MODE enforcement + TRUST_REMOTE_USER_HEADER" \
  --body "Carrying on intarweb-dev for home-infra; will mark ready when validated upstream."
# Next sync-upstream run cherry-picks all 3 commits onto intarweb-dev → :latest gets them.
```
