# C2CMarket 第一版信誉系统母 PRD 与分阶段实施任务书（Codex 修订版）

> 日期：2026-07-24
>
> 修订者：Codex
>
> 目标项目：C2CMarket（拼车 + API 服务撮合平台）
>
> 文档性质：完整目标母 PRD。必须拆成 Trellis 子任务实施，禁止作为一次性大改直接执行。

## 1. 使用方式

本文件描述第一版信誉系统的完整最终目标，但不代表所有内容必须在一个提交、一个 migration 或一次 Codex 会话中完成。

执行时必须：

1. 创建一个 Trellis 父任务保存完整要求；
2. 按本文第 15 节创建 6 个可独立验证的子任务；
3. 为每个子任务分别编写 `prd.md`、`design.md` 和 `implement.md`；
4. 按依赖顺序逐个激活和实施子任务；
5. 每个子任务完成本地测试、迁移验证和交叉层检查后才能进入下一个；
6. 父任务只负责需求映射、跨子任务验收和最终集成审查，不直接承载大范围实现；
7. 不得为了缩短单个阶段而删除本文最终验收范围。

若真实代码或数据库约束与本文字段名不同，以真实 schema 和项目规范为准，但产品语义和验收结果必须一致。

## 2. 背景与当前事实

C2CMarket 已经具备：

- linux.do OAuth 身份绑定与信任等级同步；
- 拼车申请、成员关系、双方确认完成状态；
- API 订单付款、确认收款、交付和买家确认完成状态；
- 当前拼车场景中的买家对车主评价；
- 举报、纠纷、公开处理结果和申诉；
- `user_restrictions` 基础表；
- 用户公开主页及部分信誉摘要字段。

当前已确认的问题：

- 买家信誉和卖家信誉没有独立、完整的数据闭环；
- API 订单尚未进入评价体系；
- 真实前端适配层存在固定信任等级、完成数、评价数和纠纷数；
- `sourceUrl` 非空被错误等同于“原帖作者已验证”；
- 完成次数、责任取消、响应时间等字段未完全由真实交易聚合；
- 当前纠纷结构没有明确的责任人、责任严重度和可撤销信誉影响；
- 当前限制结构不能完整表达角色、动作和撤销；
- 用户没有统一的信誉成长中心；
- 缓存和派生状态缺少纯时间变化的明确失效机制。

实现前必须重新读取当前代码和最新 migration，不得只依赖本节描述。

## 3. 最终目标

完成全部子任务后，系统必须具备：

- 真实信誉数据聚合；
- 买家信誉与卖家信誉分离；
- `overall`、`carpool`、`api` 三种范围；
- 信誉表现等级、证据充足度和风险状态；
- 拼车与 API 订单的双向已验证评价；
- 不可报复性修改的双盲评价；
- 责任取消、纠纷责任和有效限制；
- 贝叶斯修正评分；
- 少量自动派生徽章；
- 用户自己的信誉成长中心；
- 公共用户、车源、API 服务和交易对手摘要中的统一展示；
- 管理端可追溯的计算依据、排除记录和重算能力；
- 原帖作者资源级验证及用户级聚合；
- 版本化规则、状态历史和时间驱动重算；
- 完整数据库迁移、OpenAPI、测试和说明文档。

## 4. 产品原则

### 4.1 平台边界

C2CMarket 是撮合平台，不是支付托管、交易担保或官方认证平台。

所有信誉页面必须明确：

- 信誉信息基于平台可验证记录，仅供交易决策参考；
- linux.do 绑定不代表 linux.do 官方认证；
- linux.do 信任等级是外部社区身份信号，不是站内履约分；
- “暂无记录”表示证据不足，不等于信誉差；
- 平台不能验证全部站外付款和履约事实。

禁止出现：

- 平台担保；
- 绝对可靠；
- 官方认证；
- 保证安全；
- 保证可用。

### 4.2 风险优先

展示顺序：

```text
硬风险
→ 身份是否可追溯
→ 可验证完成记录
→ 责任取消与纠纷
→ 已验证评价
→ 徽章与等级
```

历史好评、交易数量和高等级不能覆盖当前限制或重大风险。

### 4.3 角色和范围分离

角色：

```text
ReputationRole:
- seller
- buyer
```

范围：

```text
ReputationScope:
- overall
- carpool
- api
```

同一用户每个角色、每个范围独立计算。`overall` 使用两个业务范围的真实计数合并，不得用前端平均或把不同角色混在一起。

