# C2CMarket 项目结构、数据库与 API 总览

日期：2026-06-23
执行者：Codex
用途：给开发者或 GPT Pro 审阅当前项目完成度、结构、数据库表、API 契约和生产化缺口。

## 1. 结论摘要

当前项目已经从前端 Mock 原型推进到“Go 后端 + PostgreSQL + Vue 前端真实模式”的本地真实闭环阶段。核心业务已经覆盖官方价格、拼车、API 服务/购买意向、个人资料/联系方式、公告、需求池、收藏、评价、举报/纠纷/申诉、通知和搜索。

后端结构已经接近推荐的 Go 项目形态：`backend/cmd/api` 是入口，`backend/internal/app` 负责装配，`backend/internal/server` 负责 HTTP 路由和 handler，`backend/internal/module/<domain>` 按业务域放 model/service/repository contract，`backend/internal/store/postgres` 放 PostgreSQL 实现。

前端还没有完全删除 Mock。当前是双模式：配置 `VITE_API_MODE=real` 或 `VITE_API_BASE_URL` 后，大部分主业务通过 backend adapter 调 Go API；未配置真实模式时，`frontend/src/lib/api.ts` 仍使用 `frontend/src/data/mock.ts`、`frontend/src/data/announcements.mock.ts` 和 `sessionStorage` 作为本地 demo/fallback。

生产化方面，仓库已经有 production env 模板、Compose 覆盖、migration 服务、Dockerfile、runbook、smoke 脚本和后端 hardening：HTTP server timeout、生产 Secure cookie、OAuth timeout/响应大小限制、CORS/Origin allowlist、安全响应头、基础限流、列表分页、幂等 processing 过期恢复、API purchase intent 联系方式访问审计、搜索 trigram indexes。真实生产仍需要外部 OAuth provider、正式域名/TLS、静态前端托管、数据库备份恢复、日志/监控/告警、密钥轮换、反向代理和受控生产验证流程。产品边界明确不包含支付、托管、担保、凭据交付、第三方 API key/账号凭据保管或上游 API 代理；站内账号密码仅保存不可逆哈希。

### 2026-07-06 维护更新

- 当前最新 PostgreSQL migration 是 `000036_search_trigram_alignment`，后端 `ExpectedMigrationVersion=36`，`/readyz` 会在数据库 schema 低于该版本时降级。
- 密码写入已升级到 `argon2id_v1`，旧 `sha256_salted_v1` 只保留登录校验和成功后 rehash；首个 admin 通过显式 bootstrap 环境变量创建，不再由 migration 固定密码种子创建。
- 前端生产构建必须显式配置真实后端：`VITE_API_MODE=real` 或 `VITE_API_BASE_URL`；`VITE_ENABLE_MOCK=true` 在 production build 中会失败。Mock/demo 仍保留为本地开发路径，真实模式不得静默 fallback 到 mock 成功数据。
- 后端 service 迁移策略已经明确：`internal/module/<domain>` 拥有业务 service/repository contract，`internal/module/core` 只作为兼容 facade 委托，不再作为新增业务能力的膨胀入口。
- CI 新增 migration 文档漂移检查；源码发布包通过 `scripts/package-source.sh` 生成并自检排除 `.git/`、`output/`、`tmp/`、`node_modules/`、`dist/`、`build/`、`coverage/` 等临时/构建产物。

## 2. 整体项目目录结构

```text
c2c-market/
├── README.md                         # 项目总 README、当前完成范围、运行与验证方式
├── compose.yaml                      # 本地 PostgreSQL、migration、backend Compose
├── compose.prod.yaml                 # 生产 Compose 覆盖：禁用 dev auth，强制 oauth2 和密钥
├── .env.example                      # 本地开发环境变量模板
├── .env.production.example           # 生产环境变量模板
├── backend/                          # Go HTTP 后端
├── frontend/                         # Vue 3 + Vite + TypeScript 前端
├── docs/
│   ├── openapi/c2c-market-api-v1.yaml
│   ├── ops/deployment-runbook.md
│   └── project-architecture-api-db-overview-2026-06-23.md
└── scripts/                          # 本地真实后端 smoke 脚本
```

后端目录：

```text
backend/
├── cmd/api/main.go                   # 当前 Go 服务入口
├── Dockerfile
├── README.md
├── go.mod
├── go.sum
├── migrations/                       # golang-migrate 风格 up/down SQL
└── internal/
    ├── app/                          # 配置、store、service、server 装配
    ├── config/                       # 环境变量加载与生产约束
    ├── database/                     # pgxpool / readiness
    ├── domain/                       # AppError 和错误码
    ├── health/                       # health payload
    ├── middleware/                   # session cookie、CSRF、幂等、request id
    ├── module/                       # 按业务域组织 model/service/repository contract
    ├── response/                     # JSON 与 Problem Details 响应
    ├── server/                       # chi router、handler、DTO、HTTP helper
    ├── store/postgres/               # PostgreSQL repository 实现
    └── validator/                    # 严格 JSON、If-Match、request hash、时间解析
```

前端目录：

