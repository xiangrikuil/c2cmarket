# AI API Model Audit Admin Guide

日期：2026-07-07
执行者：Codex

## Entry Points

后台页面：`/admin/model-audit`

后端接口前缀：`/api/v1/admin/model-audit`

所有接口需要已登录管理员身份。写操作需要 CSRF token。

## Target Workflow

1. 创建审计目标，填写名称、OpenAI-compatible base URL、声称模型和 API Key。
2. 创建后列表只显示目标元数据，不返回 API Key 明文。
3. 编辑目标时，API Key 留空表示保持原值；填写新值表示轮换密钥。
4. 停用目标会把 `enabled=false`，历史运行和报告继续保留。

审计目标可选绑定 API 服务或服务模型 ID，用于后续把风险信号回填到商户治理流程。当前实现不做自动下线、自动处罚或交易状态变更。

## Baseline Workflow

可信基线保存模型、来源类型、探针版本、采样参数和结构化特征。来源类型包括：

- `official_api`
- `trusted_api`
- `local_model`
- `manual_import`

基线 JSON 必须是对象。新 prompt bank、官方模型更新或模型参数变化后应创建新基线，而不是覆盖旧基线。

随机指纹基线的 `featureJson` 使用 `randomFingerprint` 字段承载结构化分布。最小可用格式如下：

```json
{
  "randomFingerprint": {
    "categorical": [
      {
        "promptId": "rand_digit_1_10_v1",
        "n": 80,
        "counts": { "7": 72, "4": 8 },
        "values": ["4", "7"],
        "invalidRate": 0
      }
    ]
  }
}
```

## Run Modes

- `quick`: 执行低成本随机指纹和协议探针。
- `standard`: 在 quick 基础上加入主动指纹和 KBF 入口。
- `strict`: 加入模型等价性、logprob tracking 和边界输入变化探针。
- `scheduled`: 用于周期性巡检配置和运行记录。

运行创建在 MVP 中是同步执行。请求返回后，运行应已进入 completed 或 failed/cancelled 等终态。

## Reports

报告包含：

- 总体风险等级、风险分、置信度。
- 每个探针的 risk、score、confidence 和 evidence。
- 建议动作和 caveats。
- Markdown 文本，便于管理员复制到复核记录。

报告中不得把黑盒统计结果写成绝对证明。推荐处理方式是降低渠道优先级、扩大样本、运行 strict 模式、联系商户解释或发起人工复核。

## Scheduled Monitors

巡检配置记录 target、baseline、mode、cron spec、enabled、last run 和 last risk。当前迁移和接口已经持久化巡检元数据；实际 worker 可以复用同一 run service。

## Security Notes

- 不在日志、响应、报告或文档中输出 API Key 明文。
- 默认不保存真实用户 prompt 或 response。
- canary prompt 和 response 文本也默认不保存，只保留 hash 和结构化字段。
- `CONTACT_ENCRYPTION_KEY`、`CONTACT_FINGERPRINT_KEY` 和 `CONTACT_KEY_VERSION` 同时保护联系方式与模型审计目标密钥；生产环境必须使用 32 字节以上的不同强密钥。

## Verification Checklist

- `backend/migrations/000037_model_audit.up.sql` 和 down 文件匹配。
- `/readyz` expected schema version 为 `37`。
- 前端 mock mode 和 real-backend mode 都通过同一 facade。
- 搜索变更，确认没有 API Key 明文、token、session、cookie 或绝对审计结论。