### 4.4 表现、证据与风险分离

后端必须同时输出：

- `tier`：历史及当前表现；
- `confidence`：证据充足度；
- `state`：当前风险状态；
- `metrics`：真实指标；
- `warnings`：风险原因；
- `badges`：自动徽章；
- `nextTierProgress`：可主动改善的条件进度；
- `ruleVersion`：规则版本；
- `calculatedAt`：计算时间；
- `nextRecalculationAt`：最晚重新计算时间。

### 4.5 数据真实性

- 所有信誉字段来自数据库真实事实或可重建派生结果；
- 缺失数据使用 `null`、`unknown` 或明确的不可用状态；
- 查询成功并确认真实计数为零时，才允许返回 `0`；
- 禁止用固定值伪装真实统计；
- 禁止前端计算等级、风险状态或阈值；
- 所有页面使用同一个后端信誉服务和规则版本；
- 列表不得产生 N+1 查询。

## 5. 领域枚举

### 5.1 表现等级

```text
ReputationTier:
- insufficient
- normal
- reliable
- high_trust
```

中文：

- `insufficient`：资料不足；
- `normal`：正常；
- `reliable`：较可靠；
- `high_trust`：高可信。

### 5.2 风险状态

```text
ReputationState:
- active
- caution
- restricted
```

显示优先级：

1. `restricted` 显示“受限”；
2. `caution` 显示“谨慎”；
3. `active` 显示真实 `tier`。

### 5.3 证据充足度

第一版按可验证完成数：

- `< 3`：`low`；
- `3～9`：`medium`；
- `>= 10`：`high`。

证据不足不能显示为低信誉。

## 6. 真实指标

### 6.1 可验证完成数

拼车：

- 卖家统计其作为车主的 `completed` 成员关系；
- 买家统计其作为买家的 `completed` 成员关系；
- 只统计真实成员关系终态，不统计车源状态。

API：

- 卖家统计买家最终确认 `completed` 的 API 订单；
- 买家统计自己最终确认 `completed` 的 API 订单；
- `purchase intent`、付款完成和卖家提交交付均不能代替买家确认完成。

共同要求：

- 同一交易只计一次；
- 已取消、进行中、纠纷中但未完成的交易不计完成；
- 被管理员排除信誉统计的交易不计入；
- 历史无法证明终态的记录不补成完成。

### 6.2 交易排除

若现有交易表没有排除能力，增加可审计字段或独立表，至少表达：

```text
- transaction_type
- transaction_id
- excluded_at
- excluded_by_admin_id
- reason_code
- reason
- restored_at
- restored_by_admin_id
- created_at
- updated_at
```

排除交易会同时影响交易双方的相关统计。普通用户不能操作。排除和恢复必须写审计日志，不能改变原业务终态。

### 6.3 责任取消

分别聚合：

- `sellerFaultCancelledCount`
- `buyerFaultCancelledCount`
- `unknownResponsibilityCancelledCount`

责任只能来自明确取消责任字段、取消事件或管理员裁定。

禁止：

- 仅根据 `cancelled` 猜责任；
- 把对方责任计入自己；
- 把未知责任默认给任何一方。

历史无法判断的取消进入 `unknown`，页面显示“部分历史取消无法判断责任”。

### 6.4 完成率和责任取消率

```text
roleControllableTerminalCount
= completedCount + roleFaultCancelledCount

roleCompletionRate
= completedCount / roleControllableTerminalCount

roleFaultCancelRate
= roleFaultCancelledCount / roleControllableTerminalCount
```

分母为 0 返回 `null`。对方责任和未知责任不进入自己的责任率分母。

### 6.5 评价指标

输出：

- `verifiedReviewCount`
- `rawAverageRating`
- `weightedRating`
- `ratingDistribution`
- `recentReviewCount90d`
- 常见正向标签
- 常见负向标签

贝叶斯修正：

```text
weightedRating
= (reviewCount / (reviewCount + 5)) * userAverage
+ (5 / (reviewCount + 5)) * platformAverage
```

`platformAverage`：

- 使用同角色、同范围的已公开、已验证评价；
- 有效评价少于 20 条时使用中性先验 `4.0`；
- 页面同时展示评价数；
- 一条五星不能提高到较可靠或高可信。

### 6.6 响应时间

仅在事件时间戳能够严格定义“请求”和“响应”时启用：