```text
frontend/
├── package.json
├── vite.config.ts
├── src/
│   ├── main.ts
│   ├── App.vue
│   ├── router.ts
│   ├── pages/                        # 页面级路由组件
│   ├── components/                   # 页面内组件和通用 UI 片段
│   ├── queries/                      # TanStack Query hooks
│   ├── stores/                       # Pinia session/UI 状态
│   ├── lib/                          # API facade、真实后端 adapter、工具函数
│   ├── data/                         # mock seed 数据
│   ├── types/                        # 前端共享类型
│   └── theme/                        # 主题配置
├── public/                           # 图标等静态资源
└── dist/                             # 已构建产物
```

## 3. 后端模块结构和职责

后端采用“HTTP 层横向统一 + 业务域纵向模块”的结构，不是单纯的 `handler/ service/ repository/ model/ dto/` 横向目录。HTTP handler 统一在 `internal/server`，业务模型和服务按 `internal/module/<domain>` 聚合。

| 路径 | 职责 |
| --- | --- |
| `backend/cmd/api` | 进程入口：加载配置、创建 app、监听 HTTP。 |
| `backend/internal/config` | 加载 `PORT`、`APP_ENV`、`DATABASE_URL`、OAuth、联系方式加密/HMAC key；生产环境强制 `DATABASE_URL`、`OAUTH_PROVIDER_MODE=oauth2`、关闭 dev auth、配置联系信息密钥。 |
| `backend/internal/app` | 应用装配边界：如果配置 `DATABASE_URL` 则连接 PostgreSQL store，并把同一个 store 注入各业务 repository contract；创建 core service 和 HTTP server。 |
| `backend/internal/server` | HTTP 边界：`routes.go` 注册所有 chi 路由，`*_handler.go` 做请求解析、session/CSRF/If-Match/幂等调用、DTO 映射和响应。 |
| `backend/internal/middleware` | 请求边界小工具：`c2c_session` cookie、`X-CSRF-Token`、`Idempotency-Key`、request id。 |
| `backend/internal/module/core` | 兼容 facade，聚合各模块 service，保留旧 service 方法名并委托到模块服务。 |
| `backend/internal/module/auth` | 用户、session、dev session、站内密码登录、OAuth identity、linux.do binding、权限。 |
| `backend/internal/module/catalog` | 产品分类、产品套餐、风险告知、API model catalog seed/read。 |
| `backend/internal/module/contact` | 联系方式、联系方式版本、联系窗口、访问日志、联系值加密/解密边界。 |
| `backend/internal/module/officialprice` | 官方低价线索、审核、价格记录和幂等审核事务。 |
| `backend/internal/module/carpool` | 车源、风险确认、上车申请、联系窗口、加入确认、membership、完成/退出/移除。 |
| `backend/internal/module/apimarket` | API 服务发布、审核、上线/暂停/恢复、模型/价格/套餐快照。 |
| `backend/internal/module/apiintent` | API 购买意向、冻结买卖双方联系方式版本、意向生命周期。 |
| `backend/internal/module/profile` | 我的资料、公开用户资料、商户资料、公开 slug/store alias。 |
| `backend/internal/module/announcement` | 公告、用户 receipt、公告管理端审计。 |
| `backend/internal/module/demand` | 求车/需求池发布、关闭/重开、管理审核。 |
| `backend/internal/module/favorite` | 当前用户对公开车源和 API 服务的收藏标记。 |
| `backend/internal/module/review` | 已完成拼车 membership 的买家评价车主。 |
| `backend/internal/module/report` | 举报、纠纷、申诉、公开脱敏纠纷摘要。 |
| `backend/internal/module/notification` | 当前用户站内业务通知列表、未读数、已读状态。 |
| `backend/internal/module/search` | 公开市场搜索聚合，只返回公开安全摘要。 |
| `backend/internal/store/postgres` | PostgreSQL 实现，按业务域拆 SQL 文件，共享 pgxpool 和联系信息 crypto。 |
| `backend/internal/validator` | 严格 JSON、未知字段拒绝、1 MiB body 限制、`If-Match` 版本解析、request hash。 |
| `backend/internal/response` | `application/json` 和 `application/problem+json` 输出、ETag 设置。 |

## 4. 前端结构和真实/Mock 分流

### 4.1 技术栈

`frontend/package.json` 显示当前前端为 Vue 3 + Vite + TypeScript，使用 Vue Router、Pinia、TanStack Vue Query、lucide-vue-next、reka-ui/shadcn-vue 风格组件、Tailwind CSS、marked、DOMPurify、Unovis 等。

### 4.2 分流入口

真实后端分流集中在 `frontend/src/lib/backendClient.ts`：

- `VITE_API_MODE=real` 时强制真实模式。
- 或者设置 `VITE_API_BASE_URL` 时也进入真实模式。
- `backendRequest()` 使用 `credentials: 'include'` 发送 cookie。
- `getCurrentBackendSession()` 请求 `/api/v1/auth/session` 并缓存 `csrfToken`。
- `backendMutation()` 自动带 `X-CSRF-Token`，需要时生成 `Idempotency-Key`，需要乐观锁时带 `If-Match`。
- 真实模式下 `ensureBackendSession()` 不再静默调用 `/auth/dev-session`；session 缺失或权限不匹配会抛出 `SESSION_EXPIRED` / `PERMISSION_DENIED`。
- 非真实模式仍可调用 `/api/v1/auth/dev-session` 创建本地开发 session。

