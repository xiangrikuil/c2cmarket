# C2C Market Backend

Go 后端服务入口位于 `cmd/api`。当前结构已迁移到模块化一期：

- `cmd/api`：进程入口、配置加载、监听端口。
- `internal/app`：配置、PostgreSQL store、核心服务和 HTTP server 的依赖组装边界。
- `internal/server`：chi router、路由注册、HTTP handler、server 层 DTO。
- `internal/middleware`：request ID、session cookie、CSRF header、idempotency key 等可复用 HTTP 边界助手。
- `internal/response`：JSON 和 Problem Details 响应格式。
- `internal/validator`：严格 JSON、`If-Match`、请求 hash 和时间解析等共享请求校验。
- `internal/database`：pgxpool 打开和 PostgreSQL readiness。
- `internal/module/core`：兼容 facade，保留旧 service 方法名并委托到模块服务。
- `internal/module/catalog`：产品分类、产品套餐和 API model catalog 的模型、仓储接口、seed 数据和只读服务。
- `internal/module/{auth,idempotency,contact,officialprice,profile,announcement,demand,favorite,review,report,notification,search}`：已拥有模型、仓储接口和业务 service。
- `internal/module/{carpool,apimarket,apiintent,apiorder}`：已拥有模型、仓储接口和业务 service，分别承载拼车、API 服务发布审核、API purchase intent 生命周期和 API order 付款/交付状态机。
- `internal/store/postgres`：PostgreSQL Store，已按业务域拆分 SQL 文件，共享同一个 pool 和 contact crypto 基础设施。

## 本地运行

```bash
go run ./cmd/api
```

默认监听 `:8080`，可通过 `PORT` 环境变量覆盖。

进程入口使用显式 `http.Server`，当前默认 timeout 为：

```text
ReadHeaderTimeout = 5s
ReadTimeout       = 15s
WriteTimeout      = 30s
IdleTimeout       = 60s
```

## Docker 运行

项目根目录提供 `compose.yaml`：

```bash
docker compose up -d postgres
docker compose --profile migrate run --rm migrate
docker compose --profile app build backend
docker compose --profile app up -d backend
```

生产模拟使用根目录 `compose.prod.yaml` 覆盖开发默认值：

```bash
cp .env.production.example .env.production
docker compose --env-file .env.production -f compose.yaml -f compose.prod.yaml config
docker compose --env-file .env.production -f compose.yaml -f compose.prod.yaml --profile app build backend
docker compose --env-file .env.production -f compose.yaml -f compose.prod.yaml --profile app up -d backend
```

完整部署和回滚流程见 `../docs/ops/deployment-runbook.md`。

PostgreSQL 默认连接串：

```text
postgres://c2c_market:c2c_market_dev_password@localhost:5432/c2c_market?sslmode=disable
```