- 使用最近 90 天中位数；
- 样本少于 10 返回 `null`；
- 不使用登录时间、页面加载时间或在线状态代替；
- 响应时间不作为第一版核心等级门槛。

## 7. 纠纷责任与限制

### 7.1 未解决纠纷的对象

当前纠纷必须明确 `subject_user_id`，表示当前被调查或被举报的主体。

- `open`、`waiting_info` 只使 `subject_user_id` 对应角色进入 `caution`；
- 举报人、证人或仅作为交易对手出现的用户不能自动受到负面信誉影响；
- 责任未裁定前不得增加确认责任纠纷数；
- 纠纷详情可以记录双方，但信誉影响必须依据明确主体和最终裁定。

历史数据无法可靠回填 `subject_user_id` 时，标记为 `unknown`，不得猜测。

### 7.2 纠纷信誉裁定

建议新增 `dispute_reputation_outcomes`，一条纠纷可有零条或多条参与方裁定：

```text
- id
- dispute_case_id
- user_id
- role                 # buyer | seller | both
- responsibility       # confirmed_fault | no_fault | shared_fault
- severity             # minor | major | critical
- effect_status        # active | reversed
- public_reason_code
- decided_by_admin_id
- decided_at
- reversed_by_appeal_id
- reversed_by_admin_id
- reversed_at
- reversal_reason
- created_at
- updated_at
```

约束：

- `confirmedFaultDisputeCount365d` 只统计 `effect_status=active` 且责任为 `confirmed_fault` 或 `shared_fault`；
- `confirmedMajorFaultDisputeCount365d` 只统计严重度为 `major` 或 `critical`；
- 严重度不得通过关键词推测；
- 管理员裁定需要稳定枚举、版本控制和审计日志；
- 申诉批准后必须明确反转或替换关联 outcome；
- 申诉批准不能只改 `appeals.status` 而不修正信誉影响。

### 7.3 有效限制

现有 `user_restrictions` 需要能够表达：

```text
- restriction_type
- role_scope           # buyer | seller | all
- action_code          # publish_carpool / apply_carpool / create_order / ...
- reason_code
- public_reason
- starts_at
- ends_at
- revoked_at
- revoked_by_admin_id
- revoke_reason
- created_by_admin_id
- created_at
```

有效条件：

```text
starts_at <= now
AND (ends_at IS NULL OR now < ends_at)
AND revoked_at IS NULL
```

### 7.4 `caution` 与 `restricted`

`caution`：

- 存在未解决纠纷；
- 最近 90 天角色责任取消不少于 3 次且责任取消率大于 20%；
- 当前资源原帖作者为 `mismatch`；
- 其他需要公开提示但未形成限制的风险。

`restricted`：

- 存在影响该角色和动作的有效 `user_restriction`；
- 已确认严重违规并已生成影响该角色和动作的有效 `user_restriction`。

未解决纠纷默认只触发 `caution`，不直接阻止交易。需要阻止动作时，管理员或明确业务规则必须创建可审计的 `user_restriction`。

账号停用、会话失效等身份认证状态由认证和授权模块独立拦截，不得据此推断信誉 `restricted`。若账号风险需要体现在信誉状态和具体业务动作中，必须同步生成明确、可审计的有效 `user_restriction`。

因此第一版不得使用语义含混的：

```text
UNRESOLVED_DISPUTE_BLOCKS_ACTION
```

业务拦截统一使用：

```text
USER_ACTION_RESTRICTED
REPUTATION_ROLE_RESTRICTED
```

错误响应必须包含可公开的 `restrictionType`、`roleScope`、`actionCode` 和处理入口，不泄露内部风控细节。

## 8. 双向评价

### 8.1 支持场景

拼车：

- 买家评价车主；
- 车主评价买家。

API：

- 买家评价卖家；
- 卖家评价买家。

### 8.2 资格

- 只有真实完成交易可评价；
- 评价双方必须是交易参与者；
- 评价角色与交易角色一致；
- 同一交易、同一评价人只有一条当前评价；
- 被排除信誉统计的交易不能产生有效信誉评价；
- 管理员不能伪造普通用户评价。

### 8.3 统一存储

优先新建：

```text
transaction_reviews
transaction_review_revisions
```

`transaction_reviews` 至少包含：

```text
- id
- transaction_type       # carpool_membership | api_order
- carpool_membership_id
- api_order_id
- reviewer_user_id
- reviewee_user_id
- reviewer_role
- reviewee_role
- rating
- tags
- note
- status                 # sealed | published | removed
- review_deadline_at
- visible_at
- frozen_at
- removed_at
- removed_by_admin_id
- removal_reason
- created_at
- updated_at
- version
```