### 4.3 API facade 和 backend adapters

`frontend/src/lib/api.ts` 是前端主要 API facade。它仍导入 `frontend/src/data/mock.ts`，内部大量函数采用：

```ts
if (shouldUseRealBackend()) return backendXxx(...)
```

真实后端 adapter 主要位于：

| 文件 | 对接后端域 |
| --- | --- |
| `frontend/src/lib/apiMarketBackend.ts` | API 服务、API purchase intent、owner/admin API 操作。 |
| `frontend/src/lib/carpoolBackend.ts` | 拼车车源、申请、联系窗口、membership 生命周期、admin 拼车审核。 |
| `frontend/src/lib/profileBackend.ts` | 我的资料、联系方式、公开用户页、商户资料。 |
| `frontend/src/lib/officialPriceBackend.ts` | 官方价格、低价线索、admin 审核。 |
| `frontend/src/lib/demandBackend.ts` | 需求池公开/我的/admin。 |
| `frontend/src/lib/favoriteBackend.ts` | 收藏列表、收藏状态、收藏/取消。 |
| `frontend/src/lib/reviewBackend.ts` | 评价中心、公开评价。 |
| `frontend/src/lib/reportBackend.ts` | 举报、纠纷、申诉。 |
| `frontend/src/lib/notificationBackend.ts` | 通知列表、未读数、已读。 |
| `frontend/src/lib/searchBackend.ts` | 全局搜索。 |
| `frontend/src/lib/announcementsApi.ts` | 公告用户端和 admin；真实模式走后端，非真实模式走公告 mock store。 |

### 4.4 仍存在的 Mock / demo 路径

当前前端还没有清理为 real-only：

- `frontend/src/data/mock.ts` 仍存在，包含大量 seed 数据和前端类型。
- `frontend/src/data/announcements.mock.ts` 仍存在。
- `frontend/src/lib/api.ts` 仍保留 `sessionStorage` store，例如 API purchase intent、carpool application、admin audit、official price、carpool、API service、demand、notification read、favorite 等本地状态。
- `frontend/src/lib/announcementsApi.ts` 在非真实模式下也使用 `sessionStorage`。
- `frontend/src/pages/LoginPage.vue` 是当前 `/login` 页面，支持站内用户名密码登录和 linux.do OAuth 登录。
- `frontend/src/router.ts` 中 `/auth/mock` 仍 redirect 到 `/login`，用于兼容早期本地演示入口。
- 多个页面还有“当前 mock 数据 / 本地 mock 状态”的 UI 文案。

因此，上线前建议单独做 “frontend real-only hardening / mock cleanup” 任务：清理 Mock 文案、重命名登录页/route、明确保留或移除 demo 模式，并扫描真实模式下是否还有 mock 数据混入页面。

## 5. PostgreSQL migration 与全部核心表清单

### 5.1 Migration 版本

| 版本 | 范围 |
| --- | --- |
| `000001_extensions_and_identity` | pgcrypto、用户、认证 session、linux.do 绑定、权限、限制、商户资料。 |
| `000002_catalog_and_policy` | 产品分类/套餐、发布策略、风险告知、策略历史。 |
| `000003_idempotency_events_notifications_audit` | 幂等键、领域事件、通知、管理审计日志。 |
| `000004_official_price` | 官方低价线索和官方价格记录。 |
| `000005_contact_methods` | 联系方式和加密联系方式版本。 |
| `000006_contact_sessions` | 联系窗口、窗口条目、访问日志。 |
| `000007_seed_catalog_risk_and_policy` | 初始 catalog、风险告知和发布策略 seed。 |
| `000008_contact_and_foundation_integrity` | 联系方式/联系窗口完整性、幂等约束、官方价格约束和索引。 |
| `000009_carpool_contract` | 拼车车源、车源风险确认、申请、申请风险确认。 |
| `000010_carpool_reservation_and_integrity` | 预约截止、席位语义、owner 联系方式选择、联系窗口一致性、风险版本完整性。 |
| `000011_carpool_membership_lifecycle` | buyer/owner 加入确认、joined applications、active memberships。 |
| `000012_carpool_membership_cycle_lifecycle` | 账期条款、完成确认、completed membership、buyer leave、owner remove。 |
| `000013_api_market_services` | API model catalog、API service 生命周期、接入方式、模型、固定套餐。 |
| `000014_api_market_purchase_intents` | API purchase intent、冻结买家/owner 联系方式版本、意向生命周期。 |
| `000015_api_intent_direct_contacts` | 移除 legacy API intent contact-window，改为直接冻结联系方式披露模型。 |
| `000016_api_intent_contract_hardening` | API intent 接入方式、联系类型/标签快照、联系版本身份约束、状态时间戳约束。 |
| `000017_profile_public_contact` | 用户公开资料字段、公开 username 索引、商户公开 slug 索引。 |
| `000018_announcements` | 公告、用户 receipt、admin 公告审计。 |
| `000019_demands` | 求车/需求池发布、发布者/admin 生命周期、公开 active 索引。 |
| `000020_favorites` | 当前用户收藏公开车源和公开 API 服务。 |
| `000021_reviews` | 已完成拼车 membership 的买家对车主公开评价。 |
| `000022_reports_disputes_appeals` | 举报、纠纷、申诉、追加式 dispute events。 |
| `000023_api_intent_contact_access_logs` | API purchase intent 直接披露联系方式访问审计，不存联系方式明文。 |
| `000024_search_trigram_indexes` | 启用 `pg_trgm` 并为公开搜索字段增加 GIN trigram indexes。 |
| `000025_native_admin_login` | 站内密码 credential 表和无固定密码种子的 admin bootstrap 基础。 |
| `000026_account_identity_profile` | 账号资料、邮箱验证和自定义头像字段。 |
| `000027_api_service_instant_orders` | API 服务接单设置、API orders、订单事件和付款说明读取审计。 |
| `000028_api_order_dispute_targets` | API order 作为举报/纠纷/申诉目标的约束支持。 |
| `000029_feedback_tickets` | 用户反馈工单、补充、admin 处理事件和未读状态。 |
| `000030_carpool_quota_fields` | 拼车服务倍率和平均额度披露字段。 |
| `000031_email_registration_verification` | 邮箱注册 challenge/verification 表；当前公网注册端点仍禁用。 |
| `000032_carpool_cancel_exit_lifecycle` | 买家取消、车主撤回接受和取消联系窗口历史。 |
| `000033_product_plan_quota_unit_carpool` | 套餐额度单位和车源额度单位快照。 |
| `000034_api_model_provider_catalog` | 可管理 API model provider 与 provider-backed model catalog。 |
| `000035_password_argon2_admin_bootstrap` | Argon2id 密码算法和固定 admin seed 清理。 |
| `000036_search_trigram_alignment` | 商户资料搜索 trigram expression 对齐到 display-name-only 公开搜索。 |

