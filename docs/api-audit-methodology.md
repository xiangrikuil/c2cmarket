# AI API Model Audit Methodology

日期：2026-07-07
执行者：Codex

## Scope

模型一致性审计用于评估第三方 OpenAI-compatible API 渠道与其声称模型、可信基线之间的统计一致性风险。它只输出风险信号，不提供法律、密码学或绝对证明。

该模块不改变 C2CMarket 的产品边界：平台不代理 API 流量，不托管或交付买卖双方凭据，不处理支付或担保履约，也不会因为一次审计结果自动处罚商户。

## Risk Levels

- `consistent`: 本次样本和可用探针与可信基线没有明显偏离。
- `suspicious`: 存在可观测漂移，需要扩大样本、复跑 strict 模式或人工复核。
- `high_risk`: 多个高置信探针或高贡献证据显示显著偏离，需要暂停高优先级使用并人工复核。
- `insufficient_data`: 样本量、基线或可用探针不足，不能给出可靠风险判断。
- `not_applicable`: 单个探针不适用于当前渠道，例如供应商不支持 logprobs。

## Probe Families

### Random Fingerprint

随机指纹探针使用系统生成的 canary prompt 采样随机数字、类别或二进制序列，并比较目标分布与可信基线分布。核心指标包括 Jensen-Shannon distance、total variation、cosine distance、entropy、invalid rate 和样本量。

单次 quick 审计可以先给出低成本信号，但随机指纹需要足够样本才可靠。低样本运行应返回 `insufficient_data`，不能把一次输出当成模型结论。

### Billing, Latency, Protocol

协议探针检查 `/models` 可用性、返回 model 字段、usage 计数、延迟分位数、错误率和超时行为。它主要发现代理链路、异常供应商行为或 usage gap，不直接证明模型来源。

### Active Fingerprint

主动指纹探针参考 LLMmap 思路，用格式、数值、拒答、代码风格、n-gram 等特征与基线做距离比较。当前实现保留 prompt bank、特征结构和报告入口；没有足够基线时返回 `insufficient_data`。

### KBF

Knowledge Boundary Fingerprint 使用模型知识边界附近的问题，比较目标答案对声称模型和竞争模型的似然。证据包含 claimed match rate、log-likelihood ratio 和 mixed-routing hint。

KBF 需要稳定问题集、基线计数和校准样本。新模型发布或官方模型更新后，应重建基线。

### Model Equality Testing

模型等价性检验使用字符 n-gram kernel MMD 和 permutation p-value 比较两组输出分布。它适合 strict 或离线复核，不适合把低样本 p-value 当成单独决策依据。

### Logprob And Drift

Log probability tracking 只在渠道支持 logprobs 时运行；否则返回 `not_applicable`。边界输入变化检测和 scheduled 巡检用于发现持续漂移。

## Data Handling

目标 API Key 使用后端 secret codec 加密存储，接口创建或更新后不返回明文。

默认只保存系统生成 canary prompt 的 hash、结构化参数、解析值、延迟、usage、错误码和探针证据。真实用户 prompt 或 response 默认不进入模型审计样本表。只有管理员显式开启 `storePromptText` 或 `storeResponseText` 时，系统生成样本文本才会被保存。

报告 JSON 和 Markdown 必须保留 caveats，说明黑盒统计审计的限制。

## Operating Guidance

- quick: 用于低成本初筛，结果常见为低置信或数据不足。
- standard: 纳入主动指纹和 KBF 入口，适合日常人工复核。
- strict: 纳入模型等价性、logprob 和边界变化探针，适合争议或高价值渠道复核。
- scheduled: 低成本周期性巡检，记录最近风险和运行元数据。

建议在以下事件后重建可信基线：

- 官方模型版本或推理策略更新。
- canary prompt bank 或探针版本变更。
- 供应商参数、路由、base URL 或服务层变更。
- 连续巡检出现显著漂移。