交易外键必须保证二者恰好一个非空。唯一约束保证同一交易、同一评价人一条当前评价。

`transaction_review_revisions` 保存每次公开前修改和管理员处理前后的完整快照。

### 8.4 评价窗口

- 交易完成时间为 `T0`；
- 截止时间为 `T0 + 14 天`；
- 截止前可以提交；
- 评价尚未公开且未冻结时可以修改；
- 截止后不能提交或普通修改。

### 8.5 双盲公开与冻结

必须同时满足防泄露和防报复性修改：

1. 第一方提交后，评价保持 `sealed`；
2. 对方只能知道“对方已提交评价”，不能看到评分、标签和内容；
3. 双方都提交时，在同一事务中设置双方 `visible_at` 和 `frozen_at`，立即公开并永久冻结普通修改；
4. 截止时间到达时，已提交的一方自动视为公开并冻结；
5. 评价一旦公开，普通用户不能再修改评分、标签或内容；
6. 公开后的违法内容只能由管理员执行受审计的移除或脱敏处理，不能悄悄重写原评价；
7. 读取时必须根据双方提交状态和截止时间判断可见性，业务正确性不能依赖定时任务准时执行；
8. 后台任务可以固化到期状态，但只作为性能优化。

旧 `carpool_reviews` 迁移策略：

- 现有公开评价迁移为 `published` 且 `frozen_at` 不为空；
- 保留原创建、修改时间；
- 迁移验证通过前不删除旧表；
- 新旧兼容期只允许一个写入口。

### 8.6 评价输入

- 评分 1～5；
- 最多 5 个平台预设标签；
- 单个标签最多 16 字；
- 说明必填，最多 600 字；
- 禁止联系方式、密码、API Key、Token、Session、Cookie 和恢复码；
- 输出必须转义；
- 管理员移除评价必须有原因和审计。

## 9. 信誉规则

所有阈值集中在后端版本化规则中，例如：

```text
backend/internal/module/reputation/rules.go
ruleVersion = reputation-v1
```

等级计算、下一等级条件、用户文案和管理员依据必须来自同一规则定义。

### 9.1 卖家

资料不足：

```text
completedCount < 3
```

正常：

```text
completedCount >= 3
AND 未达到 reliable
```

较可靠：

```text
completedCount >= 10
sellerCompletionRate >= 95%
sellerFaultCancelRate <= 5%
unresolvedDisputeCount = 0
confirmedMajorFaultDisputeCount365d = 0
state = active
```

高可信：

```text
仍满足 reliable 的全部条件
reliableSince 连续 >= 90 天
completedCount >= 30
completedCount90d >= 3
verifiedReviewCount >= 10
weightedRating >= 4.6
confirmedMajorFaultDisputeCount365d = 0
unresolvedDisputeCount = 0
state = active
```

### 9.2 买家

资料不足：

```text
completedCount < 3
```

正常：

```text
completedCount >= 3
AND 未达到 reliable
```

较可靠：

```text
completedCount >= 10
buyerCompletionRate >= 95%
buyerFaultCancelRate <= 5%
unresolvedDisputeCount = 0
confirmedMajorFaultDisputeCount365d = 0
state = active
```

高可信：

```text
仍满足 reliable 的全部条件
reliableSince 连续 >= 90 天
completedCount >= 30
completedCount90d >= 3
verifiedReviewCount >= 10
weightedRating >= 4.6
confirmedMajorFaultDisputeCount365d = 0
unresolvedDisputeCount = 0
state = active
```

### 9.3 等级升降

- `tier` 每次按当前真实指标重新计算；
- 指标不满足时允许降级；
- `state` 决定当前主展示；
- 只有当前持续满足 `reliable` 的全部条件时才累计 `reliableSince`；
- 任一 `reliable` 条件不再满足，或进入 `caution`、`restricted` 时，将 `reliableSince` 置空并中断连续计时；
- 重新满足 `reliable` 全部条件且恢复 `active` 后，从当前时间重新开始连续时长；
- 没有最近 90 天真实完成记录时不能仅靠等待升级到 `high_trust`；
- 每次等级或状态变化写历史。

### 9.4 评价条件的成长表达

评价是交易对手自愿行为，不能设计成用户可主动刷取的任务。

成长中心可以展示：

