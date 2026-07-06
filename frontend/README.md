# C2CMarket Frontend

中文名：C2C 市集
英文名：C2CMarket

定位：AI 官方低价情报、订阅拼车、求车需求和 API 额度撮合平台。

## 技术栈

- Vite 8+
- Vue 3
- TypeScript
- Vue Router
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
http://localhost:5173
```

开发模式默认读取 `frontend/.env.development`，使用真实 API 模式，并通过 Vite proxy 把 `/api`、`/health`、`/readyz` 转发到 `http://127.0.0.1:8080`。如需换后端地址：

```bash
VITE_DEV_API_PROXY_TARGET=http://127.0.0.1:18090 pnpm dev
```

本地初始管理员账号来自 migration `000025_native_admin_login`，用户名为 `admin`。初始密码只在交付记录中提供，不写入仓库文档。

## 真实后端构建

生产构建需要把前端切到真实 API：

```bash
VITE_API_MODE=real VITE_API_BASE_URL=https://CHANGE_ME_DOMAIN pnpm build
```

生产构建会拒绝 `VITE_ENABLE_MOCK=true`，避免显式 mock 开关混入发布产物。

部署 `dist/` 时，静态服务器需要把 SPA 路由回退到 `index.html`，API 请求由 `VITE_API_BASE_URL` 指向 Go 后端。完整部署流程见 `../docs/ops/deployment-runbook.md`。

## 账号恢复设置

公开注册和主登录入口仍是 linux.do OAuth。OAuth 首次创建的站内账号没有默认密码；登录后必须在 `/my/account` 绑定验证邮箱并设置备用密码，完成后才能进入大部分业务页。备用密码只用于已绑定 linux.do 用户恢复访问，不是公开密码注册入口。

## 页面

```text
/                         行情首页
/official-prices          官方低价情报
/official-prices/submit   提交低价线索
/official-prices/manage   官方最低价格管理
/carpools                 订阅拼车列表
/carpools/c1              车源详情
/demands                  找车源 / 求车需求
/api-market               API 额度市集
/api-market/a1            API 额度详情
/my                       我的中心
/admin                    管理台
```

## 主题

- 项目内置主题：`src/theme/aqua-console.css`
- 可粘贴到 tweakcn/shadcn 的 Aqua Console 变量块：`tweakcn-theme.css`
- Aqua Console 主题 JSON：`tweakcn-theme.json`

## 边界

平台只做信息展示、意向撮合、站外联系和信誉记录，不托管支付、不保存第三方账号密码、不保存 API key/token、不自动交付接口。站内账号密码只通过后端保存不可逆哈希，前端不持久化明文密码。
