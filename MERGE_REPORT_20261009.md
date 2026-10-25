# Full Sync Merge Report

> Latest status: see section 10 (2026-10-25). Sections 1-9 are preserved as the historical 2026-10-09 record.

- Report date: 2026-10-09
- Branch: `sync/frontend-20260805`
- Current HEAD: `c2d756744`
- State: full sync completed locally; ready for Beta testing on `http://127.0.0.1:3010`
- Remote deployment: not performed

## 1. Merge Target

| Item | Value |
| --- | --- |
| Upstream branch | `upstream/main` |
| Upstream commit | `7aa3531ef` |
| Pre-merge branch commit | `0a1a11dbf` |
| Merge base | `c2b7a9a9e0b548c2051a949fceabb59029adcb49` |
| Merge commit | `dc8223b10` |
| Post-merge Docker fix | `c2d756744` |
| Backup branch | `backup/pre-full-sync-20261009` at `0a1a11dbf` |
| Ancestor check | `upstream/main` is an ancestor of current `HEAD` |

## 2. Merge Scope

- Upstream range: `c2b7a9a9e0b548c2051a949fceabb59029adcb49..7aa3531ef`
- Upstream commits: 40
- Upstream diff: 301 files changed, 19,756 insertions, 3,735 deletions
- Net branch diff from pre-merge `0a1a11dbf` to current `HEAD`: 303 files changed, 19,765 insertions, 3,742 deletions

Main integrated areas include:

- `tokenkit` token-counting module and root-module integration
- `moejs`/JS task plugin upgrades and PGO release build support
- Relay conversion, Responses/Chat/Claude/Gemini compatibility, and billing updates
- Access token, step-up verification, authentication, and audit changes
- Default/classic frontend builds and static asset serving updates
- New xAI, Doubao, Seedream, Gemini, and web-search provider behavior
- Frontend permission, security, pricing, usage-log, and i18n updates

## 3. Conflict Resolution

The merge commit recorded three conflicts:

| File | Resolution |
| --- | --- |
| `Dockerfile` | Combined upstream dual-frontend builds, `tokenkit`, PGO build, and license copying with this branch's `GOWORK=off` and local module handling. |
| `relay/channel/zhipu_4v/adaptor.go` | Preserved GLM-5.3 `reasoning_effort` normalization and Responses URL behavior while retaining upstream additions. |
| `router/web-router.go` | Preserved theme-aware default/classic frontend routing while adopting upstream static serving, compression, cache, and rate-limit integration. |

Private behavior retained by the merged tree includes:

- GLM-5.3 reasoning normalization
- Responses image stripping and context recovery logic
- Generated Codex tool-schema normalization
- Model numeric-tag filtering
- Default/classic theme-aware frontend routing

Upstream behavior adopted includes:

- `tokenkit` module and calibrated token counting
- `GOWORK=off` module builds
- PGO build path
- License copying into the runtime image
- Updated frontend compression and static serving

## 4. Post-Merge Fixes

### Context Recovery Compatibility

`controller/context_recovery.go` used the removed `service.CountTextToken` API. The merged code now uses:

```go
tokenkit.Count(model, text)
```

### Local Dist Build

The first local image build exposed that `Dockerfile.localdist` also needed the new `tokenkit` module layout. Commit `c2d756744` adds:

- `GOWORK=off`
- Copying `tokenkit/go.mod` before `go mod download`
- Correct comments describing both local modules

## 5. Verification Record