### 5.2 全部核心表清单

当前 migrations 创建的表：

```text
admin_audit_logs
announcement_audit_logs
announcement_receipts
announcements
api_model_catalog
api_model_price_versions
api_model_providers
api_order_events
api_order_payment_instruction_access_logs
api_orders
api_purchase_intent_contact_access_logs
api_purchase_intents
api_service_access_modes
api_service_models
api_service_packages
api_service_payment_options
api_services
appeals
auth_identities
auth_sessions
carpool_application_policy_acknowledgements
carpool_applications
carpool_completion_confirmations
carpool_cycle_terms
carpool_join_confirmations
carpool_listing_policy_acknowledgements
carpool_listings
carpool_memberships
carpool_reviews
contact_access_logs
contact_method_versions
contact_methods
contact_session_items
contact_sessions
demands
dispute_cases
dispute_events
domain_events
email_verification_codes
favorites
feedback_events
feedback_tickets
idempotency_keys
linux_do_bindings
merchant_profiles
notifications
official_price_leads
official_price_records
product_categories
product_plan_policy_history
product_plans
reports
risk_notice_versions
risk_notices
user_permissions
user_password_credentials
user_restrictions
users
```

## 6. 业务模块对应的数据表

| 业务模块 | 主要表 |
| --- | --- |
| 身份 / 登录 / 权限 | `users`、`auth_identities`、`auth_sessions`、`user_password_credentials`、`linux_do_bindings`、`user_permissions`、`user_restrictions` |
| 商户资料 / 公开资料 | `merchant_profiles`，以及 `users` 上的 profile/privacy/public username 字段 |
| 产品目录 / 风险策略 | `product_categories`、`product_plans`、`product_plan_policy_history`、`risk_notices`、`risk_notice_versions` |
| 幂等 / 领域事件 / 审计 / 通知 | `idempotency_keys`、`domain_events`、`admin_audit_logs`、`notifications` |
| 官方价格 | `official_price_leads`、`official_price_records` |
| 联系方式 / 联系窗口 | `contact_methods`、`contact_method_versions`、`contact_sessions`、`contact_session_items`、`contact_access_logs` |
| 拼车 | `carpool_listings`、`carpool_listing_policy_acknowledgements`、`carpool_applications`、`carpool_application_policy_acknowledgements`、`carpool_join_confirmations`、`carpool_memberships`、`carpool_cycle_terms`、`carpool_completion_confirmations` |
| API 服务市场 | `api_model_catalog`、`api_model_price_versions`、`api_services`、`api_service_access_modes`、`api_service_models`、`api_service_packages` |
| API 购买意向 | `api_purchase_intents`、`api_purchase_intent_contact_access_logs` |
| 公告 | `announcements`、`announcement_receipts`、`announcement_audit_logs` |
| 需求池 | `demands` |
| 收藏 | `favorites` |
| 评价 | `carpool_reviews` |
| 举报 / 纠纷 / 申诉 | `reports`、`dispute_cases`、`appeals`、`dispute_events` |
| 搜索 | 无独立表；从官方价格、车源、需求、API 服务、用户/商户公开资料聚合读取；`000024` 增加 `pg_trgm`/GIN 索引，`000036` 将商户资料索引对齐为 display-name-only。 |

## 7. API 路由按域分组