```text
已验证评价：7
修正评分：4.7
该证据由真实交易对手自愿形成，不提供索评任务。
```

禁止显示：

```text
还差 3 条评价
还差 3 个五星
邀请对方评价以升级
```

评价门槛在“为什么是这个等级”中透明展示，但对应进度项使用：

```text
status = unavailable
remainingValue = null
actionLabel = null
actionUrl = null
```

不计入“可主动完成项目”的进度分子和分母。

## 10. 原帖作者验证

### 10.1 资源级事实

原帖验证属于具体车源或 API 服务，不属于用户的单一布尔属性。

```text
SourceAuthorVerificationStatus:
- not_submitted
- pending
- verified
- mismatch
- expired
```

建议表：

```text
source_author_verifications
- id
- resource_type
- resource_id
- source_url
- expected_external_user_id
- actual_external_user_id
- status
- verification_method
- verified_by_admin_id
- verified_at
- expires_at
- failure_reason
- created_at
- updated_at
```

只有资源自身为 `verified` 时，资源卡片才能显示“原帖作者已验证”。

若没有自动核验能力：

- 使用管理员审核；
- 旧资源默认 `pending` 或 `not_submitted`；
- 不得继续把 URL 非空显示成已验证。

### 10.2 用户级聚合

用户/卖家范围输出：

```text
SourceAuthorAggregateState:
- not_applicable
- no_sources
- pending
- partial
- verified
- mismatch
```

聚合优先级：

1. 买家角色 → `not_applicable`；
2. 没有要求原帖验证的当前可交易资源 → `no_sources`；
3. 任一适用资源为 `mismatch` → `mismatch`；
4. 所有适用资源均为 `verified` → `verified`；
5. 至少一个适用资源为 `verified`，其余为 `not_submitted`、`pending` 或 `expired` → `partial`；
6. 有适用资源但没有 `verified` 或 `mismatch`，即资源均为 `not_submitted`、`pending` 或 `expired` → `pending`。

聚合层的 `pending` 表示“尚未全部完成验证”，不是把每个资源都改写为资源级 `pending`。用户级摘要必须同时返回各资源级状态计数，不能把未提交、过期或部分验证伪装成全部验证。

## 11. 派生状态、历史和时间失效

### 11.1 状态表

建议 `user_reputation_states`：

```text
- user_id
- role
- scope
- tier
- state
- confidence
- rule_version
- metrics_json
- warnings_json
- badges_json
- tier_entered_at
- reliable_since
- state_entered_at
- dirty_at
- calculated_at
- source_data_updated_at
- next_recalculation_at
```

唯一约束：

```text
(user_id, role, scope)
```

该表是可重建派生缓存，不是真实业务事实。

### 11.2 历史表

`user_reputation_history` 至少记录：

```text
- user_id
- role
- scope
- from_tier
- to_tier
- from_state
- to_state
- rule_version
- reason_snapshot
- created_at
```

### 11.3 写事件失效

以下事件将相关双方状态标记为 dirty 或立即重算：

- 交易完成；
- 取消责任确定；
- 交易排除或恢复；
- 评价提交、公开、移除；
- 纠纷创建、状态变化和责任裁定；
- 申诉批准或驳回；
- 限制创建、撤销；
- linux.do 绑定变化；
- 原帖验证变化。

### 11.4 纯时间变化

每次计算必须求出下一个可能改变结果的最早时间：

```text
nextRecalculationAt = min(
  评价窗口截止时间,
  有效限制结束时间,
  90 天滚动窗口事件离开时间,
  365 天滚动窗口事件离开时间,
  reliableSince + 90 天,
  原帖验证过期时间
)
```

快照有效条件：

```text
dirty_at IS NULL
AND (
  next_recalculation_at IS NULL
  OR now < next_recalculation_at
)
AND rule_version = currentRuleVersion
```

没有下一个时间边界时，`next_recalculation_at` 可以为空。

### 11.5 读取一致性

- 本人、公开详情和列表都必须检查快照有效性；
- 列表一次收集所有相关用户，批量读取和批量重算过期项；
- 禁止逐项调用信誉接口；
- 可用后台任务提前重算，但读取正确性不能依赖后台任务准时运行；
- 批量重算失败时返回明确不可用状态，禁止返回固定假数据；
- 提供管理员全量重建命令并记录结果。

## 12. 公开字段与隐私

### 12.1 不可选择隐藏的最小信誉摘要

只要用户参与公开交易，下列字段是市场完整性所需，不能由用户单独隐藏：