| Area | Command / Check | Result |
| --- | --- | --- |
| Root Go tests | `GOWORK=off go test ./...` | Passed |
| Relaykit tests | `cd relaykit && GOWORK=off go test ./...` | Passed |
| Tokenkit tests | `cd tokenkit && GOWORK=off go test ./...` | Passed |
| Frontend typecheck | `bun run typecheck` | Passed |
| Default frontend build | `bun run build` | Passed |
| Classic frontend build | `bun run build` in `web/classic` | Passed |
| Frontend full tests | `./node_modules/.bin/vitest run --maxWorkers=2 --testTimeout=60000` | 175 files and 2,171 tests passed |
| Local image build | `docker build -f Dockerfile.localdist -t new-api-v2:local .` | Passed |
| API health | `GET http://127.0.0.1:3010/api/status` | HTTP 200 |
| Frontend health | `GET http://127.0.0.1:3010/` | HTTP 200 |
| Static assets | Default JS and CSS requests | HTTP 200 |

Known test observations:

- Plain parallel `bun run test` hit timeout/resource contention. A full run limited to two workers passed.
- `bun run lint` is not green and reports many pre-existing errors, especially under `web/classic`. This was not introduced by the merge and remains unresolved.
- Real PostgreSQL and SQLite migration/runtime verification was not executed in this merge pass. Local runtime validation used MySQL 8.2.0. The merge therefore must not be claimed as fully verified across all supported databases.

## 6. Local Runtime

| Item | Value |
| --- | --- |
| Application URL | `http://127.0.0.1:3010` |
| Container | `new-api-3010` |
| Port mapping | `127.0.0.1:3010 -> 3000` |
| Network | `new-api-net` |
| Database | Local MySQL database `new-api` |
| Redis | Enabled |
| Current image | `new-api-v2:local` |
| Current image ID | `sha256:6d93b0faaf9df99779acd3c7971274172e184898bdbce075336db11758bda2ad` |
| Candidate tag | `new-api-v2:local.candidate-logo` (same ID) |
| Previous image tag (rollback) | `new-api-v2:local.pre-logo-20261009` |
| Previous image ID (rollback) | `sha256:c6e9e3f58cf6e78e7394f7d03c285fceddee79d0a90630dea7b025d47745d514` |
| Older synced image tag | `new-api-v2:local.pre-sync-20261009` |
| Older synced image ID | `sha256:32972bcd2bf10361c6e3c076f4bf2ff118dabb17e74d4fcf1264fd9c50a7bba0` |

Database backup:

- File: `/Volumes/D/new-api-v2-backups/new-api-before-full-sync-20261009.sql.gz`
- Size: 189 KB compressed
- Gzip integrity: passed
- Snapshot counts at backup time: 37 tables, 1 user, 3,284 logs, 14 channels

## 7. Rollback

Application image rollback:

```bash
docker tag new-api-v2:local.pre-sync-20261009 new-api-v2:local
docker rm -f new-api-3010
```

Recreate `new-api-3010` using the existing environment and mounts, or restore the container configuration used before the rebuild.

Database rollback should only be performed if Beta testing reveals an irreversible data or migration issue:

```bash
gunzip -c /Volumes/D/new-api-v2-backups/new-api-before-full-sync-20261009.sql.gz | docker exec -i mysql mysql -uroot -p new-api
```

## 8. Icon Regression Fix and TLS Certificate Renewal (2026-10-09)

### 8.1 Header Icon Missing (`https://www.coai-api.cn/logo6.png`)

Symptom: after the sync the site header/footer logo disappeared.

Root cause: the configured logo URL was served over HTTPS from a host whose certificate had expired. Browsers blocked the failed TLS image request, and the client-side logo loader treated the failure as a final state, leaving a broken placeholder with no fallback.

Fix in `web/src/hooks/use-system-config.ts`:

- On remote logo load failure, fall back to the bundled `DEFAULT_LOGO` instead of leaving the broken URL.
- Sync the Zustand config (`setConfig({ logo: DEFAULT_LOGO })`), `loadedLogoUrl`, and the favicon so the header, store, and tab icon stay consistent.
- The effect sees `loadedLogoUrl` equal to `config.logo` after the fallback and does not retry the failing remote URL (no retry loop).

Bundled fallback asset (served locally so it survives remote/TLS failures):