所有业务 API 当前挂在 `/api/v1` 下，系统探活例外。

### 7.1 System

```text
GET /health
GET /readyz
```

### 7.2 Auth / Session / OAuth

```text
POST /api/v1/auth/dev-session              # development/test only
POST /api/v1/auth/password/login
GET  /api/v1/auth/oauth/start
GET  /api/v1/auth/oauth/callback
GET  /api/v1/auth/session
POST /api/v1/auth/logout
```

### 7.3 Search

```text
GET /api/v1/search
```

### 7.4 Profile / Contact / Merchant Profile

```text
GET    /api/v1/me/profile
PATCH  /api/v1/me/profile
GET    /api/v1/me/contact-methods
POST   /api/v1/contact-methods
PATCH  /api/v1/contact-methods/{id}
DELETE /api/v1/contact-methods/{id}
POST   /api/v1/contact-methods/{id}/set-default
POST   /api/v1/contact-methods/{id}/verify
GET    /api/v1/me/merchant-profile
POST   /api/v1/me/merchant-profile
PATCH  /api/v1/me/merchant-profile
GET    /api/v1/users/{username}/public-profile
GET    /api/v1/users/{username}/reviews
GET    /api/v1/users/{username}/disputes
GET    /api/v1/merchant-profiles/{slug}
```

### 7.5 Announcements

```text
GET   /api/v1/announcements
GET   /api/v1/announcements/active
GET   /api/v1/announcements/home
GET   /api/v1/announcements/{slug}
GET   /api/v1/me/announcements/unread-count
GET   /api/v1/me/announcements/important-unread-count
POST  /api/v1/me/announcements/{id}/seen
POST  /api/v1/me/announcements/{id}/read
POST  /api/v1/me/announcements/{id}/dismiss
GET   /api/v1/admin/announcements
POST  /api/v1/admin/announcements
GET   /api/v1/admin/announcements/{id}
PATCH /api/v1/admin/announcements/{id}
POST  /api/v1/admin/announcements/{id}/publish
POST  /api/v1/admin/announcements/{id}/offline
POST  /api/v1/admin/announcements/{id}/duplicate
GET   /api/v1/admin/announcement-audit-logs
```

### 7.6 Catalog / Product / API Models

```text
GET /api/v1/product-categories
GET /api/v1/product-plans
GET /api/v1/product-plans/{id}
GET /api/v1/api-models
GET /api/v1/api-models/{id}
```

### 7.7 Official Price

```text
GET  /api/v1/official-prices
GET  /api/v1/official-prices/{id}
POST /api/v1/official-price-leads
GET  /api/v1/me/official-price-leads
GET  /api/v1/me/official-price-leads/{id}
GET  /api/v1/admin/official-price-leads
GET  /api/v1/admin/official-price-leads/{id}
POST /api/v1/admin/official-price-leads/{id}/approve
POST /api/v1/admin/official-price-leads/{id}/reject
POST /api/v1/admin/official-price-leads/{id}/request-changes
```

### 7.8 Demands

```text
GET  /api/v1/demands
POST /api/v1/demands
GET  /api/v1/demands/{id}
GET  /api/v1/me/demands
GET  /api/v1/me/demands/{id}
POST /api/v1/me/demands/{id}/close
POST /api/v1/me/demands/{id}/reopen
GET  /api/v1/admin/demands
GET  /api/v1/admin/demands/{id}
POST /api/v1/admin/demands/{id}/approve
POST /api/v1/admin/demands/{id}/request-changes
POST /api/v1/admin/demands/{id}/reject
POST /api/v1/admin/demands/{id}/take-down
POST /api/v1/admin/demands/{id}/restore
```

### 7.9 Favorites

```text
GET    /api/v1/me/favorites
GET    /api/v1/me/favorites/{targetType}/{targetId}
PUT    /api/v1/me/favorites/{targetType}/{targetId}
DELETE /api/v1/me/favorites/{targetType}/{targetId}
```

### 7.10 Reviews

```text
GET /api/v1/me/reviews
PUT /api/v1/me/reviews/carpool-memberships/{membershipId}
GET /api/v1/users/{username}/reviews
```

### 7.11 Reports / Disputes / Appeals

```text
POST /api/v1/reports
GET  /api/v1/me/reports
POST /api/v1/me/appeals
GET  /api/v1/me/appeals
GET  /api/v1/users/{username}/disputes
GET  /api/v1/admin/reports
GET  /api/v1/admin/reports/{id}
POST /api/v1/admin/reports/{id}/triage
POST /api/v1/admin/reports/{id}/reject
POST /api/v1/admin/reports/{id}/open-dispute
GET  /api/v1/admin/disputes
GET  /api/v1/admin/disputes/{id}
POST /api/v1/admin/disputes/{id}/request-info
POST /api/v1/admin/disputes/{id}/resolve
POST /api/v1/admin/disputes/{id}/close
GET  /api/v1/admin/appeals
GET  /api/v1/admin/appeals/{id}
POST /api/v1/admin/appeals/{id}/approve
POST /api/v1/admin/appeals/{id}/reject
```

### 7.12 Notifications

```text
GET  /api/v1/me/notifications
GET  /api/v1/me/notifications/unread-count
POST /api/v1/me/notifications/{id}/read
POST /api/v1/me/notifications/read-all
```

