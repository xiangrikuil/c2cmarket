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

## 3. 两套隔离栈的手工恢复入口

正常发布必须使用第 9 节的 GitHub/GHCR 流程；VPS 不保留 Git 工作树，也不现场 build。只有排障或回滚已有 release 时才手工调用同一部署脚本。

Production 已有 release：

```bash
SHA=<40-character-git-sha>
/opt/c2cmarket/releases/production/${SHA}/scripts/deploy-vps-backend.sh \
  production \
  ghcr.io/xiangrikuil/c2cmarket-backend:${SHA}
```

Staging 已有 release：

```bash
SHA=<40-character-git-sha>
/opt/c2cmarket/releases/staging/${SHA}/scripts/deploy-vps-backend.sh \
  staging \
  ghcr.io/xiangrikuil/c2cmarket-backend:${SHA}
```

脚本固定使用 `c2c-prod` / `c2c-staging`，project name 是容器、网络和 named volume 隔离边界。验证：

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

## 8. GitHub、GHCR 与 VPS 一次性准备

### 8.1 独立部署身份与目录

GitHub Actions 不得复用个人 `root` 私钥。先通过 VPS console 或已验证的个人 SSH 会话创建独立 `deploy` 用户：

```bash
adduser --disabled-password --gecos '' deploy
usermod -aG docker deploy
install -d -o deploy -g deploy -m 0750 /opt/c2cmarket
install -d -o deploy -g deploy -m 0750 /opt/c2cmarket/shared
install -d -o deploy -g deploy -m 0750 /opt/c2cmarket/releases
install -d -o deploy -g deploy -m 0750 /opt/c2cmarket/releases/production
install -d -o deploy -g deploy -m 0750 /opt/c2cmarket/releases/staging
```

`docker` group 具备等同宿主高权限的容器控制能力；独立用户的目的在于与个人登录密钥隔离、单独撤销和审计，而不是把 Docker 变成低权限操作。

在本地生成专用 key。由于 `deploy` 用户没有密码，首次公钥安装要使用你已经验证过的个人 root key，之后 GitHub 只使用 deploy key：

```bash
ssh-keygen -t ed25519 -f ~/.ssh/c2cmarket_github_actions -C github-actions-c2cmarket
scp -o IdentitiesOnly=yes -i ~/.ssh/id_ed25519 \
  ~/.ssh/c2cmarket_github_actions.pub \
  root@192.236.230.132:/tmp/c2cmarket_github_actions.pub
ssh -o IdentitiesOnly=yes -i ~/.ssh/id_ed25519 root@192.236.230.132
install -d -o deploy -g deploy -m 0700 /home/deploy/.ssh
install -o deploy -g deploy -m 0600 \
  /tmp/c2cmarket_github_actions.pub \
  /home/deploy/.ssh/authorized_keys
rm -f /tmp/c2cmarket_github_actions.pub
exit
ssh -o IdentitiesOnly=yes -i ~/.ssh/c2cmarket_github_actions deploy@192.236.230.132
```

在本地把两套真实 env 临时传给 `deploy`，再安装到共享目录；不能提交 Git：

```bash
scp -o IdentitiesOnly=yes -i ~/.ssh/c2cmarket_github_actions \
  .env.production .env.staging \
  deploy@192.236.230.132:/tmp/
ssh -o IdentitiesOnly=yes -i ~/.ssh/c2cmarket_github_actions deploy@192.236.230.132
install -m 0600 /tmp/.env.production /opt/c2cmarket/shared/.env.production
install -m 0600 /tmp/.env.staging /opt/c2cmarket/shared/.env.staging
rm -f /tmp/.env.production /tmp/.env.staging
```

如果 `/opt/c2cmarket/current` 已经是普通目录，启用自动部署前先把它移动为一个版本目录，再创建 symlink；installer 会拒绝覆盖普通目录。生产备份 systemd 继续通过 `/opt/c2cmarket/current` 读取当前成功发布包。

### 8.2 GHCR 只读登录

GitHub Actions 使用仓库 `GITHUB_TOKEN` 发布 private package `ghcr.io/xiangrikuil/c2cmarket-backend`。VPS 的 `deploy` 用户创建一个 classic PAT，只授予 `read:packages`，然后在 `deploy` 用户的 SSH 会话中执行一次：

