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
- `internal/module/{auth,idempotency,contact,officialprice,profile,announcement,favorite,review,report,reputation,notification,search}`：已拥有模型、仓储接口和业务 service。
- `internal/module/{carpool,apimarket,apiintent,apiorder,apiquota}`：已拥有模型、仓储接口和业务 service，分别承载拼车、API 服务发布审核、API purchase intent 生命周期、API order 付款/交付状态机和限时 API 额度包库存。
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
scripts/build-backend-image.sh HEAD 0.1.0 c2cmarket-backend:0.1.0
# 将 .env.production 中的 BACKEND_IMAGE 改为 c2cmarket-backend:0.1.0
docker compose --env-file .env.production -f compose.yaml -f compose.prod.yaml config
docker compose --env-file .env.production -f compose.yaml -f compose.prod.yaml --profile app up --no-build -d backend
curl -fsS http://127.0.0.1:8080/version
```

release build 脚本要求工作区无 staged、unstaged 或未跟踪改动，并只使用
`HEAD` 解析出的 commit archive 作为 Docker context。生产 Compose 要求
`BACKEND_IMAGE`，不会从当前工作区执行 build。

完整部署和回滚流程见 `../docs/ops/deployment-runbook.md`。

PostgreSQL 默认连接串：

```text
postgres://c2c_market:c2c_market_dev_password@localhost:5432/c2c_market?sslmode=disable
```

PostgreSQL migration 通过 Compose 的一次性 `migrate` 服务执行，migration SQL 位于 `migrations/`；当前期望版本为 `82`。服务进程会在配置 `DATABASE_URL` 时创建 PostgreSQL 连接池，并通过 `/readyz` 暴露数据库和 migration readiness。当前 users、auth sessions、account appeal sessions、user password credential hashes、idempotency、product catalog reads、official price leads/records、contact methods、contact sessions、contact access logs、API purchase intent contact access logs、carpool listings、carpool cycle terms、carpool applications、join confirmations、memberships、completion confirmations、API services、API quota batches/offers/rounds/inventory/credentials、API purchase intents、API orders、API order events、API order payment-instruction access logs、API order delivery credentials、profile/merchant profile、announcements、favorites、reviews、reports、dispute cases、dispute reputation outcomes、user restrictions、reputation governance events、reputation snapshots/history、source author verifications/audit events、appeals、dispute events 和 notifications 已接 PostgreSQL，搜索从这些公开可见业务表读取摘要结果。

限时 API 额度包是 API 市场内独立于 Sub2API 自由金额购买的商品类型。额度来自卖家站外控制的中转系统，平台只保存“卖家声明可售美元额度”的站内销售约束，不提供、充值或验证上游余额。批次最晚在绝对失效时间前 1 小时停止新订单；全天包使用 10 分钟付款窗口，定时放量包使用 5 分钟。发布事务先划拨完整计划额度并生成独立库存行，抢购事务通过 PostgreSQL `FOR UPDATE SKIP LOCKED` 领取库存，以 `(round, buyer)` 唯一 claim 保证同一买家同轮跨规格限购 1 份。三类付费 SKU 都要求明确的 5h/每日倍率计费后美元限额或“不限”，新订单冻结成交时规则，历史空值保持“未说明”。限时订单继续冻结固定 USD 额度、CNY 总价、有效售价、模型倍率、停售/失效时间和成交时商户声明；当前公共商品不展示商户自报 TTFT，统一展示连接级平台探针汇总。

API 健康探针使用卖家级可复用连接：一个 Endpoint + 专用 Key 可绑定多个服务，每个五分钟槽只执行一次带 Bearer 鉴权的 `GET {BaseURL}/models`，不指定模型、不记录模型列表或 TTFT。创建和变更连接会立即验证 Key；不再使用 DNS TXT、HTTP challenge 或管理员审批，也不宣称卖家拥有目标。Base URL 不自动补 `/v1`，HTTP 需要显式风险确认，所有请求仍执行公网地址、DNS 重绑定和重定向限制。连接不可用会暂停所绑定服务接收新订单，但不修改历史订单、履约或纠纷状态。

新发布的限时额度包只允许卖家手工交付。历史预导入商品仍可继续导入和管理买家专属凭据，并保持既有订单履约：CSV 只接受 `api_base_url,api_key,instructions` 或 `panel_login_url,username,password,instructions` 两种严格模板，单次最多 5000 行、5 MiB，并整批校验和去重。API Key/初始密码使用 contact crypto 加密，HMAC fingerprint 用于跨文件重复检测；公开接口、摘要、通知、事件、日志和幂等响应不包含原始秘密。历史预导入凭据在订单创建时只做预留，必须等卖家明确确认站外收款后才绑定到参与方订单详情。

信誉治理把纠纷主体、责任裁定和业务限制分开保存。未解决纠纷只对 `subject_user_id` 形成 caution，不会处罚举报人、证人或自动禁止交易；管理员只能对已解决纠纷创建版本化 outcome，并通过角色和动作精确限制 `carpool_publish`、`carpool_apply`、`carpool_accept`、`api_service_publish`、`api_order_create`、`contact_view` 或 `review_submit`。只有已开始、未到期且未撤销的 restriction 会阻止匹配动作；到期、手动撤销或关联申诉批准后立即恢复。所有治理 mutation 都要求 admin session、CSRF、`Idempotency-Key` 和 `If-Match`，并写入不可修改的 `reputation_governance_events`；管理员不能直接改写用户信誉等级。账号是否可登录仍由 auth 的 `active` 状态独立判定，密码、OAuth 和既有 session 使用相同规则。

官网价格由管理员通过 `/api/v1/admin/official-price-records*` 维护；新增、编辑和下架会在事务内写入兼容 lead、price record、domain event、admin audit log、notification 和幂等结果，公开读取只返回 active 记录，普通用户提交 `official-price-leads` 已禁用。联系方式完整值使用 AES-GCM 加密落库，并写入 HMAC fingerprint；拼车 HTTP 响应只在有效联系窗口内向参与方返回完整值。拼车 owner 发布要求当前账号已绑定 linux.do，发布动作会复查产品 `publish_policy`、套餐额度配置和车主联系方式后直接进入 `active`；车源请求只提交每人每月额度数值，`quota_label`、`quota_unit` 和 `quota_period` 由 `product_plans` 注入并随车源返回；admin 可对公开车源 `pause` 下架并 `restore` 恢复，遗留 `pending_review` 车源仍可通过 admin approve/request-changes/reject 处理。API 服务当前使用早期自动通过策略，owner 提交审核时若已绑定 linux.do 且商户联系方式有效，会返回 `review_status=approved`、`publication_status=offline`，仍需 owner 手动 publish 才公开；`pending_review` 等状态和 admin 审核路由保留。公开 API 服务列表、详情、搜索、收藏校验和购买意向创建只面向当前可接单服务，公开 DTO 只暴露付款方式标签，不暴露收款说明或收款码。API 购买意向只在成功创建响应、买家详情和对应 owner 详情中返回冻结后的完整联系方式，并在每次直接披露时写入不含明文的访问日志；同一购买意向最多生成一笔 API order，重复或并发创建返回 `API_PURCHASE_INTENT_HAS_ORDER`，已有 order 后不能再按普通购买意向取消或关闭。API order 创建会冻结所选微信/支付宝付款方式的收款说明和收款码快照；买家通过显式付款资料读取接口查看，响应使用 `Cache-Control: private, no-store` 并写入不含明文的访问日志。商户确认站外收款后，可以提交一次结构化站内交付凭证，支持买家专属的 API Key + Base URL 或初始登录账号，提交后不可修改；API Key 和初始密码使用既有 contact crypto 加密存入 `api_order_delivery_credentials`，`deliveryNote` 只保存非敏感摘要，列表、公开页面、通知、事件、日志、举报和幂等缓存不包含明文凭据。收藏仅记录当前用户对公开车源或公开 API 服务的个人标记。评价只允许已完成拼车 membership 和已完成 API order 的买卖双方在 14 天内互评：一方提交后内容保持 sealed，双方都提交或窗口截止时才公开并冻结；公开后普通用户不能修改，管理员只能通过版本化、幂等且留 revision 的移除动作隐藏公开内容，不能改写评分、标签或说明。评价不改变交易状态，也不构成支付、托管、担保、纠纷责任或凭据交付证明。举报/纠纷/申诉仅记录脱敏说明、人工处理状态、公开摘要和事件，不处理支付、退款、赔付、托管、担保、履约或 API 凭证交付。通知中心仅读取和更新当前用户站内业务通知，不发送短信、Webhook、真实推送或外部工单；邮箱验证码、注册成功、拼车上车申请/接受提醒和 API 购买意向提醒由 profile/auth 模块通过 development sender 或阿里云 DirectMail SMTP 发送。搜索只返回公开可见资源摘要，不返回联系方式、隐藏 store alias owner、admin 内部字段或凭据材料。

第一版本公开注册/登录入口只支持 linux.do OAuth。OAuth 登录入口通过 `GET /api/v1/auth/oauth/start` 和 `GET /api/v1/auth/oauth/callback` 创建真实 session；`(provider, provider_subject)` 是不可变身份键，首次登录在事务中创建独立 `users`、`auth_identities` 和 `linux_do_bindings`，username 冲突只生成替代 handle，不复用已有用户。OAuth 登录不会授予 admin permission。站内备用密码入口 `POST /api/v1/auth/password` 和 `POST /api/v1/auth/password/login` 仅允许已绑定 linux.do 的用户设置和登录，使用 `user_password_credentials` 中的 salted hash 创建真实 session，不保存明文密码。邮箱验证码注册兼容端点 `POST /api/v1/auth/email-registration/start` 和 `POST /api/v1/auth/email-registration/confirm` 固定返回 `EMAIL_REGISTRATION_DISABLED`，不会发送注册验证码、创建账号或设置 session；已登录用户的邮箱验证仍作为资料/联系信息功能保留。本地开发默认 `OAUTH_PROVIDER_MODE=fake`，用于自动化 smoke；生产环境必须使用 `OAUTH_PROVIDER_MODE=oauth2` 并配置 `OAUTH_CLIENT_ID`、`OAUTH_CLIENT_SECRET`、`OAUTH_AUTHORIZE_URL`、`OAUTH_TOKEN_URL`、`OAUTH_USERINFO_URL`、`OAUTH_REDIRECT_URL`。OAuth token exchange 和 userinfo 请求使用 10 秒 timeout 的专用 HTTP client，响应体读取限制为 1 MiB。后端不保存 OAuth provider access token 或 refresh token，只保存用户身份绑定摘要。

管理员 Bootstrap 只适用于没有任何管理员的空库首次初始化。它创建全新管理员并写入固定来源标记 `initial-admin-v1`；重复执行只验证来源、用户、权限和密码凭据是否完整，不修改密码。已有管理员或目标 username 被占用时会 fail closed，不会复用、提升或修改现有用户。首次初始化成功后必须同时清空 `C2C_BOOTSTRAP_ADMIN_USERNAME` 和 `C2C_BOOTSTRAP_ADMIN_PASSWORD`。已有管理员数据库升级到 migration 62 时没有该来源标记，启动新版本前也必须清空这两个变量；不得自动补写标记或认领旧管理员。

登录 session 初始空闲有效期为 7 天。通过认证的普通业务访问距离 `renewed_at` 满 24 小时后，PostgreSQL 使用带过期、撤销和绝对期限条件的原子更新续期至 `min(当前时间+7天, absolute_expires_at)`，并仅在实际更新成功时同步发送 Cookie。静态资源、健康检查、`OPTIONS`、认证入口、logout、SSE 和后台计数轮询不触发续期；连续登录最长 30 天，之后必须重新登录。

开发认证入口默认只在 `APP_ENV=development` 或 `APP_ENV=test` 时开启。生产环境必须配置 `DATABASE_URL`、绝对 HTTPS `FRONTEND_ORIGIN`、`CONTACT_ENCRYPTION_KEY`、`CONTACT_FINGERPRINT_KEY`、`CONTACT_KEY_VERSION`、至少 32 字节的 `EMAIL_VERIFICATION_PEPPER`、OAuth provider 配置和阿里云 DirectMail SMTP 配置，且不能启用 `ENABLE_DEV_AUTH=true`。生产邮箱验证码、注册成功和业务提醒邮件使用 `EMAIL_PROVIDER=aliyun_directmail`，需要 `SMTP_HOST`、`SMTP_PORT=465`、`SMTP_USERNAME`、`SMTP_PASSWORD`、`MAIL_FROM_ADDRESS`、`MAIL_FROM_NAME`，生产发信地址必须由部署环境显式配置。生产 session/OAuth cookie 使用 `HttpOnly=true`、`Secure=true`、`SameSite=Lax`；logout 和 OAuth state 清理 cookie 使用相同 Path/Secure/SameSite 组合。OAuth callback 会把清理后的相对 `returnTo` 拼接到 `FRONTEND_ORIGIN`，用于前后端分域部署后的安全回跳。

## HTTP 边界 Hardening

- CORS/Origin：`FRONTEND_ORIGIN` 是生产必填的主前端 origin，并自动加入 allowlist；`ALLOWED_ORIGINS` 可用英文逗号追加其他明确 origin。cookie 认证响应不会使用 wildcard origin；生产状态变更请求会拒绝不在 allowlist 内的浏览器 `Origin`。
- 模型审计安全出站：target 只接受公网 HTTPS Base URL，保存和实际连接都会解析并拒绝私网、loopback、link-local、metadata、特殊用途及混合 DNS 结果；连接只拨已验证 IP，禁止重定向，并限制连接、TLS、响应头、总请求时间及响应体大小。`MODEL_AUDIT_ALLOWED_HOSTS` 使用英文逗号配置精确 host，不支持 wildcard；空值表示允许任意通过公网地址检查的 HTTPS host。
- 安全响应头：后端统一设置 deny-by-default API CSP、`Permissions-Policy`、`X-Frame-Options: DENY`、`X-Content-Type-Options: nosniff` 和 `Referrer-Policy: strict-origin-when-cross-origin`；`APP_ENV=production` 时设置 `Strict-Transport-Security: max-age=31536000; includeSubDomains`。前端静态站点通过 `frontend/public/_headers` 使用独立的页面 CSP。
- Metrics：`GET /metrics` 输出 Prometheus/OpenMetrics 指标。生产环境强制使用至少 32 字节的独立 `METRICS_BEARER_TOKEN`，请求需携带 `Authorization: Bearer <token>`；无效凭据返回 `401 METRICS_AUTH_REQUIRED`。该路由应仅通过受限的运维入口访问。
- 限流：当前为有最大 key 数的进程内 1 分钟窗口，按 route group、IP 和登录 userID 分开计数并定期清理。OAuth、search、API purchase intent 创建、额度包直达订单、联系方式读取、举报/申诉创建和 dev contact/session 入口超限返回 `429`，Problem Details `code=RATE_LIMITED` 和整数 `Retry-After`。额度包购买使用每用户 12 次、每 IP 3000 次的独立预算，避免共享出口误伤；限流只用于减压，库存正确性仍完全由 PostgreSQL 保证。
- 分页：主要列表接口支持 `limit` / `cursor`，默认 `20`、最大 `100`，响应为 `{ "items": [], "nextCursor": "..." }`。当前 cursor 是 opaque base64url offset cursor，调用方只应透传。
- 幂等：`processing`、`failed`、`completed` 分别保留 15 分钟、1 小时、7 天；completed 同请求 replay 返回缓存或资源重建结果，超过 64 KiB 且无法重建的响应返回 `IDEMPOTENCY_RESULT_NOT_REPLAYABLE`，不会再次执行 mutation；同 key 不同 request hash 在记录到期前返回 `IDEMPOTENCY_KEY_REUSED`，到期后可由新请求接管。
- 数据维护：配置 PostgreSQL 时，应用立即执行并按 `MAINTENANCE_INTERVAL` 周期运行有限批次。多实例通过 advisory transaction lock 互斥；Session、验证码、幂等、通知和无引用领域事件按配置保留。API 交付凭据在订单完成或商品有效期结束等较晚锚点后继续保留 `API_DELIVERY_CREDENTIAL_RETENTION`，开放纠纷或待审申诉期间暂停销毁；到期后订单凭据和对应的预导入副本会不可逆清除全部凭据载荷，只保留非敏感审计事实。结束的联系窗口只改为 `expired`，不会删除联系方式密文、联系访问记录、管理员或纠纷审计。

## 验证

```bash
go test ./...
```

额度包并发验证（2026-07-19，Codex）使用名称以 `_quota_test` 结尾的专用 PostgreSQL 数据库：

```bash
C2C_TEST_DATABASE_URL='postgres://c2c_market:c2c_market_dev_password@127.0.0.1:5432/c2c_market_quota_test?sslmode=disable' \
  go test ./internal/store/postgres -run '^TestAPIQuotaPostgresRush1500BuyersFor1000Copies$' -count=1 -v