| File | SHA-256 |
| --- | --- |
| `web/public/logo6.png` | `1e91cd24020af58e9eaeef174512c1e964d17aaa55bdfedf63350292a7456a10` |
| `web/classic/public/logo6.png` | `1e91cd24020af58e9eaeef174512c1e964d17aaa55bdfedf63350292a7456a10` |

Both hashes match the served remote image; the bundled asset is a 1229x1229 PNG (274,474 bytes).

Regression test: `web/src/hooks/__tests__/system-config-logo.test.tsx` asserts that a remote logo load failure falls back to `DEFAULT_LOGO`.

- Command: `cd web && bun run test src/hooks/__tests__/system-config-logo.test.tsx` (vitest)
- Result: 1 test file passed, 1 test passed.

### 8.2 TLS Certificate for `coai-api.cn` (zh host)

`www.coai-api.cn` resolves to the zh host `124.220.165.189` and is served by the Jitsi nginx (not the new-api gateway container).

Root cause: the previous TrustAsia certificate expired on 2026-10-04 12:59:59 GMT.

Fix applied on the zh host:

- Installed `certbot` and issued a Let's Encrypt certificate covering `coai-api.cn` and `www.coai-api.cn`.
- New certificate validity: 2026-10-09 07:48:24 GMT to 2027-01-07 07:48:23 GMT; issuer `Let's Encrypt (YE2)`.
- Updated Jitsi nginx key files (backups kept as `.bak-20261009-cert-renew`):
  - `/root/.jitsi-meet-cfg/web/keys/coai-api.cn_bundle.crt`
  - `/root/.jitsi-meet-cfg/web/keys/coai-api.cn.key`
- `nginx -t` passed and nginx was hot-reloaded.
- Renewal deploy hook installed: `/etc/letsencrypt/renewal-hooks/deploy/coai-jitsi.sh` (reinstalls the renewed cert into the Jitsi nginx key paths and reloads nginx).
- `certbot.timer` is enabled; next scheduled run 2026-10-10 05:15:51 CST.

External verification: `https://www.coai-api.cn/logo6.png` returns HTTP 200 with `content-type: image/png`; certificate is valid for `coai-api.cn` and `www.coai-api.cn`.

Renewal verification: `certbot renew --dry-run` on the zh host.

- First attempt failed on Let's Encrypt "secondary validation" (a single ACME validation viewpoint timed out on port 80); the primary validation served the challenge token with HTTP 200.
- Second attempt passed: "Congratulations, all simulated renewals succeeded: /etc/letsencrypt/live/coai-api.cn/fullchain.pem (success)".

Because the http-01 challenge is validated from multiple external vantage points, an occasional single-viewpoint timeout can occur; the renewal path itself is functional and managed by `certbot.timer`. The standalone challenge serves on port 80, so nothing needs to listen on 80 between renewals.

Final certificate source change (2026-10-09, later): the user applied for a cloud-issued certificate and wanted that one deployed instead of the Let's Encrypt cert.

- Deployed cert bundle: `~/Downloads/coai-api.cn_nginx/` (TrustAsia DV TLS RSA CA 2024, RSA-2048, chain of 3).
- Validity: 2026-10-09 10:00:00 GMT to 2027-01-07 09:59:59 GMT; SANs `coai-api.cn`, `www.coai-api.cn`.
- Installed to the zh Jitsi nginx key paths; previous Let's Encrypt files kept as `.bak-20261009-tencent`.
- `nginx -t` passed, nginx reloaded; external `openssl s_client` shows the TrustAsia cert on both hostnames with `Verify return code: 0 (ok)`.
- Decision (2026-10-09, final): Let's Encrypt auto-renewal is the source of truth; no manual cloud certificate is needed.
  - The certbot deploy hook was re-enabled and hardened: it now discovers the Jitsi web container dynamically instead of hardcoding `docker-jitsi-meet-web-1`, and installs `fullchain.pem`/`privkey.pem` then reloads nginx.
  - The hook was run once; nginx now serves the Let's Encrypt cert (`issuer CN=YE2`, valid to 2027-01-07), confirmed externally with `Verify return code: 0 (ok)` on both hostnames.
  - `certbot.timer` is enabled (`OnCalendar=00,12`, `RandomizedDelaySec=43200`, `Persistent=true`); `certbot.service` runs `certbot -q renew --no-random-sleep-on-renew`.