```bash
read -rsp 'GHCR read token: ' GHCR_READ_TOKEN
printf '%s' "${GHCR_READ_TOKEN}" | docker login ghcr.io --username xiangrikuil --password-stdin
unset GHCR_READ_TOKEN
```

不得把 token 写进仓库、env 文件或 shell history。确认 `deploy` 用户能运行 `docker info`，并确保 `/home/deploy/.config/rclone/rclone.conf` 已配置且权限为 0600。

### 8.3 GitHub environments

在 GitHub `Settings → Environments` 创建 `staging` 和 `production`。两者配置同名 secrets：

```text
VPS_HOST=192.236.230.132
VPS_USER=deploy
VPS_SSH_PRIVATE_KEY=<c2cmarket_github_actions private key>
VPS_SSH_KNOWN_HOSTS=<verified known_hosts line>
```

`production` 必须配置 required reviewer；`staging` 不配置审批。可以用 `ssh-keyscan -H 192.236.230.132` 生成候选 known-hosts 行，但必须通过 VPS console 上的 `/etc/ssh/ssh_host_ed25519_key.pub` 或服务商控制台独立核对 fingerprint 后再保存为 `VPS_SSH_KNOWN_HOSTS`。workflow 不允许 `StrictHostKeyChecking=no`。

## 9. 自动发布顺序

`.github/workflows/ci.yml` 是唯一测试门禁：所有 PR 运行测试；`staging` / `main` push 在测试成功后调用 reusable `.github/workflows/release-backend.yml`。

1. feature branch 提 PR 到 `staging`，CI 通过后合并。
2. staging push 构建 `ghcr.io/xiangrikuil/c2cmarket-backend:<git-sha>`，上传精简发布包并自动部署 `c2c-staging` / 8081。
3. installer 只在 migration、backend 启动和 `/health`、`/readyz` 全部成功后更新 `/opt/c2cmarket/staging-current`。
4. 完成 staging 真实 OAuth、邮件、CORS 与核心流程验证，再由 `staging` 提 PR 到 `main`。
5. main CI 通过后发布同一 commit SHA 镜像；production environment 等待 reviewer 确认。
6. production deploy 先执行并上传 R2 dump，再 pull 镜像、运行 migration、以 `--no-build` 更新 `c2c-prod` / 8080。
7. 所有检查成功后更新 `/opt/c2cmarket/current`，随后检查公开 TLS、CORS 与真实登录。

release 目录为：

```text
/opt/c2cmarket/releases/staging/<git-sha>
/opt/c2cmarket/releases/production/<git-sha>
```

部署失败会保留上传包和 release 目录用于诊断，但不会切换 current symlink。migration 已成功时不自动执行 down migration。

### 9.1 应用版本回滚

选择上一成功 SHA，在对应 release 目录重新执行部署脚本：

```bash
OLD_SHA=<40-character-git-sha>
/opt/c2cmarket/releases/production/${OLD_SHA}/scripts/deploy-vps-backend.sh \
  production \
  ghcr.io/xiangrikuil/c2cmarket-backend:${OLD_SHA}
ln -sfn /opt/c2cmarket/releases/production/${OLD_SHA} /opt/c2cmarket/current
```

此操作只回滚应用镜像和 current link，不回退数据库 schema。若新 release 已运行 migration，必须先检查 migration 向后兼容性和 R2 备份，禁止自动执行破坏性 down。

## 10. 排障顺序

1. `curl` 检查 8080/8081 loopback health/readiness。
2. 检查对应 Compose project 的容器与日志。
3. `caddy validate`、`systemctl status caddy` 与 Caddy journal。
4. 检查 Caddy 证书自动续期、Cloudflare Full (strict) 与公开 TLS。
5. 检查 proxied A 记录是否仍指向 `192.236.230.132`。
6. 检查 UFW Cloudflare allowlist。
7. 检查 staging Access/OPTIONS bypass、后端 CORS 与 OAuth 环境归属。
8. 检查 GitHub environment 审批/secrets、GHCR package 权限和 `deploy` 用户的 Docker 登录。
9. 检查 `/opt/c2cmarket/releases`、current symlink、systemd backup、rclone remote、R2 对象与 lifecycle。

Cloudflare `502`/`523`/`525`/`526` 分别从 Caddy upstream、源站可达性与源站 TLS 边界排查，不得先放宽 Go CORS allowlist。