PostgreSQL migration 通过 Compose 的一次性 `migrate` 服务执行，migration SQL 位于 `migrations/`。服务进程会在配置 `DATABASE_URL` 时创建 PostgreSQL 连接池，并通过 `/readyz` 暴露数据库和 migration readiness。当前 users、auth sessions、user password credential hashes、idempotency、product catalog reads、official price leads/records、contact methods、contact sessions、contact access logs、API purchase intent contact access logs、carpool listings、carpool cycle terms、carpool applications、join confirmations、memberships、completion confirmations、API services、API purchase intents、API orders、API order events、API order payment-instruction access logs、API order delivery credentials、profile/merchant profile、announcements、demands、favorites、reviews、reports、dispute cases、appeals、dispute events 和 notifications 已接 PostgreSQL，搜索从这些公开可见业务表读取摘要结果。官网价格由管理员通过 `/api/v1/admin/official-price-records*` 维护；新增、编辑和下架会在事务内写入兼容 lead、price record、domain event、admin audit log、notification 和幂等结果，公开读取只返回 active 记录，普通用户提交 `official-price-leads` 已禁用。联系方式完整值使用 AES-GCM 加密落库，并写入 HMAC fingerprint；拼车 HTTP 响应只在有效联系窗口内向参与方返回完整值。拼车 owner 发布要求当前账号已绑定 linux.do，发布动作会复查产品 `publish_policy`、套餐额度配置和车主联系方式后直接进入 `active`；车源请求只提交每人每月额度数值，`quota_label`、`quota_unit` 和 `quota_period` 由 `product_plans` 注入并随车源返回；admin 可对公开车源 `pause` 下架并 `restore` 恢复，遗留 `pending_review` 车源仍可通过 admin approve/request-changes/reject 处理。API 服务当前使用早期自动通过策略，owner 提交审核时若已绑定 linux.do 且商户联系方式有效，会返回 `review_status=approved`、`publication_status=offline`，仍需 owner 手动 publish 才公开；`pending_review` 等状态和 admin 审核路由保留。公开 API 服务列表、详情、搜索、收藏校验和购买意向创建只面向当前可接单服务，公开 DTO 只暴露付款方式标签，不暴露收款说明或收款码。API 购买意向只在成功创建响应、买家详情和对应 owner 详情中返回冻结后的完整联系方式，并在每次直接披露时写入不含明文的访问日志；同一购买意向最多生成一笔 API order，重复或并发创建返回 `API_PURCHASE_INTENT_HAS_ORDER`，已有 order 后不能再按普通购买意向取消或关闭。API order 创建会冻结所选微信/支付宝付款方式的收款说明和收款码快照；买家通过显式付款资料读取接口查看，响应使用 `Cache-Control: private, no-store` 并写入不含明文的访问日志。商户确认站外收款后，可以提交一次结构化站内交付凭证，支持买家专属的 API Key + Base URL 或初始登录账号，提交后不可修改；API Key 和初始密码使用既有 contact crypto 加密存入 `api_order_delivery_credentials`，`deliveryNote` 只保存非敏感摘要，列表、公开页面、通知、事件、日志、举报和幂等缓存不包含明文凭据。需求池仅记录求车信息、预算、地区、偏好、来源链接和公开/关闭/下架状态，创建和重开直接进入公开匹配中。收藏仅记录当前用户对公开车源或公开 API 服务的个人标记。评价仅记录已完成拼车 membership 的买家对车主体验反馈，不改变 membership 状态或任何支付、托管、担保、凭据交付状态。举报/纠纷/申诉仅记录脱敏说明、人工处理状态、公开摘要和事件，不处理支付、退款、赔付、托管、担保、履约或 API 凭证交付。通知中心仅读取和更新当前用户站内业务通知，不发送短信、Webhook、真实推送或外部工单；邮箱验证码、注册成功、拼车上车申请/接受提醒和 API 购买意向提醒由 profile/auth 模块通过 development sender 或阿里云 DirectMail SMTP 发送。搜索只返回公开可见资源摘要，不返回联系方式、隐藏 store alias owner、admin 内部字段或凭据材料。

第一版本公开注册/登录入口只支持 linux.do OAuth。OAuth 登录入口通过 `GET /api/v1/auth/oauth/start` 和 `GET /api/v1/auth/oauth/callback` 创建真实 session，并 upsert `users`、`auth_identities`、`linux_do_bindings` 和可选 admin permission。站内备用密码入口 `POST /api/v1/auth/password` 和 `POST /api/v1/auth/password/login` 仅允许已绑定 linux.do 的用户设置和登录，使用 `user_password_credentials` 中的 salted hash 创建真实 session，不保存明文密码。邮箱验证码注册兼容端点 `POST /api/v1/auth/email-registration/start` 和 `POST /api/v1/auth/email-registration/confirm` 固定返回 `EMAIL_REGISTRATION_DISABLED`，不会发送注册验证码、创建账号或设置 session；已登录用户的邮箱验证仍作为资料/联系信息功能保留。本地开发默认 `OAUTH_PROVIDER_MODE=fake`，用于自动化 smoke；生产环境必须使用 `OAUTH_PROVIDER_MODE=oauth2` 并配置 `OAUTH_CLIENT_ID`、`OAUTH_CLIENT_SECRET`、`OAUTH_AUTHORIZE_URL`、`OAUTH_TOKEN_URL`、`OAUTH_USERINFO_URL`、`OAUTH_REDIRECT_URL`。OAuth token exchange 和 userinfo 请求使用 10 秒 timeout 的专用 HTTP client，响应体读取限制为 1 MiB。后端不保存 OAuth provider access token 或 refresh token，只保存用户身份绑定摘要。

