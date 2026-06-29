# C2C Market

日期：2026-06-23
执行者：Codex

C2C Market 是一个前后端分离项目：

- `frontend/`：Vue 3 + Vite + TypeScript 前端
- `backend/`：Go HTTP 后端
- `docs/`：OpenAPI 与部署运维文档
- `scripts/`：本地 smoke 验证脚本

## 当前完成范围

当前本地真实业务闭环已经覆盖：

- API 集市：模型目录、服务发布、送审、admin 审核、商户上线/暂停/恢复、买家购买意向、买家/商户双方联系方式读取。
- 拼车：已绑定 linux.do 的车主直接发布公开车源、admin 下架/恢复治理、遗留审核队列处理、公开列表/详情、买家申请、owner 接受/拒绝、30 分钟联系窗口、双方确认上车、membership 完成/退出/移除；每人每月额度的名称、单位和周期由套餐目录配置。
- 个人资料与联系方式：我的资料、联系方式管理、公开用户页、商户资料、store alias API 服务展示。
- 公告：用户端列表/banner/详情、已见/已读/关闭、未读数、admin 创建/编辑/发布/下线/复制/审计。
- 官方低价 / 价格情报：公开价格列表/详情、提交低价线索、我的线索、admin 审核通过/复核/拒绝、首页行情引用真实价格记录。
- 需求池 / 求车：需求发布、公开列表/详情、我的需求、关闭/重开、admin 审核通过/要求修改/拒绝/下架/恢复、首页与搜索 facade 真实读取。
- 收藏：车源和 API 服务收藏状态、收藏、取消收藏、我的收藏列表，真实模式走 Go 后端和 PostgreSQL favorites 表。
- 评价中心：已完成拼车 membership 的买家评价车主、评价中心查看/修改、公开用户主页展示，真实模式走 Go 后端和 PostgreSQL carpool_reviews 表。
- 举报 / 纠纷 / 申诉：联系方式举报、公开用户举报、admin 举报处理、纠纷打开/处理、用户申诉、admin 申诉处理和公开主页脱敏纠纷摘要，真实模式走 Go 后端和 PostgreSQL reports/dispute_cases/appeals 表。
- 统一通知中心：站内业务通知列表、未读数、单条已读、全部已读，真实模式走 Go 后端和 PostgreSQL notifications 表；公告 receipt 仍由公告模块独立负责。
- 全局搜索：公开官方价格、车源、求车、API 服务、公开用户和公开身份 API 商户搜索，真实模式走 Go 后端和 PostgreSQL public predicates；store alias 只作为 API 服务结果展示公开店铺名，不反查隐藏用户。
- 真实登录 / 权限：后端提供站内用户名密码登录、OAuth start/callback、真实 session、linux.do 绑定摘要和权限返回；本地 smoke 使用 fake OAuth provider，生产必须使用 `OAUTH_PROVIDER_MODE=oauth2` 并禁止 dev auth；前端真实模式不再自动调用 `/auth/dev-session` 切换身份。
- 邮箱发送：开发/测试环境使用 development sender 并返回邮箱验证码 `devCode` 便于本地自动化；生产环境使用阿里云 DirectMail SMTP，需配置 465 TLS SMTP 账号后才允许启动。OAuth 注册成功邮件仅在 provider 返回有效邮箱且本次创建新用户后发送，发送失败不阻断注册。
- 部署运维与 hardening：生产 env 模板、生产 Compose 覆盖、部署 runbook、后端 Docker build、migration/readyz 流程和全量 smoke runner 已补齐；后端入口已配置 HTTP server timeout、生产 cookie `Secure`、OAuth 请求 timeout/响应大小限制、CORS/Origin allowlist、基础限流、安全响应头、分页契约、幂等 processing 过期恢复、API purchase intent 联系方式访问审计和搜索 trigram indexes。

当前路线图内业务化模块已完成本地真实闭环和部署运维准备。公告 PostgreSQL update 遗留 bug 已修复；PostgreSQL 路径下 `scripts/announcement-smoke.mjs` 已覆盖 admin 创建、编辑、发布、下线、复制、审计和用户 receipt 流程。

## 产品边界

C2CMarket 是信息发布、价格情报与社区撮合平台，不是支付、托管、账号托管或 API 代理平台。

