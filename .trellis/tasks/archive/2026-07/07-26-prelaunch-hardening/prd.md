# 上线前安全加固与维护改造

日期：2026-07-26
执行者：Codex

## Goal

在不推倒现有架构、不轮换或泄露任何真实凭据、不破坏当前脏工作区的前提下，完成 C2CMarket 上线前 P0/P1 加固，建立可复现发布和本地可复核质量闸门，并在风险可控时继续 P2 渐进式维护改造。

## User Value

- 防止 OAuth 身份误绑定、账号接管和管理员权限提升。
- 防止模型审计出站请求访问内网、回环、云元数据或通过 DNS/重定向绕过。
- 防止生产后端绕过 Cloudflare/反向代理直接暴露。
- 确保发布包、应用版本、迁移版本、OpenAPI、镜像和固定 Git 提交可追溯。
- 避免验证码、Session、幂等数据、通知和审计数据无限增长或失去有效边界。
- 让限流、联系人密钥版本、数据库连接池、响应头、监控和 CI 满足上线运维要求。

## Confirmed Facts

- 当前分支为 `docs/open-source-readme`，基线提交为 `0f14ad75d9ec7e658d49830533c7c603c7c4d849`。
- 工作区有 233 项未提交变更，覆盖本任务会触及的认证、数据库、服务端、前端和规范文件；本任务不得回退或覆盖其他任务修改。
- `auth_identities` 已有 `UNIQUE(provider, provider_subject)`，但当前 OAuth 存储流程先按 `users.username` upsert，并在身份冲突时更新 `auth_identities.user_id`。
- 当前管理员 Bootstrap 会复用同名用户、赋予管理员权限，并 upsert 覆盖密码凭据。
- 模型审计只做绝对 URL 基础解析，适配器使用默认 `http.Client`；没有 HTTPS-only、地址分类、DNS rebinding 或重定向防护。
- 生产 Compose 已取消 PostgreSQL 端口，但继承开发 Compose 的后端公开端口；基础 Compose 将后端绑定到所有宿主机接口。
- 可信代理功能默认关闭，并且只有来源地址位于显式 `TRUSTED_PROXIES` 时才读取转发头；已有防伪造回归测试。
- 源码打包仍使用 `git ls-files --cached --others --exclude-standard`，会把未跟踪本地文件打入包。
- `/readyz` 已检查 PostgreSQL 和迁移版本 61，但没有应用版本、Git SHA 或构建时间。
- 邮箱验证码摘要使用裸 SHA-256；错误验证码不会原子增加 `attempt_count`；新验证码不会使旧挑战失效。
- 联系人记录已存 key version，但 codec 只有单一 AEAD，解密未按记录版本选 key，也未使用 AAD。
- PostgreSQL 使用 `pgxpool.New` 默认连接池配置，没有任务要求的连接池和数据库会话超时配置。
- 幂等清理只在启动时执行，且只删除过期 `processing` 记录；没有周期 worker。
- 内存限流器有并发锁和过期清理，但没有最大 key 数、固定成本清理、指标或统一 `Retry-After` 输出。
- 安全响应头目前只有 `nosniff`、Referrer-Policy 和生产 HSTS。
- CI 只运行基础 Go 测试、OpenAPI 路由检查、迁移文档检查、前端安装/typecheck/build/test。
- 当前本地后端测试、OpenAPI 路由检查、迁移文档检查和 Git diff whitespace 检查通过。
- 本轮第一次前端基线命令因 login shell 选择 Node 14 而被 Corepack 解析阻断；显式固定 Node `v24.13.0` 后，typecheck 与 48 个测试文件/189 个测试全部通过。

## Requirements

### Scope Order

1. 身份与管理员 Bootstrap。
2. SSRF 与生产入口/可信代理。
3. 可复现打包、构建元数据、迁移和 OpenAPI 契约。
4. 验证码、幂等、Session 与周期清理。
5. 限流、联系人密钥环、数据库连接池、安全响应头和可观测性。
6. CI/CD、安全扫描、部署/安全/运维/备份/发布文档。
7. P2 渐进拆分，仅在 P0/P1 全部通过且不会扩大上线风险时实施。

### Constraints