```

该测试创建 1500 个独立买家竞争 1000 份库存，并断言成功订单、轮次 claim 和库存预留均恰好为 1000。真实 HTTP 路由可用 `../scripts/api-quota-rush-smoke.mjs` 复测；`--help` 列出参数，session JSONL 只在本地或预发布环境准备，不应提交仓库。

当前 route 组：

- Health/readiness：`GET /health`、`GET /readyz`
- Auth/session/OAuth：`/api/v1/auth/password`、`/api/v1/auth/password/login`、`/api/v1/auth/email-registration/start`、`/api/v1/auth/email-registration/confirm`、`/api/v1/auth/oauth/start`、`/api/v1/auth/oauth/callback`、`/api/v1/auth/session`、`/api/v1/auth/logout`；邮箱注册端点固定禁用，备用密码仅限已绑定 linux.do 用户；开发专用 `/api/v1/auth/dev-session`
- Search：`GET /api/v1/search`
- Profile/contact/merchant profile：`/api/v1/me/profile`、`/api/v1/me/contact-methods`、`/api/v1/contact-methods/*`、`/api/v1/me/merchant-profile`、`/api/v1/users/{username}/public-profile`、`/api/v1/merchant-profiles/{slug}`
- Announcements：用户端 `/api/v1/announcements*`、receipt `/api/v1/me/announcements/*`、管理端 `/api/v1/admin/announcements*`
- Catalog/official price：`/api/v1/product-*`、`/api/v1/api-models*`、`/api/v1/official-prices*`、`/api/v1/official-price-leads*`（提交已禁用，保留只读兼容）、`/api/v1/admin/official-price-records*`、`/api/v1/admin/official-price-leads*`（遗留审核兼容）
- Favorites：用户 `/api/v1/me/favorites*`
- Reviews：用户 `GET /api/v1/me/reviews`、`POST|PUT /api/v1/me/transactions/{type}/{id}/review`、兼容 `PUT /api/v1/me/reviews/carpool-memberships/{membershipId}`，公开 `GET /api/v1/users/{username}/reviews`，管理员 `POST /api/v1/admin/reviews/{id}/remove`
- Reports/disputes/appeals/reputation governance：用户 `/api/v1/reports`、`/api/v1/me/reports`、`/api/v1/me/appeals`，公开 `/api/v1/users/{username}/disputes`，admin `/api/v1/admin/reports*`、`/api/v1/admin/disputes*`、`/api/v1/admin/appeals*`、`POST /api/v1/admin/disputes/{id}/reputation-outcome`、`POST /api/v1/admin/users/{id}/reputation-restrictions`、`POST /api/v1/admin/reputation-restrictions/{id}/revoke`
- Notifications：用户 `/api/v1/me/notifications*`
- Carpool：公开 `/api/v1/carpools*`、买家 `/api/v1/me/carpool-*`、owner `/api/v1/owner/carpool-*`、admin `/api/v1/admin/carpools*`
- API market/order：公开 `/api/v1/api-services*`、`/api/v1/api-quota-offers*`，owner `/api/v1/owner/api-services*`、`/api/v1/owner/api-quota-*`，buyer `/api/v1/me/api-purchase-intents*`、`/api/v1/me/api-orders*`，owner `/api/v1/owner/api-purchase-intents*`、`/api/v1/owner/api-orders*`，admin `/api/v1/admin/api-*`
- Dev contact sessions：`/api/v1/dev/contact-sessions`、`/api/v1/contact-sessions/{id}/contacts`

契约文件：

- OpenAPI: `../docs/openapi/c2c-market-api-v1.yaml`
- Generated OpenAPI types: `../frontend/src/api/generated/openapi/`
- PostgreSQL migrations: `migrations/*.up.sql` / `migrations/*.down.sql`

当前可运行切片的用户、OAuth 身份绑定、会话、linux.do 绑定摘要、幂等、产品目录、官网价格记录维护、公开价格读取、联系窗口、拼车车源、账期/退出/使用规则、上车申请、确认加入、成员关系、完成确认、买家退出、车主移除、API 服务发布审核、API 购买意向、API order 付款/交付、个人资料、联系方式、公开主页、商户资料、公告、收藏、评价、举报、纠纷、申诉、通知中心和全局搜索均已接 PostgreSQL。管理员新增、编辑和下架官网价格记录会在同一个 PostgreSQL transaction 中写入兼容 lead、price record、domain event、admin audit log、notification 以及 completed idempotency response cache；公开价格列表和详情只返回 active 记录。拼车车主接受申请会在同一个 PostgreSQL transaction 中锁申请/车源、创建 30 分钟联系窗口、冻结双方联系方式版本、写 domain event、通知和 completed idempotency response cache；应用层会在成功接受后向已验证邮箱的买家发送 best-effort 邮件提醒。API 购买意向创建会在同一个 transaction 中锁 public API service、冻结买家和商户联系方式版本、写 intent/event/notification、写 buyer 查看 merchant 联系方式访问日志，并完成只含资源标识的幂等记录；成功响应直接返回冻结商户联系方式且设置 `Cache-Control: no-store`，应用层会在成功创建后向已验证邮箱的商户发送 best-effort 邮件提醒。买家详情读取 merchant 联系方式、owner 详情读取 buyer 联系方式也会写入 `api_purchase_intent_contact_access_logs`，字段仅包括 intent、viewer、被查看侧、request id 和访问时间，不记录联系方式明文。API order 创建、买家提交付款、商户确认收款、商户一次性交付凭证、买家确认完成、纠纷登记和付款超时都围绕 `api_orders` 状态机执行；交付凭证明文只在参与方详情/action 响应返回，并使用 `Cache-Control: private, no-store`。收藏 `PUT` 复用 session、CSRF 和 Idempotency-Key，只允许公开可见车源或 API 服务作为目标；收藏和取消收藏不改变目标资源状态。评价通过统一交易路由复用 session、CSRF 和 Idempotency-Key，只允许已完成拼车 membership 或 API order 的买卖双方在 14 天内各评价一次；`POST` 创建 sealed 评价，公开前 `PUT` 可修改，双方提交或截止时公开并冻结。公开评价可由管理员使用 `If-Match` 执行留痕移除，但冻结内容不能改写。举报创建、纠纷处理和申诉处理复用 session、CSRF、Idempotency-Key、If-Match 和 ETag；公开纠纷摘要只返回脱敏 summary/result，不暴露 reporter、admin、联系方式、内部备注或原始证据。通知中心只提供当前用户业务通知 list、unread count、read one 和 read all；公告 receipt 是 per-user 状态，不改变公告源内容，也不和业务通知 inbox 混用。公告 PostgreSQL admin update 已修复，`announcement-smoke.mjs` 在 PostgreSQL 路径覆盖创建、编辑、发布、下线、复制、审计和 receipt 版本失效。搜索只读公开可见摘要，不新增数据库表；migration `000024` 启用 `pg_trgm` 并为高频公开搜索字段加 GIN trigram index。migration 是数据库契约基线，后续任务继续补齐部署运维。