平台允许用户发布和参与包括 ChatGPT Plus、ChatGPT Pro、ChatGPT Business 在内的订阅费用分摊或拼车信息。此类活动不是 OpenAI 或其他服务提供商提供、授权或担保的官方服务，可能受到服务提供商账号、成员、访问权限或使用规则限制，并可能造成账号限制、成员移除、工作区停用、封号、额度提前耗尽、聊天记录或个人数据暴露、费用损失及服务中断。用户应自行核对适用规则，在充分知情后自愿参与并自行承担风险。

平台不处理站内支付，不提供履约担保，不保存、展示、传递或托管第三方账号密码、API Key、Token、Cookie、Session、验证码、恢复码、面板主账号凭据等认证材料，也不代理上游 API 流量。站内账号密码只允许以不可逆哈希形式保存，不能保存明文密码。

## 前端

```bash
cd frontend
pnpm install
pnpm dev
```

## 后端

```bash
cd backend
go run ./cmd/api
```

默认监听 `:8080`，健康检查接口：

```text
GET /health
GET /readyz
```

生产环境必须配置允许的前端来源，推荐同时设置：

```text
FRONTEND_ORIGIN=https://app.example.com
ALLOWED_ORIGINS=https://app.example.com
```

`ALLOWED_ORIGINS` 可用英文逗号配置多个 origin。后端 cookie 认证不会在 CORS 中使用 `*`，生产状态变更请求会校验 `Origin` 是否在 allowlist 内。本地 development/test 在未配置时默认允许 `127.0.0.1` / `localhost` 的 Vite 开发和预览端口。

后端列表接口的统一分页参数为 `limit` / `cursor`，默认 `limit=20`，最大 `100`，响应统一包含 `items` 和可空 `nextCursor`。当前 cursor 是不透明 base64url offset cursor，调用方只能透传，不应解析内部结构。

生产邮件发送使用阿里云 DirectMail SMTP，不使用 AccessKey 或阿里云 API SDK。当前 `.env.production.example` 已预留 `EMAIL_PROVIDER=aliyun_directmail`、`SMTP_HOST`、`SMTP_PORT=465`、`SMTP_USERNAME`、`SMTP_PASSWORD`、`MAIL_FROM_ADDRESS` 和 `MAIL_FROM_NAME`；填入阿里云 DirectMail 控制台生成的 SMTP 账号和密码即可。生产环境缺少这些字段会启动失败，避免验证码或注册成功邮件在没有真实邮件发送能力时静默成功。

## Docker / PostgreSQL

本地开发数据库使用 Docker Compose：

```bash
cp .env.example .env
docker compose up -d postgres
docker compose --profile migrate run --rm migrate
docker compose ps
```

默认连接信息：

```text
postgres://c2c_market:c2c_market_dev_password@localhost:5432/c2c_market?sslmode=disable
```

PostgreSQL migration 通过 Compose 的一次性 `migrate` 服务执行，SQL 位于 `backend/migrations/`。

如果需要重新初始化数据库和重新执行 migration：

```bash
docker compose down -v
docker compose up -d postgres
docker compose --profile migrate run --rm migrate
```

后端服务镜像已经可以构建：

```bash
docker compose --profile app build backend
docker compose --profile app up -d backend
```

生产模拟部署使用独立 env 模板和 Compose 覆盖：

```bash
cp .env.production.example .env.production
docker compose --env-file .env.production -f compose.yaml -f compose.prod.yaml config
docker compose --env-file .env.production -f compose.yaml -f compose.prod.yaml --profile migrate run --rm migrate
docker compose --env-file .env.production -f compose.yaml -f compose.prod.yaml --profile app build backend
docker compose --env-file .env.production -f compose.yaml -f compose.prod.yaml --profile app up -d backend
```

完整部署、发布、readyz、smoke 和回滚流程见 [docs/ops/deployment-runbook.md](docs/ops/deployment-runbook.md)。

注意：当前 Go 后端入口为 `backend/cmd/api`，`internal/app` 负责依赖组装，`internal/server` 负责 HTTP 路由。业务包已按模块拆分：`auth`、`idempotency`、`contact`、`catalog`、`officialprice`、`carpool`、`apimarket`、`apiintent`、`profile`、`announcement`、`demand`、`favorite`、`review`、`report`、`notification`、`search` 均已拥有各自 service 与 repository contract；`internal/module/core` 只保留兼容方法名并委托到模块服务。