Why http-01 renewals can occasionally fail:

- The http-01 challenge needs inbound TCP port 80 reachable from every Let's Encrypt validation vantage point. On this China-hosted host, repeated `certbot renew --dry-run` runs were intermittent: some succeeded, some failed with `During secondary validation: 124.220.165.189: Fetching http://coai-api.cn/.well-known/acme-challenge/... Timeout during connect (likely firewall problem)`.
- Host-side firewalling is not the cause: `ufw` is inactive, the `YJ-FIREWALL-INPUT` iptables chain only rejects port 22 from brute-force sources, and DNS has A records only (no AAAA), so IPv6 is not involved.
- This is a partial reachability issue on port 80 from some ACA/Let's Encrypt secondary-validation network paths, not a certbot misconfiguration.
- Practical impact is low: `certbot.timer` attempts renewal twice daily starting 30 days before expiry (~60 attempts), so a single transient validation failure self-heals on the next run, and the deploy hook installs the renewed cert automatically.
- Fully deterministic alternative if desired: switch `coai-api.cn` to a DNS-01 challenge. The domain is on DNSPod (`ursula.dnspod.net` / `bad.dnspod.net`), which has API-based DNS-01 plugins. DNS-01 does not depend on inbound port 80 reachability, but it requires a DNSPod API token from the user.

### 8.3 Local Image Build (Docker Hub metadata workaround)

The standard `Dockerfile.localdist` build failed because Docker Hub metadata lookups for the pinned base image digests timed out on this host.

Workaround: built the same runtime from local base images so no registry pull was required.

- Builder stage based on local `golang:1.26.1-alpine`.
- Runtime stage based on local `new-api-v2:local.pre-logo-20261009`, replacing only `/new-api` with the freshly built binary that embeds the latest default/classic frontend dist.
- Temporary files used for this build: `Dockerfile.localdist.offline` and `Dockerfile.localdist.offline.dockerignore` (removed after verification).
- This workaround only affects image construction; the Go binary and embedded frontend are identical to what `Dockerfile.localdist` would produce.

### 8.4 Post-Rebuild Verification (port 3010)

| Check | Command / Method | Result |
| --- | --- | --- |
| API status | `curl -fsS http://127.0.0.1:3010/api/status` | HTTP 200 |
| Config logo preserved | decode `data.logo` from `/api/status` | `https://www.coai-api.cn/logo6.png` (unchanged; no DB edit) |
| Container startup | `docker logs new-api-3010` | Migration OK, "ready", no panic/fatal/error |
| Remote logo TLS | `curl -I https://www.coai-api.cn/logo6.png` | HTTP 200, `image/png` |
| Bundled logo asset | `GET http://127.0.0.1:3010/logo6.png` | HTTP 200, `image/png`, 274474 bytes |
| Browser render | headless check of homepage `<img>` | header + footer logo `naturalWidth=1229`, `complete=true`; favicon also loads |
| Logo fallback test | `cd web && bun run test src/hooks/__tests__/system-config-logo.test.tsx` | 1 passed |
| Built bundle contains fallback | grep dist for "Failed to load logo" | present in `web/dist/static/js` |

Container `new-api-3010` was recreated from image `new-api-v2:local` (`sha256:6d93b0faaf9d...`), preserving its previous network, port mapping, mounts, environment, entrypoint, working directory, and restart policy (`no`). No database change was made.

## 9. Remaining Work