- `tier`
- `state`
- `confidence`
- `ruleVersion`
- `calculatedAt`
- 可验证完成数或与等级一致的完成数档位；
- 角色责任取消率；
- `unknownResponsibilityCancelledCount` 是否大于 0；
- `unresolvedDisputeCount`；
- 生效中的公开限制状态；
- `verifiedReviewCount`；
- `weightedRating`，仅在评价样本满足展示门槛时；
- 原帖作者资源级状态或用户级聚合状态。

### 12.2 仍可由隐私设置控制

- 精确最近活跃时间；
- 响应时间中位数；
- 完整完成记录列表；
- 已处理纠纷的详细公开摘要；
- 非信誉所必需的资料字段。

若已处理重大责任纠纷仍影响等级，公共摘要必须显示“存在影响等级的已处理责任纠纷”及数量，但可以隐藏案件细节。

现有 `showCompleted...` 和 `showResolvedDisputeSummary` 需要迁移说明：它们不能隐藏用于公开等级解释的最小聚合指标，但仍控制详细列表或详细摘要。

## 13. 信誉成长中心

新增：

```text
/me/reputation
```

左侧菜单：

```text
信誉与成长
```

页面包含：

- 买家/卖家角色切换；
- 全部/拼车/API 范围切换；
- 当前等级、风险状态和证据充足度；
- 真实指标；
- 规则版本和计算时间；
- 可主动改善的下一等级条件；
- 不可主动刷取的被动证据；
- 纠纷、限制和申诉处理入口；
- 最近等级变化。

进度状态：

```text
met
not_met
blocked
unavailable
```

责任取消率可以计算至少还需多少笔无责任取消的真实完成交易：

```text
F / (C + F + N) <= R
```

使用向上取整。文案必须说明这是基于当前历史数据的数学条件，不预测最终升级日期。

时间条件只能显示：

> 若其他条件保持满足，最早可在某日满足连续时长条件。

不能显示：

> 你将在某日自动升级。

## 14. API、页面和业务接入

### 14.1 API

语义至少覆盖：

```http
GET /api/v1/users/{username}/reputation?scope=overall|carpool|api
GET /api/v1/me/reputation
GET /api/v1/reputation/rules
GET /api/v1/me/reviews
POST /api/v1/me/transactions/{type}/{id}/review
PUT /api/v1/me/transactions/{type}/{id}/review
```

列表优先在原查询中批量关联信誉摘要。若提供批量接口，必须限制数量、校验权限并防止泄露私密字段。

客户端不能提交：

- tier；
- state；
- confidence；
- badge；
- calculated metrics。

### 14.2 页面

统一接入：

- 用户公开主页；
- 车源列表和详情；
- 车主查看申请人；
- API 服务列表和详情；
- 我的评价；
- 信誉成长中心；
- 管理端用户信誉审计。

所有页面删除固定信誉数据。缺失时显示“暂无数据”或“资料不足”，不能显示假 0。

公共卡片最多显示两个高价值徽章，风险必须先于徽章。

### 14.3 管理端

管理员可以：

- 查看角色和范围状态；
- 查看原始指标、规则版本和历史；
- 查看纠纷裁定、申诉修正和限制；
- 排除或恢复异常交易；
- 审核原帖作者；
- 触发单用户或全量重算。

管理员不能：

- 任意加减信用分；
- 伪造评价；
- 直接修改派生等级而不改变事实；
- 无审计地移除负面记录。

### 14.4 业务拦截

有效限制接入：

- 发布车源；
- 发布 API 服务；
- 开启接单；
- 申请拼车；
- 创建 API 购买意向；
- 创建 API 订单；
- 接单；
- 受限联系方式访问；
- 提交评价。

后端是最终权限边界。未解决纠纷本身只显示 `caution`，除非已产生明确限制。

## 15. Trellis 父子任务

### 父任务

父任务保存本文件、子任务映射和最终验收，不直接实施全部代码。

建议标题：

```text
C2CMarket 第一版信誉系统
```

### 子任务 1：真实数据与假数据清理

建议 slug：

```text
reputation-truthful-data
```

范围：

- 真实用户身份投影；
- 拼车和 API 完成数聚合；
- 固定信任等级、完成数、评价数、纠纷数清理；
- 公共摘要缺失值语义；
- 列表批量查询基础；
- 交易排除基础结构。

退出条件：

