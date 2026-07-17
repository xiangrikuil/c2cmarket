# Cloudflare Workers with VPS Production and Staging Backends

日期：2026-07-17
执行者：Codex

## 1. 当前拓扑

| 环境 | 前端 | API | VPS loopback | Compose project | 数据库 |
| --- | --- | --- | --- | --- | --- |
| Production | `https://c2cmarket.shop` | `https://api.c2cmarket.shop` | `127.0.0.1:8080` | `c2c-prod` | 独立 production volume |
| Staging | `https://staging.c2cmarket.shop` | `https://api-staging.c2cmarket.shop` | `127.0.0.1:8081` | `c2c-staging` | 独立 staging volume |

两个前端由 Cloudflare Workers Static Assets 托管。两个 API hostname 使用 Cloudflare proxied A 记录指向 VPS `192.236.230.132`。VPS Caddy 自动申请和续期公开受信 TLS 证书，接收 Full (strict) HTTPS 后按 hostname 转发到对应 loopback backend。VPS 不运行 Cloudflare Tunnel。

PostgreSQL 仅存在于各自 Compose network，不发布宿主端口。production override 将 backend host publish 固定为 `127.0.0.1`，公网只能经过 Cloudflare 与 Caddy 到达 API。

## 2. 环境文件

从模板创建两个不进入 Git 的环境文件：

```bash
cp .env.production.example .env.production
cp .env.staging.example .env.staging
```

两套环境必须使用不同的 PostgreSQL 密码、contact encryption/fingerprint key、key version、bootstrap password 与 linux.do OAuth client。可以复用同一 DirectMail SMTP 账号。

关键值：

```dotenv
# production
FRONTEND_ORIGIN=https://c2cmarket.shop
ALLOWED_ORIGINS=https://c2cmarket.shop
OAUTH_REDIRECT_URL=https://api.c2cmarket.shop/api/v1/auth/oauth/callback
BACKEND_PORT=8080
TRUST_X_FORWARDED_FOR=true
TRUSTED_PROXIES=172.16.0.0/12

# staging
FRONTEND_ORIGIN=https://staging.c2cmarket.shop
ALLOWED_ORIGINS=https://staging.c2cmarket.shop
OAUTH_REDIRECT_URL=https://api-staging.c2cmarket.shop/api/v1/auth/oauth/callback
BACKEND_PORT=8081
TRUST_X_FORWARDED_FOR=true
TRUSTED_PROXIES=172.16.0.0/12
MAIL_FROM_NAME=C2CMarket Staging
```

`172.16.0.0/12` 只用于信任 Docker bridge 上由宿主 Caddy 转发的 `X-Forwarded-For`。8080/8081 必须保持 loopback-only，Compose network 不得加入不受信任的 HTTP 客户端容器。

## 3. 启动两套隔离栈

Production：

```bash
docker compose -p c2c-prod --env-file /opt/c2cmarket/shared/.env.production -f compose.yaml -f compose.prod.yaml up -d postgres
docker compose -p c2c-prod --env-file /opt/c2cmarket/shared/.env.production -f compose.yaml -f compose.prod.yaml --profile migrate run --rm migrate
docker compose -p c2c-prod --env-file /opt/c2cmarket/shared/.env.production -f compose.yaml -f compose.prod.yaml --profile app up -d --build backend
```

Staging：

```bash
docker compose -p c2c-staging --env-file /opt/c2cmarket/shared/.env.staging -f compose.yaml -f compose.prod.yaml up -d postgres
docker compose -p c2c-staging --env-file /opt/c2cmarket/shared/.env.staging -f compose.yaml -f compose.prod.yaml --profile migrate run --rm migrate
docker compose -p c2c-staging --env-file /opt/c2cmarket/shared/.env.staging -f compose.yaml -f compose.prod.yaml --profile app up -d --build backend
```

不得省略 `-p`；project name 是容器、网络和 named volume 隔离边界。验证：

```bash
curl -fsS http://127.0.0.1:8080/health
curl -fsS http://127.0.0.1:8080/readyz
curl -fsS http://127.0.0.1:8081/health
curl -fsS http://127.0.0.1:8081/readyz
ss -lntp | grep -E '127\.0\.0\.1:(8080|8081)'
```

## 4. Caddy 与自动 TLS

Caddyfile 配置以下两个 hostname：

```text
api.c2cmarket.shop
api-staging.c2cmarket.shop
```

安装配置并验证。Caddy 会通过 ACME 自动申请公开受信证书并负责续期，无需把 TLS 私钥复制出 VPS：