开发认证入口默认只在 `APP_ENV=development` 或 `APP_ENV=test` 时开启。生产环境必须配置 `DATABASE_URL`、绝对 HTTPS `FRONTEND_ORIGIN`、`CONTACT_ENCRYPTION_KEY`、`CONTACT_FINGERPRINT_KEY`、`CONTACT_KEY_VERSION`、OAuth provider 配置和阿里云 DirectMail SMTP 配置，且不能启用 `ENABLE_DEV_AUTH=true`。生产邮箱验证码、注册成功和业务提醒邮件使用 `EMAIL_PROVIDER=aliyun_directmail`，需要 `SMTP_HOST`、`SMTP_PORT=465`、`SMTP_USERNAME`、`SMTP_PASSWORD`、`MAIL_FROM_ADDRESS`、`MAIL_FROM_NAME`，生产发信地址必须由部署环境显式配置。生产 session/OAuth cookie 使用 `HttpOnly=true`、`Secure=true`、`SameSite=Lax`；logout 和 OAuth state 清理 cookie 使用相同 Path/Secure/SameSite 组合。OAuth callback 会把清理后的相对 `returnTo` 拼接到 `FRONTEND_ORIGIN`，用于前后端分域部署后的安全回跳。

## HTTP 边界 Hardening

- CORS/Origin：`FRONTEND_ORIGIN` 是生产必填的主前端 origin，并自动加入 allowlist；`ALLOWED_ORIGINS` 可用英文逗号追加其他明确 origin。cookie 认证响应不会使用 wildcard origin；生产状态变更请求会拒绝不在 allowlist 内的浏览器 `Origin`。
- 安全响应头：后端统一设置 `X-Content-Type-Options: nosniff` 和 `Referrer-Policy: strict-origin-when-cross-origin`；`APP_ENV=production` 时设置 `Strict-Transport-Security: max-age=31536000; includeSubDomains`。CSP 由前端静态站点或反向代理按页面资产策略配置。
- 限流：当前为进程内 1 分钟窗口，按 route group、IP 和登录 userID 组合计数。OAuth、search、API purchase intent 创建、联系方式读取、举报/申诉创建和 dev contact/session 入口超限返回 `429`，Problem Details `code=RATE_LIMITED`，并尽量返回 `Retry-After`。
- 分页：主要列表接口支持 `limit` / `cursor`，默认 `20`、最大 `100`，响应为 `{ "items": [], "nextCursor": "..." }`。当前 cursor 是 opaque base64url offset cursor，调用方只应透传。
- 幂等：completed 同请求 replay 保持返回缓存或资源重建结果；同 key 不同 request hash 返回 `IDEMPOTENCY_KEY_REUSED`；未过期 `processing` 返回 `IDEMPOTENCY_IN_PROGRESS`；同请求 hash 且已过期的 `processing` 可被接管重试。应用启动时会清理保守过期的 processing 记录。

## 验证

```bash
go test ./...
```

当前 route 组：

- Health/readiness：`GET /health`、`GET /readyz`
- Auth/session/OAuth：`/api/v1/auth/password`、`/api/v1/auth/password/login`、`/api/v1/auth/email-registration/start`、`/api/v1/auth/email-registration/confirm`、`/api/v1/auth/oauth/start`、`/api/v1/auth/oauth/callback`、`/api/v1/auth/session`、`/api/v1/auth/logout`；邮箱注册端点固定禁用，备用密码仅限已绑定 linux.do 用户；开发专用 `/api/v1/auth/dev-session`
- Search：`GET /api/v1/search`
- Profile/contact/merchant profile：`/api/v1/me/profile`、`/api/v1/me/contact-methods`、`/api/v1/contact-methods/*`、`/api/v1/me/merchant-profile`、`/api/v1/users/{username}/public-profile`、`/api/v1/merchant-profiles/{slug}`
- Announcements：用户端 `/api/v1/announcements*`、receipt `/api/v1/me/announcements/*`、管理端 `/api/v1/admin/announcements*`
- Catalog/official price：`/api/v1/product-*`、`/api/v1/api-models*`、`/api/v1/official-prices*`、`/api/v1/official-price-leads*`（提交已禁用，保留只读兼容）、`/api/v1/admin/official-price-records*`、`/api/v1/admin/official-price-leads*`（遗留审核兼容）
- Demands：公开 `/api/v1/demands*`、用户 `/api/v1/me/demands*`、admin `/api/v1/admin/demands*`
- Favorites：用户 `/api/v1/me/favorites*`
- Reviews：用户 `/api/v1/me/reviews*`、公开 `/api/v1/users/{username}/reviews`
- Reports/disputes/appeals：用户 `/api/v1/reports`、`/api/v1/me/reports`、`/api/v1/me/appeals`，公开 `/api/v1/users/{username}/disputes`，admin `/api/v1/admin/reports*`、`/api/v1/admin/disputes*`、`/api/v1/admin/appeals*`
- Notifications：用户 `/api/v1/me/notifications*`
- Carpool：公开 `/api/v1/carpools*`、买家 `/api/v1/me/carpool-*`、owner `/api/v1/owner/carpool-*`、admin `/api/v1/admin/carpools*`
- API market/order：公开 `/api/v1/api-services*`、owner `/api/v1/owner/api-services*`、buyer `/api/v1/me/api-purchase-intents*`、buyer `/api/v1/me/api-orders*`、owner `/api/v1/owner/api-purchase-intents*`、owner `/api/v1/owner/api-orders*`、admin `/api/v1/admin/api-*`
- Dev contact sessions：`/api/v1/dev/contact-sessions`、`/api/v1/contact-sessions/{id}/contacts`