- 不读取后打印、轮换、删除或替换真实凭据；不修改真实 `.env*` 文件。
- 新配置与测试只能使用明显占位值或测试专用固定假值。
- 不执行 `git reset --hard`、`git clean -fd` 或覆盖用户未提交修改。
- 数据库变化只增加前向迁移，不修改已应用迁移。
- 不引入微服务，不把 PostgreSQL 核心状态迁移到 Redis，不创建第二套认证/订单/存储系统。
- 高风险状态变化必须保留或建立事务边界。
- API 尽量兼容；必要的破坏性变化必须同步 OpenAPI、前端和迁移说明。
- 所有验证在本地自动执行；不以关闭检查、跳过测试或伪造结果换取通过。

## Acceptance Criteria

### P0

- [x] OAuth 身份只由 `(provider, provider_subject)` 决定；既有身份不能在登录中迁移 `user_id`。
- [x] OAuth 首次登录总是创建新本地用户；候选 username 与普通用户、管理员或其他 provider 冲突时生成唯一稳定替代值。
- [x] OAuth 首次登录并发冲突最终只绑定一个本地账号，事务失败不遗留半成品。
- [x] 管理员 Bootstrap 不复用普通/OAuth 用户，不覆盖密码或身份，不隐式提升权限，并可证明幂等来源。
- [x] 模型审计出站请求只允许安全 HTTPS 公网目标，连接时固定验证解析地址，默认拒绝重定向和超大/慢响应。
- [x] 生产 Compose 不公开 PostgreSQL，后端不绑定所有宿主机接口；管理/健康入口不能通过备用端口绕过代理。
- [x] 只有可信代理来源可提供标准化客户端 IP，限流与审计使用同一解析结果。
- [x] 源码包只来自固定 commit/tag 的已跟踪内容，不包含 `.git`、`.env*`、pnpm store、依赖、构建产物或本地文件。
- [x] 应用可暴露非敏感版本、Git SHA、构建时间和目标迁移版本；Docker 镜像绑定固定提交。
- [x] OpenAPI、真实路由和迁移目标版本一致。

### P1

- [x] 验证码使用 HMAC-SHA256 + 配置 pepper；失败次数原子递增、达到上限失效，新挑战替代旧挑战，并发只能成功一次。
- [x] Session、验证码、幂等、联系窗口和适用的通知/事件数据具有明确保留策略与周期清理 worker。
- [x] 幂等 completed/failed/processing 均有有限保留策略，大响应不被无限完整持久化。
- [x] 限流器具有有界 key 数、定期清理、双维度策略、指标、`Retry-After` 和稳定 429 Problem Details。
- [x] 联系人加密按记录 key version 解密，新写入使用当前版本，AES-GCM AAD 绑定记录/字段/版本，并提供 dry-run 批量重加密入口。
- [x] 数据库连接池、连接生命周期、健康检查周期和 PostgreSQL statement/lock/idle-in-transaction 超时可配置且经过校验。
- [x] 生产响应包含 CSP、frame、permissions、content-type、referrer 和 HSTS 头，开发环境策略有明确边界。
- [x] 关键 HTTP、数据库池、OAuth、验证码、限流、SSRF、解密、幂等、后台任务、迁移和 SSE 行为可观测且不记录敏感值。

### CI, Docs, Release

- [x] CI 包含 Go 格式、vet、test、race、漏洞检查，前端冻结安装、typecheck、test、build、audit。
- [x] CI 包含 OpenAPI、迁移、Secret Scan、文件系统/镜像扫描、SBOM、Docker 构建和固定提交发布闸门。
- [x] 临时 PostgreSQL 从空库执行全部迁移并运行 Repository/Integration 验证。
- [x] 部署、安全、运维、备份恢复和发布检查文档只使用占位配置并覆盖上线/回滚。
- [x] 最终交付列出实际修改、迁移、配置、测试结果、剩余风险和有序上线步骤。

### P2

- [x] P0/P1 全部通过后评估后端/前端巨型文件拆分和 OpenAPI 类型生成。
- [x] 只有能保持事务边界、调用契约和测试覆盖时才实施拆分；否则记录为非阻断后续任务，不伪称完成。

## Out of Scope

- 真实凭据轮换、生产数据自动重加密、生产部署执行和人工生产冒烟。
- 无关 UI 改版、框架迁移、微服务拆分、核心状态迁移到 Redis。
- 在 P0/P1 尚未稳定时进行大范围 P2 重写。

## Open Questions

无阻塞产品问题。管理员 Bootstrap 采用独立一次性命令优先方案；如果当前二进制/部署结构使其显著扩大风险，则保留显式开关的启动时模式，但必须通过数据库来源标记实现 fail-closed 幂等。