```bash
install -m 0644 deploy/caddy/Caddyfile.example /etc/caddy/Caddyfile
caddy validate --config /etc/caddy/Caddyfile --adapter caddyfile
systemctl enable --now caddy
```

Cloudflare zone 必须使用 `Full (strict)`。UFW 只允许 Cloudflare 官方 IPv4/IPv6 ranges 访问 80/443；SSH 22 单独管理。Cloudflare IP 清单变化时必须同步更新 Caddy `trusted_proxies` 与 UFW allowlist。

## 5. Cloudflare DNS 与 Access

两个 API hostname 使用：

```text
Type: A
Name: api / api-staging
Content: 192.236.230.132
Proxy status: Proxied
```

不要把前端 hostname 指向 VPS。`c2cmarket.shop` 与 `staging.c2cmarket.shop` 继续归各自 Worker 所有。

Staging 的两个 Access application 保持不变：

1. `staging.c2cmarket.shop/*`
2. `api-staging.c2cmarket.shop/*`

API application 继续 bypass OPTIONS，使浏览器 preflight 到达 Go CORS middleware。Production 不加 Access，除非产品需求明确改变。

## 6. OAuth 与外部依赖

现有 callback 不随源站迁移变化：

```text
https://api.c2cmarket.shop/api/v1/auth/oauth/callback
https://api-staging.c2cmarket.shop/api/v1/auth/oauth/callback
```

保留：

```dotenv
OAUTH_AUTHORIZE_URL=https://connect.linux.do/oauth2/authorize
OAUTH_TOKEN_URL=https://connect.linuxdo.org/oauth2/token
OAUTH_USERINFO_URL=https://connect.linuxdo.org/api/user
```

从 backend Docker network 验证 OAuth token/userinfo 与 DirectMail SMTP 连通性；宿主可连接和 `/readyz` 均不能替代该检查。探测不得输出 client secret、SMTP password、token、Cookie 或原始 userinfo。

## 7. Production R2 备份

`scripts/backup-production-postgres.sh` 生成 custom-format dump 与 SHA-256，并上传 `c2cmarket-r2:c2cmarket-backups/postgres/production/`。R2 lifecycle 保留 30 天；本地成功上传后保留 7 天。

将 rclone 配置放到 `/home/deploy/.config/rclone/rclone.conf`，权限 0600。安装 systemd units：

```bash
install -m 0644 deploy/systemd/c2cmarket-postgres-backup.service.example /etc/systemd/system/c2cmarket-postgres-backup.service
install -m 0644 deploy/systemd/c2cmarket-postgres-backup.timer.example /etc/systemd/system/c2cmarket-postgres-backup.timer
systemd-analyze verify /etc/systemd/system/c2cmarket-postgres-backup.service /etc/systemd/system/c2cmarket-postgres-backup.timer
systemctl daemon-reload
systemctl enable --now c2cmarket-postgres-backup.timer
systemctl start c2cmarket-postgres-backup.service
systemctl status c2cmarket-postgres-backup.service --no-pager
systemctl list-timers c2cmarket-postgres-backup.timer --no-pager
```

首次执行必须同时确认本地 dump、checksum、R2 对象和 `last exit code = 0`。每月至少把一个 R2 dump 恢复到隔离临时数据库，并核对 schema 与关键行数。

## 8. 常规发布顺序

1. 对 production 执行并验证 R2 备份。
2. 在 staging 构建、migration、启动并验证真实 OAuth/邮件与核心流程。
3. 构建 production backend。
4. 运行 production migration；禁止自动执行破坏性 down migration。
5. 启动 production backend。
6. 检查 loopback 与公开 `/health`、`/readyz`，确认 schema version、dirty=false、CORS 与 TLS。

## 9. 排障顺序

1. `curl` 检查 8080/8081 loopback health/readiness。
2. 检查对应 Compose project 的容器与日志。
3. `caddy validate`、`systemctl status caddy` 与 Caddy journal。
4. 检查 Caddy 证书自动续期、Cloudflare Full (strict) 与公开 TLS。
5. 检查 proxied A 记录是否仍指向 `192.236.230.132`。
6. 检查 UFW Cloudflare allowlist。
7. 检查 staging Access/OPTIONS bypass、后端 CORS 与 OAuth 环境归属。
8. 检查 systemd backup、rclone remote、R2 对象与 lifecycle。

Cloudflare `502`/`523`/`525`/`526` 分别从 Caddy upstream、源站可达性与源站 TLS 边界排查，不得先放宽 Go CORS allowlist。