1. Complete local Beta testing on port 3010.
2. Decide whether PostgreSQL and SQLite runtime verification is required before remote rollout.
3. After explicit approval, deploy to `zh` with `./update_new_api_zh.sh`.
4. Verify the `zh` deployment status and application behavior.
5. After explicit approval, deploy to `oc` with `./update_new_api_oc.sh`.
6. Verify the `oc` deployment status and application behavior.

No remote deployment should be performed until the local Beta test is accepted.

## 10. 2026-10-25 Local Sync and Verification Addendum

This section is the current status and supersedes the 2026-10-09 runtime, verification, and deployment-status details. The earlier sections remain in place as the historical record.

### 10.1 Upstream synchronization

| Item | Value |
| --- | --- |
| Branch | `sync/frontend-20260805` |
| Current merge commit | `3fbb83a6c` |
| Merge parents | `ab3746638`, `e835b1bfc` |
| Previous merge baseline | `7aa3531ef` |
| Upstream branch | `upstream/main` at `e835b1bfc` |
| Upstream commits synchronized | 124 |
| Net diff from pre-sync `ab3746638` to `HEAD` | 712 files changed, 36,065 insertions, 8,585 deletions |
| Backup branch | `backup/pre-full-sync-20261025` at `ab3746638` |
| Ancestor check | `upstream/main` is an ancestor of `HEAD`; `git rev-list --count HEAD..upstream/main` returned `0` |

The 124 commits were integrated with a true merge commit rather than squashed. There were no unresolved conflicts, and the existing private behavior was retained. The branch is now aligned with the current locally fetched official `upstream/main`.

### 10.2 Follow-up fixes

1. Local image build toolchain: `Dockerfile.localdist` now uses the official `golang:1.26.10-alpine` digest instead of the earlier `1.26.1` digest. The obsolete builder line was removed rather than leaving a duplicate stage.
2. Registration email policy: `controller/user.go` now calls `service.ValidateAccountEmail` before email verification is sent or checked. This closes a policy-bypass window when account email rules changed after a verification code had been issued. Coverage includes normalization, `+alias`, dot aliases, and disposable-domain rejection in `service/email_binding_test.go` and `controller/auth_flow_test.go`.
3. Tool schema preservation: `relaykit/relayconvert/schema_normalize_test.go` now verifies that schemas requiring no transformation retain their exact JSON bytes and property order, and that transformed `$ref` schemas still resolve correctly.
4. Logo fallback: `web/src/hooks/use-system-config.ts` now switches the store, loaded logo state, favicon, and rendered logo to the bundled `DEFAULT_LOGO` when a remote logo fails. The fallback assets in `web/public/logo6.png` and `web/classic/public/logo6.png` both have SHA-256 `1e91cd24020af58e9eaeef174512c1e964d17aaa55bdfedf63350292a7456a10`.

Authentication-related changes were reviewed against OWASP ASVS 5.0.0 and the Authentication and Session Management Cheat Sheets:

- <https://asvs.owasp.org/>
- <https://cheatsheetseries.owasp.org/cheatsheets/Authentication_Cheat_Sheet.html>
- <https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html>

Exact ASVS requirement IDs were not recorded because the ASVS CSV request timed out. No claim of full ASVS compliance is made.

### 10.3 Verification record