契约文件：

- OpenAPI: `../docs/openapi/c2c-market-api-v1.yaml`
- PostgreSQL migrations: `migrations/*.up.sql` / `migrations/*.down.sql`

当前可运行切片的用户、OAuth 身份绑定、会话、linux.do 绑定摘要、幂等、产品目录、官网价格记录维护、公开价格读取、联系窗口、拼车车源、账期/退出/使用规则、上车申请、确认加入、成员关系、完成确认、买家退出、车主移除、API 服务发布审核、API 购买意向、API order 付款/交付、个人资料、联系方式、公开主页、商户资料、公告、需求池、收藏、评价、举报、纠纷、申诉、通知中心和全局搜索均已接 PostgreSQL。管理员新增、编辑和下架官网价格记录会在同一个 PostgreSQL transaction 中写入兼容 lead、price record、domain event、admin audit log、notification 以及 completed idempotency response cache；公开价格列表和详情只返回 active 记录。拼车车主接受申请会在同一个 PostgreSQL transaction 中锁申请/车源、创建 30 分钟联系窗口、冻结双方联系方式版本、写 domain event、通知和 completed idempotency response cache；应用层会在成功接受后向已验证邮箱的买家发送 best-effort 邮件提醒。API 购买意向创建会在同一个 transaction 中锁 public API service、冻结买家和商户联系方式版本、写 intent/event/notification、写 buyer 查看 merchant 联系方式访问日志，并完成只含资源标识的幂等记录；成功响应直接返回冻结商户联系方式且设置 `Cache-Control: no-store`，应用层会在成功创建后向已验证邮箱的商户发送 best-effort 邮件提醒。买家详情读取 merchant 联系方式、owner 详情读取 buyer 联系方式也会写入 `api_purchase_intent_contact_access_logs`，字段仅包括 intent、viewer、被查看侧、request id 和访问时间，不记录联系方式明文。API order 创建、买家提交付款、商户确认收款、商户一次性交付凭证、买家确认完成、纠纷登记和付款超时都围绕 `api_orders` 状态机执行；交付凭证明文只在参与方详情/action 响应返回，并使用 `Cache-Control: private, no-store`。需求池创建直接公开，关闭、重开和 admin 下架/恢复动作复用 session、CSRF、Idempotency-Key、If-Match 和 ETag，不包含支付、托管、担保或凭据交付流程。收藏 `PUT` 复用 session、CSRF 和 Idempotency-Key，只允许公开可见车源或 API 服务作为目标；收藏和取消收藏不改变目标资源状态。评价 `PUT` 复用 session、CSRF 和 Idempotency-Key，只允许已完成拼车 membership 的买家评价车主，并以同一 `(source_type, source_id, reviewer_user_id)` 记录更新原评价。举报创建、纠纷处理和申诉处理复用 session、CSRF、Idempotency-Key、If-Match 和 ETag；公开纠纷摘要只返回脱敏 summary/result，不暴露 reporter、admin、联系方式、内部备注或原始证据。通知中心只提供当前用户业务通知 list、unread count、read one 和 read all；公告 receipt 是 per-user 状态，不改变公告源内容，也不和业务通知 inbox 混用。公告 PostgreSQL admin update 已修复，`announcement-smoke.mjs` 在 PostgreSQL 路径覆盖创建、编辑、发布、下线、复制、审计和 receipt 版本失效。搜索只读公开可见摘要，不新增数据库表；migration `000024` 启用 `pg_trgm` 并为高频公开搜索字段加 GIN trigram index。migration 是数据库契约基线，后续任务继续补齐部署运维。