### 7.13 Carpool

```text
GET   /api/v1/carpools
POST  /api/v1/carpools
GET   /api/v1/carpools/{id}
PATCH /api/v1/carpools/{id}
POST  /api/v1/carpools/{id}/submit-review
POST  /api/v1/carpools/{id}/applications
GET   /api/v1/me/carpools
GET   /api/v1/me/carpool-applications
GET   /api/v1/me/carpool-applications/{id}
POST  /api/v1/me/carpool-applications/{id}/confirm-join
GET   /api/v1/me/carpool-memberships
POST  /api/v1/me/carpool-memberships/{id}/confirm-complete
POST  /api/v1/me/carpool-memberships/{id}/leave
GET   /api/v1/owner/carpool-applications
GET   /api/v1/owner/carpool-applications/{id}
POST  /api/v1/owner/carpool-applications/{id}/accept
POST  /api/v1/owner/carpool-applications/{id}/reject
POST  /api/v1/owner/carpool-applications/{id}/confirm-join
GET   /api/v1/owner/carpool-memberships
POST  /api/v1/owner/carpool-memberships/{id}/confirm-complete
POST  /api/v1/owner/carpool-memberships/{id}/remove
GET   /api/v1/admin/carpools
GET   /api/v1/admin/carpools/{id}
POST  /api/v1/admin/carpools/{id}/approve
POST  /api/v1/admin/carpools/{id}/reject
POST  /api/v1/admin/carpools/{id}/request-changes
POST  /api/v1/admin/carpools/{id}/pause
POST  /api/v1/admin/carpools/{id}/restore
```

### 7.14 API Market / API Purchase Intents

```text
GET   /api/v1/api-services
GET   /api/v1/api-services/{id}
POST  /api/v1/api-services/{id}/purchase-intents
GET   /api/v1/me/api-purchase-intents
GET   /api/v1/me/api-purchase-intents/{id}
POST  /api/v1/me/api-purchase-intents/{id}/cancel
GET   /api/v1/owner/api-services
GET   /api/v1/owner/api-services/{id}
POST  /api/v1/owner/api-services
PATCH /api/v1/owner/api-services/{id}
POST  /api/v1/owner/api-services/{id}/submit-review
POST  /api/v1/owner/api-services/{id}/publish
POST  /api/v1/owner/api-services/{id}/pause
POST  /api/v1/owner/api-services/{id}/resume
POST  /api/v1/owner/api-services/{id}/start-revision
GET   /api/v1/owner/api-purchase-intents
GET   /api/v1/owner/api-purchase-intents/{id}
POST  /api/v1/owner/api-purchase-intents/{id}/mark-contacted
POST  /api/v1/owner/api-purchase-intents/{id}/close
GET   /api/v1/admin/api-services
GET   /api/v1/admin/api-services/{id}
POST  /api/v1/admin/api-services/{id}/approve
POST  /api/v1/admin/api-services/{id}/request-changes
POST  /api/v1/admin/api-services/{id}/reject
POST  /api/v1/admin/api-services/{id}/suspend
POST  /api/v1/admin/api-services/{id}/restore
POST  /api/v1/admin/api-services/{id}/remove
GET   /api/v1/admin/api-purchase-intents
GET   /api/v1/admin/api-purchase-intents/{id}
```

### 7.15 Dev Contact Sessions

```text
POST /api/v1/dev/contact-sessions          # development/test only
GET  /api/v1/contact-sessions/{id}/contacts
```

## 8. 鉴权、CSRF、幂等、If-Match、错误码约定

### 8.1 Session 鉴权

- Session cookie 名称为 `c2c_session`。
- `GET /api/v1/auth/session` 返回当前用户、权限、linux.do 绑定摘要、`csrfToken` 和过期时间。
- `requireSession()` 只校验 session cookie。
- `requireSessionAndCSRF()` 校验 session cookie 和 `X-CSRF-Token`，用于状态变更类请求。
- dev auth 只在 `APP_ENV=development` 或 `APP_ENV=test` 默认启用；生产必须关闭 `ENABLE_DEV_AUTH=false`。
- OAuth 生产模式必须使用 `OAUTH_PROVIDER_MODE=oauth2`，并配置 client、authorize/token/userinfo/redirect URL。
- 生产环境必须配置 `FRONTEND_ORIGIN` 或 `ALLOWED_ORIGINS`；cookie 认证 CORS 不允许使用 wildcard origin。
- 生产 `c2c_session` 和 OAuth state cookie 使用 `HttpOnly=true`、`Secure=true`、`SameSite=Lax`，清理 cookie 使用相同 Path/Secure/SameSite 属性。

### 8.2 CSRF

- Header 名称：`X-CSRF-Token`。
- 前端真实模式从 `/api/v1/auth/session` 读取 token 后缓存在 `backendClient.ts`。
- `backendMutation()` 自动携带 `X-CSRF-Token`。
- 缺失或错误返回 `403 CSRF_TOKEN_INVALID`。

### 8.3 幂等