配置 `DATABASE_URL` 后，后端会把 users、auth sessions、user password credential hashes、idempotency、official price leads/records、contact methods、contact sessions、contact access logs、API purchase intent contact access logs、carpool listings/applications/memberships、API services、API purchase intents、profile/merchant profile、announcements、demands、favorites、carpool reviews、reports、dispute cases、appeals、dispute events 和 notifications 写入 PostgreSQL，并从这些公开可见业务表聚合全局搜索。联系方式完整值使用本地配置的 AES-GCM key 加密落库，并用 HMAC fingerprint 做不可逆指纹；拼车联系方式只在有效联系窗口内向参与方解密返回。拼车发布要求车主账号已绑定 linux.do，发布时复查产品 `publish_policy` 和车主联系方式后直接进入 `active`，admin 可对公开车源下架/恢复，遗留 `pending_review` 仍支持人工 approve/request-changes/reject。API 服务当前采用早期自动通过策略：owner 提交审核时若已绑定 linux.do 且商户联系方式有效，会进入 `review_status=approved`、`publication_status=offline`，仍必须手动 publish 后才公开；人工审核状态枚举和 admin approve/request-changes/reject 路由保留。API purchase intent 创建会在同一事务内写入意向、冻结双方联系方式版本、写入事件/通知、记录买家查看商户联系方式的审计日志，并直接向成功创建意向的买家返回冻结商户联系方式；买家详情和 owner 详情读取联系方式也会写入不含明文的访问日志。幂等记录只保存资源标识，不缓存完整联系方式明文；未过期 `processing` 仍返回处理中，过期 `processing` 可由同请求 hash 接管重试，并在启动时清理保守过期记录。需求池只保存求车上下文、预算、地区、偏好、来源链接和审核状态，不处理支付、托管、担保或凭据交付。收藏只保存当前用户对公开车源或公开 API 服务的个人标记，不改变目标资源状态。评价只允许已完成拼车 membership 的买家评价车主，不改变成员关系、支付、托管、担保或凭据交付状态。举报/纠纷/申诉只记录脱敏问题描述、人工处理状态和公开摘要，不处理支付、退款、赔付、托管、担保或凭据交付。统一通知中心只读取和更新当前用户站内业务通知的 `read_at`，不会把业务通知外发为短信、Webhook 或真实推送；邮箱验证码和注册成功邮件由 profile/auth 模块通过 development sender 或阿里云 DirectMail SMTP 发送。全局搜索只返回公开可见摘要，不返回联系方式、隐藏 store alias owner、admin 内部字段或凭据材料；PostgreSQL 上通过 `pg_trgm` GIN 索引优化公开搜索字段。`GET /readyz` 会检查数据库和 migration 状态。

## 验证

```bash
cd frontend && pnpm build
cd backend && go test ./...
```

当前机器的默认 `node` 是 v14，运行前端命令时建议先切到 Node 24，或显式设置：

```bash
VITE_API_MODE=real pnpm --dir frontend build
```

当前真实业务 smoke 脚本：

```bash
API_BASE_URL=http://127.0.0.1:18080 node scripts/api-market-smoke.mjs
API_BASE_URL=http://127.0.0.1:18080 node scripts/carpool-smoke.mjs
API_BASE_URL=http://127.0.0.1:18080 node scripts/profile-smoke.mjs
API_BASE_URL=http://127.0.0.1:18080 node scripts/announcement-smoke.mjs
API_BASE_URL=http://127.0.0.1:18080 node scripts/official-price-smoke.mjs
API_BASE_URL=http://127.0.0.1:18080 node scripts/demand-smoke.mjs
API_BASE_URL=http://127.0.0.1:18080 node scripts/favorites-smoke.mjs
API_BASE_URL=http://127.0.0.1:18080 node scripts/review-smoke.mjs
API_BASE_URL=http://127.0.0.1:18080 node scripts/reports-smoke.mjs
API_BASE_URL=http://127.0.0.1:18080 node scripts/notification-smoke.mjs
API_BASE_URL=http://127.0.0.1:18080 node scripts/search-smoke.mjs
API_BASE_URL=http://127.0.0.1:18080 node scripts/auth-smoke.mjs
```

也可以串行运行当前全部 smoke：

```bash
API_BASE_URL=http://127.0.0.1:18080 node scripts/run-smokes.mjs
```