- 所有真实页面不再显示固定信誉值；
- 完成数来自真实终态；
- 缺失数据不伪装成 0；
- 查询无 N+1；
- 后端、前端、OpenAPI 和测试通过。

### 子任务 2：责任、纠纷裁定与限制

建议 slug：

```text
reputation-governance
```

依赖：子任务 1。

范围：

- 取消责任结构和历史 unknown；
- `subject_user_id`；
- `dispute_reputation_outcomes`；
- 申诉反转信誉影响；
- 角色/动作限制；
- `caution` 与 `restricted`；
- 真实业务拦截。

退出条件：

- 举报人不会因参与案件被错误处罚；
- 未裁定纠纷不增加确认责任数；
- 申诉批准会修正信誉影响；
- 只有有效限制阻止动作；
- 限制到期和撤销可恢复；
- 审计完整。

### 子任务 3：统一双向评价

建议 slug：

```text
transaction-reviews
```

依赖：子任务 1。

范围：

- `transaction_reviews` 和 revisions；
- 旧拼车评价迁移；
- 拼车/API 双向评价；
- 14 天窗口；
- 双盲公开和公开即冻结；
- 管理员受审计移除；
- 贝叶斯评价基础。

退出条件：

- 非完成交易不能评价；
- 越权评价被拒绝；
- 一方提交时内容完全保密；
- 双方提交或截止时公开并冻结；
- 公开后不能报复性修改；
- 旧评价无损迁移。

### 子任务 4：信誉规则、快照与历史

建议 slug：

```text
reputation-engine
```

依赖：子任务 1、2、3。

范围：

- 角色和范围聚合；
- tier/state/confidence；
- 贝叶斯评分；
- 版本化规则；
- 下一等级进度；
- `user_reputation_states` 和 history；
- `next_recalculation_at`；
- 批量重算和全量重建。

退出条件：

- 规则和进度使用同一 evaluator；
- 指标下降可以降级；
- 风险优先；
- 高可信需要近期完成；
- 纯时间变化按时失效；
- 列表批量读取一致。

### 子任务 5：原帖作者验证

建议 slug：

```text
source-author-verification
```

依赖：子任务 1；在子任务 6 前完成。

范围：

- 资源级验证结构；
- 管理员核验；
- 旧数据状态迁移；
- 资源级展示；
- 用户/角色范围聚合；
- mismatch 风险。

退出条件：

- URL 非空不再等同于已验证；
- 只有资源 `verified` 显示验证徽章；
- 用户聚合能区分 partial 和 mismatch；
- 过期状态会触发重算。

### 子任务 6：公共页面、成长中心与管理审计

建议 slug：

```text
reputation-surfaces
```

依赖：子任务 2、4、5。

范围：

- 公共用户信誉；
- 车源和 API 服务摘要；
- 申请人摘要；
- 我的评价；
- `/me/reputation`；
- 隐私字段迁移；
- 管理端审计；
- 响应式和可访问性。

退出条件：

- 所有页面使用同一规则版本；
- 风险展示优先；
- 最小公开信誉不能被选择性隐藏；
- 成长中心不诱导索评；
- 管理员不能任意改分；
- 桌面、移动端和可访问性检查通过。

### 父任务最终集成门禁

全部子任务完成后，父任务执行：

- 全量 Go 测试和 `go vet`；
- 前端单测、类型检查和构建；
- migration 从空库升级；
- 已有数据库逐版本升级；
- migration down 验证；
- OpenAPI 路由与类型一致性；
- 真实 API smoke；
- 公共页面隐私审查；
- 无固定信誉值搜索；
- 无 N+1 查询验证；
- 跨页面同用户一致性检查。

## 16. 数据库和兼容要求

- 每个 migration 有 up/down；
- 检查现有 constraint、trigger 和 partial index；
- 不破坏订单与拼车状态机；
- 不把 purchase intent 当完成订单；
- 旧评价安全迁移或兼容读取；
- 所有枚举有数据库 CHECK 或受控值；
- migration 失败可回滚；
- 派生状态可完整重建；
- 不保存 OAuth Token、密码、API Key、Session、Cookie 或其他敏感内容。

## 17. 测试总表

每个子任务编写自己的聚焦测试；父任务最终至少证明：

### 17.1 规则

