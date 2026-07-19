<p align="center">
  <img src="./frontend/public/c2cmarket-logo-mark.svg" alt="C2CMarket" width="88" height="88">
</p>

<h1 align="center">C2CMarket</h1>

<p align="center">
  面向 linux.do 社区的订阅拼车、API 服务、求车需求与官网价格信息撮合平台。
</p>

<p align="center">
  <a href="./README.md">简体中文</a> · <a href="./README_EN.md">English</a>
</p>

<p align="center">
  <a href="https://github.com/xiangrikuil/c2cmarket/actions/workflows/ci.yml"><img src="https://github.com/xiangrikuil/c2cmarket/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="./LICENSE"><img src="https://img.shields.io/badge/license-MIT-green.svg" alt="MIT License"></a>
  <img src="https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white" alt="Go 1.26">
  <img src="https://img.shields.io/badge/Vue-3-42b883?logo=vuedotjs&logoColor=white" alt="Vue 3">
</p>

> [!IMPORTANT]
> C2CMarket 仍在积极开发中，接口、数据库迁移和部署配置可能发生变化。部署到生产环境前，请完整检查配置、业务规则和当地适用要求。

## 项目简介

C2CMarket 是一个前后端分离的社区信息撮合平台。它帮助用户浏览、发布和管理订阅拼车、API 服务、求车需求及官网价格记录，并提供订单跟踪、通知、评价、举报和管理后台等完整业务模块。

平台专注于信息展示、意向撮合、站外联系和信誉记录，不处理站内支付，不提供托管或履约担保，也不代理上游 API 流量。

## 主要功能

- **订阅拼车**：车源发布、申请、联系窗口、上车确认、完成、退出和车主管理。
- **API 服务市场**：服务发布、审核、上下架、订单、付款确认和买卖双方履约状态跟踪。
- **求车需求**：发布、审核、公开展示、关闭与重新开放需求。
- **官网价格**：维护和展示公开可验证的官方价格参考记录。
- **社区信誉**：公开资料、收藏、评价、举报、纠纷与申诉。
- **通知中心**：公告、业务通知、未读状态和邮件提醒。
- **运营后台**：用户、商品目录、车源、API 服务、订单、公告、反馈和审计管理。
- **统一搜索**：检索公开车源、API 服务、需求、价格记录和用户资料。

## 技术栈

| 层级 | 技术 |
| --- | --- |
| 前端 | Nuxt 4、Vue 3、TypeScript、Pinia、TanStack Query、Tailwind CSS |
| 后端 | Go 1.26、chi、pgx |
| 数据库 | PostgreSQL 18、版本化 SQL migrations |
| 基础设施 | Docker Compose、Cloudflare Workers、VPS/Caddy、GHCR、GitHub Actions |
| 集成 | linux.do OAuth 2.0、阿里云 DirectMail SMTP、可选 Umami |

## 项目结构

```text
.
├── frontend/              Nuxt 4 混合渲染应用
├── backend/               Go HTTP API
│   ├── cmd/api/           服务入口
│   ├── internal/          业务模块与基础设施
│   └── migrations/        PostgreSQL migrations
├── docs/openapi/          OpenAPI 契约
├── docs/ops/              部署与运维文档
├── scripts/               契约检查与 smoke 脚本
├── compose.yaml           本地开发服务
└── compose.prod.yaml      生产部署覆盖配置
```

## 快速开始

### 环境要求

- Docker 和 Docker Compose
- Node.js `>=24.11 <25`
- pnpm `>=10 <11`
- Go 1.26（仅在不使用 Docker 运行后端时需要）

### 1. 获取代码与配置

```bash
git clone https://github.com/xiangrikuil/c2cmarket.git
cd c2cmarket
cp .env.example .env
```

`.env.example` 仅包含本地开发默认值。不要把真实凭据提交到仓库。

### 2. 启动数据库并执行迁移

```bash
docker compose up -d postgres
docker compose --profile migrate run --rm migrate
```

### 3. 启动后端

```bash
docker compose --profile app up -d --build backend
```

后端默认监听 `http://127.0.0.1:8080`：

```text
GET /health
GET /readyz
```

### 4. 启动前端

```bash
pnpm --dir frontend install --frozen-lockfile
pnpm --dir frontend dev
```

打开 `http://127.0.0.1:3000`。Nuxt 开发服务器通过运行时配置访问本地后端。

停止本地服务：

```bash
docker compose --profile app down
```

## 本地验证

提交 Pull Request 前请运行：

```bash
cd backend && go test ./...
cd ..
pnpm --dir frontend typecheck
NUXT_PUBLIC_API_MODE=real \
NUXT_PUBLIC_SITE_URL=https://c2cmarket.shop \
NUXT_PUBLIC_API_BASE_URL=https://api.c2cmarket.shop \
NUXT_API_BASE_URL=https://api.c2cmarket.shop \
pnpm --dir frontend build
pnpm --dir frontend test
node scripts/check-openapi-routes.mjs
node scripts/check-migrations-doc.mjs
```

前端生产构建必须同时配置 real 模式、公开 API 地址和服务端 API 地址；缺少任一项都会失败。

需要验证完整业务流程时，可在后端运行后执行：

```bash
API_BASE_URL=http://127.0.0.1:8080 node scripts/run-smokes.mjs
```

## 配置与部署

- 本地配置模板：[`.env.example`](./.env.example)
- 生产配置模板：[`.env.production.example`](./.env.production.example)
- Staging 配置模板：[`.env.staging.example`](./.env.staging.example)
- API 契约：[`docs/openapi/c2c-market-api-v1.yaml`](./docs/openapi/c2c-market-api-v1.yaml)
- 部署手册：[`docs/ops/deployment-runbook.md`](./docs/ops/deployment-runbook.md)
- Workers/VPS 部署说明：[`docs/ops/cloudflare-workers-vps-backends.md`](./docs/ops/cloudflare-workers-vps-backends.md)

生产环境必须使用真实 OAuth、独立的加密密钥、HTTPS 前端来源、PostgreSQL 和有效 SMTP 配置。请勿直接复用示例文件中的本地默认值。

## 产品边界与免责声明

C2CMarket 不是支付、托管、账号托管、履约担保或 API 代理平台。平台不应保存或传递第三方账号密码、Cookie、Session、验证码、恢复码或面板主账号凭据。

第三方订阅的费用分摊、成员邀请和使用方式可能受到对应服务提供商条款限制，并可能带来账号限制、服务中断、隐私暴露或费用损失。项目与 linux.do、OpenAI 及其他第三方服务提供商不存在官方隶属、授权或担保关系。使用者应自行核对适用条款并承担相关风险。

## 参与贡献

欢迎提交 Issue 和 Pull Request。开始前请阅读 [贡献指南](./CONTRIBUTING.md)，并尽量让每个变更保持范围清晰、可独立验证。

## 许可证

本项目基于 [MIT License](./LICENSE) 开源。
