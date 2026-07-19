# 贡献指南

感谢你对 C2CMarket 的关注。欢迎通过 Issue 和 Pull Request 改进项目。

## 开始之前

1. 搜索现有 Issue，确认问题或建议尚未被记录。
2. Bug 请使用 Bug 模板，并提供可复现步骤和环境信息。
3. 较大的功能或业务规则调整请先创建功能建议 Issue，说明目标、边界和预期行为。
4. 涉及第三方订阅、付款、凭据或隐私的改动，必须遵守 README 中的产品边界。

## 本地开发

按照 [README](./README.md#快速开始) 启动本地环境。代码修改完成后，至少运行与变更范围相关的检查：

```bash
cd backend && go test ./...
cd ..
pnpm --dir frontend typecheck
VITE_API_MODE=real pnpm --dir frontend build
pnpm --dir frontend test
node scripts/check-openapi-routes.mjs
node scripts/check-migrations-doc.mjs
```

## 分支与提交

- 从最新的目标分支创建短生命周期分支。
- 推荐使用 `feature/<topic>`、`fix/<topic>`、`docs/<topic>` 或 `refactor/<topic>`。
- 提交信息使用清晰的祈使句，推荐 Conventional Commits：`feat:`、`fix:`、`docs:`、`test:`、`refactor:`、`chore:`。
- 不要提交 `.env`、访问令牌、真实账号、构建产物或本地编辑器配置。

## Pull Request 要求

- 一个 PR 只解决一个清晰问题。
- 描述变更动机、主要实现和验证方式。
- 对界面变更提供桌面端和移动端截图。
- 同步更新受影响的 README、OpenAPI、migration 文档或配置模板。
- 保持中文与英文项目简介中的事实一致。
- 确认本地检查通过，并说明未执行检查的原因。

## 代码与产品约定

- 优先复用现有模块、组件和测试模式。
- 不在生产路径中使用 mock 数据掩盖真实错误。
- 不新增站内支付、托管、平台担保或上游 API 代理能力。
- 不保存或传递第三方账号密码、Cookie、Session、验证码、恢复码或面板主账号凭据。
- 用户可见文案不得暗示 linux.do 或其他服务提供商对本项目提供官方认证或担保。

## 行为准则

请保持讨论专业、尊重并聚焦事实。骚扰、歧视、人身攻击、泄露他人隐私或破坏性行为不会被接受。维护者可以关闭不符合社区协作要求的 Issue、讨论或 Pull Request。