| Area | Verification | Result |
| --- | --- | --- |
| Root Go module | `GOWORK=off go test ./...` | Passed |
| `relaykit` module | `GOWORK=off go test ./...` from `relaykit/` | Passed |
| `tokenkit` module | `GOWORK=off go test ./...` from `tokenkit/` | Passed |
| SQLite migration/runtime | Real SQLite `3.50.4`; fresh database and upgrade from the previous image; startup repeated | Passed; schema stable at 38 public tables |
| MySQL migration/runtime | Real MySQL `8.2.0`; fresh database and upgrade from the previous image; startup repeated | Passed; schema stable at 38 public tables |
| PostgreSQL migration/runtime | Real PostgreSQL `15.19`; fresh database and upgrade from the previous image; startup repeated | Passed; schema stable at 38 public tables |
| Upgrade schema hash, MySQL | Post-start schema dump hash | `e6a7a9c4238c0676735705d0128b3e83e17c9e598eaee757de599343e37f36bd` |
| Upgrade schema hash, PostgreSQL | Post-start schema dump hash | `171aec64c56d72b42c4645a7058d0e5e3249bc1480f6c3de33c06032de9ab379` |
| Upgrade schema hash, SQLite | Post-start schema dump hash | `a9a3982d9bdedd7458fce3f6df6dba35b547aafc79de6bb3eea7c5b9d2f0c0e8` |
| Frontend typecheck | `bun run typecheck` from `web/` | Passed |
| Default frontend build | `bun run build` from `web/` | Passed |
| Classic frontend build | `bun run build` from `web/classic/` | Passed |
| Focused registration policy tests | Controller and service regression tests | Passed |
| Focused relaykit schema tests | Relaykit schema normalization tests | Passed |
| Focused logo fallback test | Vitest logo regression test | Passed |
| Frontend lint | `oxlint` | No errors; 4 pre-existing warnings |
| Diff hygiene | `git diff --check` | Passed, no whitespace errors |

The temporary MySQL and PostgreSQL verification containers and volumes were removed after the matrix completed. The local `mysql`, `redis`, and `new-api-3010` containers were retained.

Verification gaps:

- The full frontend test suite hung for more than 16 minutes and was interrupted. It must not be reported as fully passing for this pass.
- A standard `docker build -f Dockerfile.localdist -t new-api-v2:local .` was attempted, but Docker Hub metadata retrieval timed out. The active image was produced with the cached Linux build chain and contains the runtime fixes, but the standard registry-dependent build path was not independently completed.

### 10.4 Local port 3010

| Item | Value |
| --- | --- |
| URL | `http://127.0.0.1:3010` |
| Container | `new-api-3010`, running |
| Current image | `new-api-v2:local` |
| Current image ID | `sha256:1eb8093fcae3f880faf7c244d104b3fd517658bf43c77c0133925977cbc5b280` |
| Rollback tag | `new-api-v2:local.bak-20261025` |
| Rollback image ID | `sha256:0fb338bf73ad4b6ebf1a3f004fa511891ba0003f00583ab9fa49b34e257e6660` |
| Frontend mode | `theme.frontend=default` |
| Registration policy | Email alias restriction and domain restriction enabled; allowlist `gmail.com,wykj.asia` |
| HTTP checks | `/api/status` 200, `/` 200, `/logo6.png` 200 |
| Recent logs | No panic, fatal, or error entries in the last 10 minutes |
| Browser check | Headless Chrome render captured at `/tmp/newapi-3010-final.png` |
| Database cleanup | Temporary registration test user deleted with `model.HardDeleteUserById`; only `admin` remains |

The running container preserved its environment, network, mounts, entrypoint, and restart policy when recreated from the current image.

Classic frontend note: persistent classic configuration is migrated back to `default`, and the official code no longer permits enabling classic through configuration. The classic production build passed, but the live 3010 route is not claimed as a verified classic-frontend runtime.

### 10.5 Kimi issue 2

Issue 2 refers to an upstream Kimi account balance arrears condition. It does not mean that this deployment disabled the Kimi channel in new-api. Recharging the upstream account should address the upstream arrears, but a real request must be run after the recharge before declaring the channel recovered. No such post-recharge recovery test was performed in this pass.

### 10.6 Current rollout status and remaining work

1. Local testing on `http://127.0.0.1:3010` is available for review.
2. After a Kimi recharge, run an authenticated end-to-end Kimi request and inspect both the upstream response and new-api logs.
3. Run the full frontend test suite in a controlled environment before remote rollout, because the previous attempt hung.
4. Do not deploy to `oc` or `zh` until the user explicitly authorizes it. No remote deployment was performed in this pass.
