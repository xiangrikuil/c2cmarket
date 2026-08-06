# Design

日期：2026-07-26
执行者：Codex

## Architecture

本任务保持现有 Go `net/http` + chi、模块 Service、PostgreSQL Repository、Vue 前端和 Compose 部署结构。新增能力进入现有边界：

- 身份与 Bootstrap：`internal/module/auth` 定义行为，`internal/store/postgres/auth.go` 保证事务与唯一约束。
- 安全出站：新增单一 `internal/platform/outboundhttp` 组件，模型审计适配器必须注入该客户端，不允许业务模块自行创建宽松 `http.Client`。
- 客户端 IP：继续由 Server 中间件解析，输出一个请求级标准化值供限流、审计和日志复用。
- 构建信息：新增小型 buildinfo 包，通过 Go ldflags 注入版本、Git SHA、构建时间；迁移目标来自 `database.ExpectedMigrationVersion`。
- 清理任务：同一二进制增加明确 worker/maintenance 子命令或进程内可关闭 runner，PostgreSQL advisory lock 保证多实例单执行。
- 数据库池：配置层解析/校验，database 层构建 `pgxpool.Config` 并设置连接创建后的数据库会话超时。
- 可观测性：复用现有请求 ID 和日志中间件，新增小型指标接口；不在领域模块散落第三方 SDK。

## Task Slices

| Slice | Scope | Independent Gate |
| --- | --- | --- |
| Identity | OAuth identity-first transaction; unique handle; bootstrap provenance and fail-closed conflict | Auth unit + PostgreSQL integration + migration |
| Outbound/network | Safe HTTPS client; DNS/rebinding/redirect/body/timeout; production listener/Compose; trusted IP reuse | Deterministic resolver/dialer tests + Compose config assertions |
| Release | Commit-only source archive; build metadata; Docker labels/ldflags; OpenAPI/migration checks | Archive inspection + `/version` + Docker build |
| Data lifecycle | HMAC verification codes; atomic attempts; cleanup lease/worker; idempotency retention/body cap | Repository integration + worker lease tests |
| Runtime hardening | Bounded limiter; keyring/AAD; pgxpool config/timeouts; CSP; observability | Unit/integration/header tests |
| CI/docs | Local-equivalent jobs, scanners, SBOM, migration DB, release gate, runbooks | Workflow static validation + local commands |
| P2 | Focused file splits and OpenAPI-generated types only after prior gates | No contract/test regression |

## Identity Data Flow

Existing login:

`provider profile -> query auth_identities(provider, subject) -> bound users row -> update only non-security profile fields -> session`

First login:

`provider profile -> transaction -> recheck identity -> reserve unique local handle -> insert users -> insert auth_identities -> insert/update provider-specific binding for same user -> commit`

The normal login path never updates `auth_identities.user_id`. A unique-constraint race rolls back the losing transaction, then reloads the winning identity in a fresh query. Handle generation uses a normalized provider/subject-derived suffix so repeated retries produce the same candidates while never selecting an existing user.

Bootstrap uses a durable provenance row or equivalent immutable marker tied to the created user. An existing matching marker makes reruns idempotent. Any username/unique-email/credential/identity collision without that marker fails. Bootstrap never upgrades an existing user and never updates an existing password.

## Safe Outbound Data Flow

`validated base URL -> resolve all host IPs -> reject non-public/special ranges -> create request -> transport DialContext dials one validated resolved IP while preserving TLS ServerName -> bounded response`

- Only HTTPS, no URL credentials, fragments, invalid ports, opaque URLs, IP zone identifiers, or noncanonical hosts.
- Redirects disabled by default.
- Resolver and dialer are interfaces for deterministic tests.
- Every resolved address is checked, including IPv4-mapped IPv6 after `Unmap`.
- Connection, TLS handshake, response header, total request and body limits are explicit.
- Optional production allowlist matches normalized hostnames before DNS.

## Compatibility And Migration

- Add new migrations after current version 61; never alter migration 1/25/26/31/53.
- Existing unsafe OAuth bindings are not automatically merged or reassigned. A diagnostic query/document identifies conflicts for manual review.
- Existing contact ciphertext keeps its recorded key version. Keyring config includes the current legacy key under the existing version, so no automatic production re-encryption occurs.
- Existing API paths remain compatible; `/version` is additive. Health/readiness expose only non-sensitive metadata.
- CSP starts enforced only after frontend asset/API/OAuth requirements are represented; if current inline assets prevent enforcement, production uses an explicit report-only transition with a documented deadline and tests.

## Failure And Rollback

- Each slice gets its own migration and focused commits; rollback never rewrites applied migration history.
- OAuth/bootstrap conflicts fail closed with stable domain errors and no partial writes.
- Safe outbound rejection is observable but never logs API keys, Authorization headers, full third-party bodies, or sensitive URLs.
- Cleanup task failure is logged/metriced and does not prevent Web startup.
- Critical security limiters fail closed when a configured shared backend is unavailable; the in-memory implementation remains the single-instance default.
- Release archive generation refuses dirty/uncommitted sources rather than silently packaging the working tree.

## Verification Strategy

- Focused unit tests for normalization, address classification, headers, config and policy.
- PostgreSQL integration tests for identity races, bootstrap collision/rollback, verification concurrency, cleanup leases and key-version reads.
- Handler tests for stable Problem Details, `Retry-After`, client IP spoofing, version response and response headers.
- Static scripts for archive contents, route/OpenAPI parity, migration parity, Compose port exposure and generated types.
- Full backend, frontend, race, build and Docker gates at slice and final integration boundaries.
