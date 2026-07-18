# C2CMarket Frontend

中文名：C2C 市集
英文名：C2CMarket

更新日期：2026-07-18
执行者：Codex

定位：AI 官网价格情报、订阅拼车、求车需求和 API 额度撮合平台。

## 技术栈

- Nuxt 4（Nitro + 内置 Vite）
- Vue 3
- TypeScript
- Vue Router（由 Nuxt 管理路由运行时）
- Pinia
- TanStack Query for Vue
- Tailwind CSS v4
- shadcn-vue 风格组件
- Aqua Console tweakcn/shadcn CSS 变量主题

## 运行

```bash
pnpm install --frozen-lockfile
pnpm dev
```

打开：

```text
http://localhost:3000
```

开发模式默认读取 `frontend/.env.development`，使用真实 API 模式，并通过 Nuxt 的开发代理把 `/api`、`/health`、`/readyz` 转发到 `http://127.0.0.1:8080`。如需换后端地址：

```bash
VITE_DEV_API_PROXY_TARGET=http://127.0.0.1:18090 pnpm dev
```

本地初始管理员账号来自 migration `000025_native_admin_login`，用户名为 `admin`。初始密码只在交付记录中提供，不写入仓库文档。

## 渲染与索引策略

- 首页、官网价格、拼车、求车和 API 市集的公开列表/详情使用 SSR + 短时 SWR，首个 HTTP HTML 直接包含业务正文。
- 搜索、登录、发布、账户、商户、管理、兼容跳转、公告和公开用户页保持 CSR，并输出 `noindex`。
- 公开详情不存在或已不可见时，服务端响应返回真实 HTTP 404。
- `/sitemap.xml` 从公开 API 动态发现可索引详情，`/robots.txt` 按生产/staging hostname 控制抓取。

## 真实后端构建与发布

生产构建需要把前端切到真实 API：

```bash
VITE_API_MODE=real \
NUXT_PUBLIC_SITE_URL=https://c2cmarket.shop \
NUXT_PUBLIC_API_BASE_URL=https://api.c2cmarket.shop \
NUXT_API_BASE_URL=https://api.c2cmarket.shop \
pnpm build
```

生产构建会拒绝 `VITE_ENABLE_MOCK=true`，避免显式 mock 开关混入发布产物。

Nuxt/Nitro 使用 `cloudflare_module` preset，生成 Worker 入口 `.output/server/index.mjs` 和静态资源 `.output/public`。从仓库根目录发布：

```bash
pnpm --dir frontend exec wrangler deploy --config ../wrangler.jsonc
# staging 使用 ../wrangler.staging.jsonc
```

不再部署 `dist/`，也不再配置 SPA `index.html` fallback。完整流程见 `../docs/ops/deployment-runbook.md`。

## Umami 埋点

默认关闭。需要统计访问人数、事件和详情页停留时，只配置前端公开 tracker 字段：

```bash
VITE_UMAMI_ENABLED=true \
VITE_UMAMI_SCRIPT_URL=https://umami.example.com/script.js \
VITE_UMAMI_WEBSITE_ID=CHANGE_ME \
VITE_UMAMI_DOMAINS=example.com \
VITE_UMAMI_HOST_URL=https://umami.example.com \
pnpm build
```

不要把 Umami API key、后台账号密码、share URL 或管理台 URL 放进 `VITE_*`。自定义事件只发送低基数字段和分桶，不发送搜索词、URL query、用户 ID、联系方式、举报说明、支付说明、API key、token、session 或 cookie。

## 账号安全设置

公开注册和主登录入口仍是 linux.do OAuth。OAuth 首次创建的站内账号没有默认密码；登录后必须在 `/my/account` 绑定验证邮箱并设置密码，完成后才能进入大部分业务页。密码只用于已绑定 linux.do 用户的站内登录，不是公开密码注册入口。

## 页面

```text
/                         行情首页
/official-prices          官网价格情报
/official-prices/submit   重定向到官网价格列表，不提供用户提交
/admin/official-prices    管理员维护官网价格记录
/carpools                 订阅拼车列表
/carpools/c1              车源详情
/demands                  找车源 / 求车需求
/api-market               API 额度市集
/api-market/a1            API 额度详情
/my                       我的中心
/admin                    管理台
```

## 主题

- 当前公开主题：极致电蓝 `src/theme/minimal-modern.css`
- `src/theme/neumorphic-cool.css` 和 `src/theme/aqua-console.css` 暂时保留样式文件，但不在应用主题入口中展示。
- 可粘贴到 tweakcn/shadcn 的 Aqua Console 变量块：`tweakcn-theme.css`
- Aqua Console 主题 JSON：`tweakcn-theme.json`

## 边界

平台只做信息展示、意向撮合、站外联系和信誉记录，不托管支付、不保存第三方账号密码、不保存 API key/token、不自动交付接口。站内账号密码只通过后端保存不可逆哈希，前端不持久化明文密码。