- Header 名称：`Idempotency-Key`。
- 前端 `backendMutation()` 可按业务 prefix 自动生成 key。
- 后端按 `userID + routeKey + Idempotency-Key + requestHash` 建立幂等记录。
- 同 key 同请求完成后返回缓存的完成响应；同 key 不同 body/route 会返回 `IDEMPOTENCY_KEY_REUSED`；进行中返回 `IDEMPOTENCY_IN_PROGRESS`。
- 对于联系方式等敏感响应，后端使用 `Cache-Control: private, no-store` 或 `no-store`，幂等记录只保存资源标识或受控响应，不缓存不该持久化的完整明文联系方式。
- 未过期 `processing` 幂等记录返回 `IDEMPOTENCY_IN_PROGRESS`；同 request hash 且已过期的 `processing` 记录允许接管重试；同 key 不同 request hash 仍返回 `IDEMPOTENCY_KEY_REUSED`。

### 8.4 If-Match / ETag

- 后端通过 `ETag: "<version>"` 暴露资源版本。
- 需要乐观锁的审核/状态变更必须携带 `If-Match`。
- `validator.RequireIfMatchVersion()` 要求 `If-Match` 为正整数版本；缺失或非法返回 `428 PRECONDITION_REQUIRED`。
- 版本不匹配返回 `VERSION_CONFLICT`。
- 前端 `backendMutation()` 支持 `ifMatch` 参数并按 `"version"` 格式发送。

### 8.5 错误响应与错误码

错误响应统一使用 `application/problem+json`，字段：

```json
{
  "type": "https://c2cmarket.local/problems/validation-failed",
  "title": "Invalid JSON",
  "status": 400,
  "code": "VALIDATION_FAILED",
  "detail": "请求 JSON 格式不正确或包含未知字段。",
  "instance": "/api/v1/...",
  "requestId": "...",
  "errors": [
    { "field": "fieldName", "code": "invalid", "message": "..." }
  ]
}
```

当前核心错误码包括：

```text
ACCOUNT_RESTRICTED
ACTIVE_APPLICATION_EXISTS
ACTIVE_API_INTENT_EXISTS
ACTIVE_MEMBERSHIP_EXISTS
CONTACT_ACCESS_FORBIDDEN
CONTACT_METHOD_DISABLED
CONTACT_METHOD_NOT_OWNED
CONTACT_METHOD_REQUIRED
CONTACT_WINDOW_EXPIRED
CSRF_TOKEN_INVALID
FIELD_NOT_ALLOWED
IDEMPOTENCY_IN_PROGRESS
IDEMPOTENCY_KEY_REUSED
INTERNAL_ERROR
INVALID_STATE_TRANSITION
JOIN_CONFIRMATION_EXPIRED
MEMBERSHIP_NOT_ACTIVE
MERCHANT_CONTACT_REQUIRED
MERCHANT_CONTACT_UNAVAILABLE
OBJECT_NOT_FOUND
PERMISSION_DENIED
PRECONDITION_REQUIRED
PRICE_NORMALIZATION_REQUIRED
PRODUCT_PLAN_RESOLUTION_REQUIRED
RATE_LIMITED
RISK_ACK_REQUIRED
SECRET_CONTENT_DETECTED
SEAT_UNAVAILABLE
SESSION_EXPIRED
SESSION_REVOKED
URL_NOT_ALLOWED
VALIDATION_FAILED
VERSION_CONFLICT
```

### 8.6 Hardening 约定

- `cmd/api` 使用显式 `http.Server`，timeout 为 `ReadHeaderTimeout=5s`、`ReadTimeout=15s`、`WriteTimeout=30s`、`IdleTimeout=60s`。
- OAuth token exchange 和 userinfo 请求使用 10 秒 timeout 的专用 HTTP client，响应体读取限制为 1 MiB。
- 基础限流当前是进程内 1 分钟窗口，保护 OAuth、search、API purchase intent 创建、联系方式读取、举报/申诉创建和 dev-only 入口；超限返回 `429 RATE_LIMITED` Problem Details。
- 主要列表接口支持 `limit` / opaque `cursor`，默认 20、最大 100，响应为 `{ items, nextCursor }`。
- API purchase intent 创建、买家详情和 owner 详情只在授权响应中披露冻结联系方式，响应 `Cache-Control: no-store`，并写入 `api_purchase_intent_contact_access_logs`，日志不含明文联系方式。
- 后端设置 `X-Content-Type-Options: nosniff`、`Referrer-Policy: strict-origin-when-cross-origin`，生产设置 HSTS；CSP 由前端静态托管或反向代理配置。

## 9. 当前已完成范围

基于 `README.md`、`backend/README.md`、routes、migrations 和 smoke 脚本，当前已完成本地真实闭环的范围包括：