- 0 笔完成 → 资料不足、低证据；
- 1 条五星不能进入较可靠；
- 3 笔完成 → 正常；
- 10 笔完成且满足比率 → 较可靠；
- 10 笔完成加 1 次责任取消不满足 5%；
- 分母为 0 返回 `null`；
- 对方责任和 unknown 不影响自己的责任率；
- 买家和卖家不串；
- carpool、api、overall 不串；
- 未解决纠纷 → caution；
- 有效限制 → restricted；
- 限制不能被历史高等级覆盖；
- 申诉批准修正 outcome；
- 连续 90 天中断后重新计时；
- 无近期完成不能只靠等待进入高可信；
- 贝叶斯计算和先验正确；
- 原帖聚合优先级正确；
- 徽章互斥。

### 17.2 评价

- 真实交易外键；
- 双向唯一约束；
- 评价窗口；
- 双盲可见性；
- 双方公开事务；
- 公开即冻结；
- 公开后普通修改被拒绝；
- revision 审计；
- 旧评价迁移；
- 管理员移除审计；
- 越权和非完成交易评价被拒绝。

### 17.3 时间与缓存

- 评价截止时间触发失效；
- 限制结束触发失效；
- 90/365 天滚动窗口触发失效；
- `reliableSince + 90 天` 触发重算；
- 原帖验证过期触发失效；
- 公共列表批量刷新过期项；
- 规则版本变化使旧快照失效；
- 重算失败不返回固定数据。

### 17.4 隐私与接口

- 最小公开信誉始终可解释；
- 隐私设置继续隐藏允许隐藏的字段；
- 举报人、内部备注和证据不公开；
- 批量接口限制数量；
- 客户端不能提交派生等级；
- OpenAPI 与真实路由一致；
- 所有空值语义一致；
- 同用户跨页面展示一致。

### 17.5 前端

- 资料不足使用中性状态；
- caution/restricted 风险优先；
- 角色和范围切换；
- blocked/unavailable 进度；
- 不显示“还差 N 条评价”；
- 不显示未验证原帖徽章；
- 不显示固定假 0；
- 错误和不可用状态；
- 移动端；
- 键盘操作；
- 状态不只依赖颜色。

## 18. 最终验收

全部 6 个子任务和父任务集成门禁完成后，才可以宣称第一版信誉系统完成：

- [ ] 页面不再出现固定信誉数据；
- [ ] 买家与卖家分别计算；
- [ ] overall/carpool/api 可区分；
- [ ] 只有真实完成终态进入完成统计；
- [ ] 责任取消有明确责任，历史 unknown 不伪造；
- [ ] 纠纷主体、最终责任和严重度可审计；
- [ ] 申诉批准会修正信誉影响；
- [ ] 未解决纠纷默认只 caution；
- [ ] 只有有效限制阻止具体动作；
- [ ] 拼车和 API 均支持双向评价；
- [ ] 评价有 14 天窗口；
- [ ] 双盲评价公开即冻结；
- [ ] 公开后不能报复性修改；
- [ ] 评价修改和移除有历史；
- [ ] 少于 3 笔显示资料不足；
- [ ] 一条五星不能进入高等级；
- [ ] 高可信要求近期真实完成；
- [ ] 评价数量不被设计成可刷成长任务；
- [ ] 原帖验证为资源级真实状态；
- [ ] 用户级原帖聚合不掩盖 partial 或 mismatch；
- [ ] 快照覆盖所有时间驱动失效；
- [ ] 用户可以查看角色和范围信誉；
- [ ] 用户可以理解等级依据和可主动改善项；
- [ ] 最小公开信誉不被选择性隐藏；
- [ ] 风险不能被好评或徽章覆盖；
- [ ] 公共页面不泄露隐私；
- [ ] 列表无 N+1；
- [ ] 管理端可审计但不能任意改分；
- [ ] OpenAPI、迁移、前端类型和测试完整；
- [ ] 文案不出现平台担保、绝对可靠或官方认证。

## 19. 最终交付说明

父任务结束时输出：

1. 子任务及提交映射；
2. migration 清单；
3. API 变更；
4. `reputation-v1` 规则；
5. 固定假数据删除位置；
6. 评价迁移结果；
7. 纠纷责任和限制迁移结果；
8. 原帖验证方式；
9. 成长中心及公共页面；
10. 缓存失效和重算机制；
11. 隐私字段处理；
12. 测试命令和结果；
13. 回滚方式；
14. 尚未实现且明确列入后续版本的内容。

实施顺序始终遵守：

```text
先保证数据是真的
→ 再建立责任和限制
→ 再补齐双向评价
→ 再计算信誉等级
→ 再验证原帖作者
→ 最后接入成长中心和全站页面
```