- API 集市：模型目录、服务发布、送审、admin 审核、owner 上线/暂停/恢复、买家购买意向、买家/owner 联系方式读取和不含明文的联系方式访问审计。
- 拼车：车源发布、送审、admin 审核、公开列表/详情、买家申请、owner 接受/拒绝、30 分钟联系窗口、双方确认上车、membership 完成/退出/移除。
- 个人资料与联系方式：我的资料、联系方式管理、公开用户页、商户资料、store alias API 服务展示。
- 公告：用户端列表/banner/详情、已见/已读/关闭、未读数、admin 创建/编辑/发布/下线/复制/审计。
- 官方低价/价格情报：公开价格列表/详情、提交低价线索、我的线索、admin 审核通过/复核/拒绝、首页行情引用真实价格记录。
- 需求池：需求发布、公开列表/详情、我的需求、关闭/重开、admin 审核通过/要求修改/拒绝/下架/恢复。
- 收藏：车源和 API 服务收藏状态、收藏、取消收藏、我的收藏列表。
- 评价中心：已完成拼车 membership 的买家评价车主、评价中心查看/修改、公开用户主页展示。
- 举报/纠纷/申诉：联系方式举报、公开用户举报、admin 举报处理、纠纷打开/处理、用户申诉、admin 申诉处理和公开主页脱敏纠纷摘要。
- 通知中心：业务通知列表、未读数、单条已读、全部已读。
- 全局搜索：公开官方价格、车源、求车、API 服务、公开用户和公开身份 API 商户搜索，PostgreSQL 已补 `pg_trgm` 搜索索引。
- 登录/权限：站内用户名密码登录、OAuth start/callback、真实 session、linux.do 绑定摘要、权限返回；本地 smoke 使用 fake OAuth provider。
- 部署准备：生产 env 模板、生产 Compose 覆盖、部署 runbook、后端 Docker build、migration/readyz 流程、全量 smoke runner 和后端上线前 hardening。

当前 smoke 脚本位于 `scripts/`：

```text
auth-smoke.mjs
official-price-smoke.mjs
api-market-smoke.mjs
carpool-smoke.mjs
profile-smoke.mjs
announcement-smoke.mjs
demand-smoke.mjs
favorites-smoke.mjs
review-smoke.mjs
reports-smoke.mjs
notification-smoke.mjs
search-smoke.mjs
run-smokes.mjs
```

## 10. 未覆盖的真实生产外部条件

以下不是当前源码内已经真实接入并验证完毕的生产外部条件：

- 真实 OAuth provider：代码支持 `oauth2`，但生产需要实际 provider app、client secret、redirect URL、scope 和回调域名配置。
- 正式域名、TLS、反向代理：Compose 只跑 backend/postgres，不包含 Nginx/Caddy/Ingress、HTTPS 证书和静态前端托管配置。
- 前端生产托管：需要将 `frontend/dist/` 部署到静态服务器/CDN，并配置 SPA fallback 和 API origin。
- 数据库生产运维：需要备份、恢复演练、监控、容量规划、升级策略、只读副本或高可用方案。
- 机密管理：当前通过 env 注入；生产仍需接入密钥管理、轮换流程、权限隔离和泄漏应急流程。
- 日志、指标、Tracing、告警：当前没有接入外部 APM/Prometheus/Sentry/日志平台。
- 邮件、短信、Webhook、移动推送：通知中心只是站内业务通知表，不发送外部消息。
- 对象存储/真实文件上传：头像等当前没有生产对象存储链路。
- 管理后台生产审计运营流程：有 admin audit / announcement audit / dispute events 表，但还需要实际运营角色、权限发放和处理 SOP。
- 生产真实支付/托管/退款/担保：产品边界明确不提供，也不应补成平台能力。
- 凭据保管/自动交付/API 代理：产品边界明确不保存第三方账号密码、API key、token、cookie、session、MFA/recovery code，也不代理上游 API 流量；站内账号密码只允许以不可逆哈希保存。
- 前端 real-only 清理：真实模式可用，但 Mock/demo 数据和文案仍存在，建议上线前单独清理或明确隔离。
- 生产环境 smoke：本地脚本可跑 fake OAuth/dev auth；生产只能做受控真实登录和非破坏性验证，不能直接用 dev session。

## 11. 给 GPT Pro 的审阅提示

建议重点审阅：

1. 当前 `internal/module/<domain>` + `internal/server/*_handler.go` 的结构是否已经足够适合继续扩展，是否需要进一步把 HTTP handler 也按 domain 子目录拆分。
2. `frontend/src/lib/api.ts` 仍混合真实 adapter 和 Mock store，是否应在上线前拆成 real-only facade 和 demo-only facade。
3. PostgreSQL 表和路由是否覆盖当前产品闭环，特别是 API purchase intent、拼车 membership、举报/纠纷/申诉、通知和搜索的边界是否一致。
4. 幂等、CSRF、If-Match/ETag、Problem Details 错误响应是否满足当前阶段的 API 契约稳定性。
5. 生产化缺口中哪些是上线前必须项，哪些可以作为上线后增强项。

主要证据文件：

```text
README.md
backend/README.md
backend/internal/server/routes.go
backend/internal/config/config.go
backend/internal/app/app.go
backend/internal/domain/errors.go
backend/internal/response/response.go
backend/internal/validator/request.go
backend/migrations/*.up.sql
backend/migrations/README.md
frontend/src/lib/backendClient.ts
frontend/src/lib/api.ts
frontend/src/lib/*Backend.ts
frontend/src/data/mock.ts
frontend/src/data/announcements.mock.ts
docs/openapi/c2c-market-api-v1.yaml
docs/ops/deployment-runbook.md
```
