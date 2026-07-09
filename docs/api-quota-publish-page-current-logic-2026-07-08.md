# API 额度发布页当前逻辑说明

日期：2026-07-08  
执行者：Codex  
范围：`/api-market/new` 的“发布 API 额度”页面，以及发布后进入公开 API 市集的关键条件。

## 1. 页面入口与主要源码

- 路由入口：`frontend/src/router.ts`
  - `/api-market/new`
  - route name：`api-new`
  - component：`ApiServicePublishPage.vue`
- 页面主容器：`frontend/src/pages/ApiServicePublishPage.vue`
- 发布页组件：
  - `frontend/src/components/api-service-publish/PriceInventorySection.vue`
  - `frontend/src/components/api-service-publish/AccountPaymentSummarySection.vue`
  - `frontend/src/components/api-service-publish/ApiAccessSourceSection.vue`
  - `frontend/src/components/api-service-publish/ProviderCategorySelector.vue`
  - `frontend/src/components/api-service-publish/ModelMultiSelect.vue`
  - `frontend/src/components/api-service-publish/MerchantNoteSection.vue`
  - `frontend/src/components/api-service-publish/ApiServicePublishPreview.vue`
- 发布表单类型与默认规则：
  - `frontend/src/components/api-service-publish/types.ts`
  - `frontend/src/components/api-service-publish/utils.ts`
- 提交入口：
  - 页面调用 `submitApiService()`，定义在 `frontend/src/lib/api.ts`
  - 真实后端适配在 `frontend/src/lib/apiMarketBackend.ts`
- 后端 API 服务契约：
  - `backend/internal/server/api_market_handler.go`
  - `backend/internal/module/apimarket/model.go`
  - `backend/internal/module/apimarket/service.go`
  - `backend/internal/store/postgres/api_market.go`

## 2. 当前页面的用户流程

页面是一个单页发布表单，左侧填写发布信息，右侧实时展示“买家预览”，底部 sticky 区域处理展示身份和发布按钮。

当前默认心智模型：

1. 商户声明自己有可售 API 美元额度。
2. 商户填写每 `$1` 美元额度售价、可售额度上限、额度有效期。
3. 收款方式不在本页编辑，直接读取“我的中心”的 API 收款设置。
4. 商户选择 API 接入类型、接入方式、可选 linux.do 原帖链接。
5. 商户选择模型大类和具体模型。
6. 商户填写备注。
7. 发布成功后，前端尝试让服务直接进入公开可接单状态。

## 3. 表单状态与默认值

表单状态定义在 `ApiServicePublishPage.vue` 的 `form`，类型是 `ApiServicePublishForm`。

关键默认值：

| 字段 | 当前默认值 | 说明 |
| --- | --- | --- |
| `merchantIdentityMode` | `store_alias` | 默认不公开社区身份，只展示商家名。 |
| `merchantDisplayName` | `小葵 API` | 默认商家展示名。 |
| `distributionSystem` | `sub2api` | 默认 Sub2API。 |
| `deliveryModes` | `['api_key_endpoint']` | 默认只声明 API 请求地址接入说明。 |
| `sourceUrl` | 空字符串 | linux.do 原帖链接可选。 |
| `providerCategory` | `gpt` | 默认 GPT / OpenAI 类。 |
| `billingMode` | `metered_credit` | 前端表单值；提交到后端会变成 `metered_usd_quota`。 |
| `cnyPerUsdCredit` | `0.8` | 每 `$1` 美元额度售价，单位人民币。 |
| `availableCreditUsd` | `500` | 商户声明的可售美元额度上限。 |
| `quotaExpiresAt` | `defaultQuotaExpiresAtInput()` | 默认生成一个北京时间 datetime-local 值。 |
| `minimumPurchaseCny` | `20` | 简化规则固定最低意向金额。 |
| `maximumPurchaseCny` | `300` | 简化规则固定单笔最高意向金额。 |
| `paymentWindowMinutes` | `10` | 固定付款确认窗口。 |
| `usageVisibility` | `merchant_confirmed` | 用量由商户说明，买家自行核对。 |
| `warranty.mode` | `no_warranty` | 默认不作承诺。 |

`applySimplifiedApiQuotaDefaults(form)` 会在初始化、校验和提交前调用。它会强制同步这些隐藏默认值：

- `billingMode = 'metered_credit'`
- `usageVisibility = 'merchant_confirmed'`
- `defaultMultiplier = 1`
- `minimumPurchaseCny = 20`
- `maximumPurchaseCny = 300`
- `validity = { mode: 'days', days: 30, startsAt: 'delivered_at' }`
- `manualBillingNote = ''`
- `packages = []`
- `imageCapability` 关闭
- `warranty` 重置为 `no_warranty`

因此，当前页面虽然类型里保留了套餐、手工核对、生图能力、售后承诺等字段，但发布页现状是“简化 API 额度发布”，这些能力不作为真实可配置项暴露。

## 4. 页面各区块逻辑

### 4.1 出售额度

组件：`PriceInventorySection.vue`

用户可编辑：

- `cnyPerUsdCredit`：每 `$1` 美元额度售价。
- `availableCreditUsd`：商户声明可出售的美元额度上限。
- `quotaExpiresAt`：额度有效至时间。

派生展示：

- `¥20` 意向约可购额度：`20 / cnyPerUsdCredit`
- `¥50` 意向约可购额度：`50 / cnyPerUsdCredit`
- 模型倍率固定显示 `1.00x`

页面文案明确：可售额度是商户声明上限，不是平台余额。

### 4.2 收款与接单

组件：`AccountPaymentSummarySection.vue`

数据来源：

- `useApiPaymentAccountSettingsQuery()`
- query key 来自 `apiPaymentAccountSettingsQueryKey()`
- 设置结构来自 `frontend/src/lib/apiPaymentSettings.ts`

当前规则：

- 本页不直接编辑收款方式。
- 读取“我的中心”的 API 收款设置。
- `watch(accountPaymentSettingsValue)` 会把账号收款设置复制进发布表单：
  - `form.paymentWindowMinutes`
  - `form.paymentOptions`
- 发布时再把这些收款方式作为本服务的接单快照提交。
- 支持的收款方式只有：
  - `wechat`
  - `alipay`
- 当前前端要求付款窗口固定为 `10` 分钟。
- 微信/支付宝启用后要求有收款码数据；提交到真实后端时，二维码本身不会进 `paymentOptions` 请求，只会补默认收款说明。

发布按钮和阻塞文案会优先检查这一块。如果账号收款设置不完整，按钮显示“先配置账号收款”或“先配置收款方式”。

### 4.3 接入与来源

组件：`ApiAccessSourceSection.vue`

用户可选接入类型：

- `sub2api`
- `other`

类型文件里还有 `new_api_proxy`，但当前发布页选项只暴露 `sub2api` 和 `other`。

接入类型联动：

- 选 `sub2api`：
  - `billingMode` 被设为 `metered_credit`
  - `usageVisibility` 被设为 `merchant_confirmed`
  - `defaultMultiplier` 固定为 `1`
  - 如果说明为空或仍是“其他 API”说明，则填入 `Sub2API 标准美元额度，接入细节由双方站外确认。`
  - 如果没有接入方式，则默认 `api_key_endpoint`
- 选 `other`：
  - `distributionSystemNote` 默认补为 `其他 API 接入，额度与用量由商户站外说明。`
  - 自动移除 `sub2api_panel_account`
  - 至少保留 `api_key_endpoint`

接入方式：

- `api_key_endpoint`：API 请求地址接入说明。
- `sub2api_panel_account`：Sub2API 面板接入说明。

`sub2api_panel_account` 只在 `distributionSystem === 'sub2api'` 时可见且可选；其他 API 接入不能声明 Sub2API 面板接入。

linux.do 原帖链接：

- 字段：`sourceUrl`
- 可选，留空不影响发布。
- 非空时前端要求 `https://linux.do/t/*`。
- 前端还会用敏感内容检查，避免把 key/token/session/cookie 等内容放进链接。
- 真实后端也新增 `api_services.source_url` 字段保存该值，并返回给公开列表和详情。

### 4.4 模型大类与具体模型

组件：

- `ProviderCategorySelector.vue`
- `ModelMultiSelect.vue`

数据来源：

- `useModelCatalog()`
- query key：`['model-catalog', 'active']`
- API：`getModelCatalog`

模型大类：

- `gpt`：OpenAI / GPT
- `claude`：Anthropic / Claude
- `other`：其他

映射函数：

- `openai -> gpt`
- `anthropic -> claude`
- 其他 provider -> `other`

当前逻辑：

- 页面只展示当前大类对应的模型。
- 初始默认选择 `gpt-5-mini`，但实际会被模型目录 watch 校正。
- 当模型目录加载后，如果当前已选模型和大类兼容，则保留。
- 如果没有兼容模型，则自动选择当前大类的第一个模型。
- 切换模型大类时，如果已有不兼容模型，会弹确认框；确认后清空不兼容选择。
- GPT 与 Claude 必须分开发布，不能放在同一个 API 服务中。

### 4.5 备注信息

组件：`MerchantNoteSection.vue`

字段：`merchantNote`

当前默认模板包括：

- 接入方式：提交意向后站外确认接入细节。
- 用量核对：用量由商户说明，买家自行核对。
- 限速规则。
- 可用时间。
- 售后口径。

限制：

- 必填。
- 最多 `800` 字。
- 敏感内容检查覆盖 `merchantDisplayName`、`merchantNote`、收款说明。

组件还提供若干快捷插入短句，例如“建议首次提交 ¥20 意向测试”“平台不担保、不代赔”等。

### 4.6 展示身份与发布按钮

位置：页面底部 sticky 区。

展示身份：

- 默认 `store_alias`。
- 勾选框含义是“不公开社区身份，仅展示商家展示名”。
- 取消勾选时切到 `public_profile`。
- 但真实后端提交适配器目前会强制传 `merchantIdentityMode: 'store_alias'`，因此前端选择公开个人身份在真实后端路径下不会按原值传递。

商家展示名校验：

- `store_alias` 时必填。
- 长度 `2-20`。
- 不能包含联系方式、链接或 linux.do 用户名。
- 不能包含“官方、担保、兜底、认证、跑路、实名”等误导词。

发布按钮：

- `canSubmit` 基于 `completeness` 全部为 `done` 才可用。
- 点击发布时再执行 `validate(true)`。
- 校验失败时 toast 第一条错误。
- 校验通过后调用 `publishMutation.mutate()`。

## 5. 校验规则汇总

最终提交校验函数：`validate(true)`。

主要前端校验：

- 展示身份必须是 `public_profile` 或 `store_alias`。
- `store_alias` 下商家展示名必填且符合长度、内容限制。
- 接入类型必须是 `sub2api` 或 `other`。
- `other` 必须填写接入系统说明。
- 至少选择一种接入方式。
- 非 Sub2API 不能选择 `sub2api_panel_account`。
- linux.do 原帖链接留空合法，非空必须是 `https://linux.do/t/*`。
- 模型大类必选。
- `cnyPerUsdCredit` 必须在 `0.01` 到 `100` 之间。
- `availableCreditUsd` 必须大于 `0`。
- `quotaExpiresAt` 必须能转成 ISO 时间，且晚于当前时间。
- 至少选择一个模型。
- 已选模型必须存在于当前后端模型目录。
- 已选模型必须全部属于当前模型大类。
- 付款窗口必须是 `10` 分钟。
- 至少启用一种收款方式。
- 启用的收款方式必须完整。
- `merchantNote` 必填且不超过 `800` 字。
- 展示名、备注、收款说明不能包含敏感凭据类内容。

注意：`validate(requireComplete)` 里有一段 `requireComplete === false` 的宽松逻辑，但当前页面只在发布时调用 `validate(true)`，所以宽松分支现状没有实际入口。

## 6. 提交流程

页面提交 payload：

```ts
submitApiService({
  ...form,
  generatedTitle: generatedTitle(form, catalogById.value),
  status: 'reviewing',
})
```

标题生成：

- `sub2api`：`{模型大类} · API 美元额度`
- 其他 API 接入：`{模型大类} · 其他 API 接入 手工核对额度`

提交成功后：

- 保存返回的 `id` 到 `submittedId`
- invalidates：
  - `['api-services']`
  - `['api-market']`
  - `['home-market']`
  - `['admin-section']`
  - `['notifications']`
- 记录 analytics：`api_service_publish_success`
- toast：`API 服务已发布并开启接单，已进入公开服务列表。`
- 页面显示“查看服务详情”入口。

## 7. 真实后端适配逻辑

真实后端路径在 `frontend/src/lib/apiMarketBackend.ts` 的 `backendSubmitAPIService()`。

流程：

1. `ensureBackendSession('merchant', false)`
2. `ensureMerchantProfile(payload)`
3. 如果没有 `ownerContactMethodId`，创建一个默认 linux.do 私信联系方式。
4. `POST /api/v1/owner/api-services` 创建服务。
5. 如果 `payload.status === 'reviewing'`：
   - `POST /api/v1/owner/api-services/{id}/submit-review`
   - `POST /api/v1/owner/api-services/{id}/publish`
   - `PATCH /api/v1/owner/api-services/{id}/order-settings`
6. 返回 `mapBackendAPIService(response)` 后的前端模型。

字段转换重点：

| 前端字段 | 后端请求字段 | 说明 |
| --- | --- | --- |
| `merchantDisplayName` | merchant profile display name | 缺商户资料时用它创建 profile。 |
| `merchantIdentityMode` | `merchantIdentityMode` | 适配器目前强制传 `store_alias`。 |
| `generatedTitle` | `title` | 后端服务标题。 |
| `shortDescription` | `shortDescription` | 默认“建议首次小额测试”。 |
| `sourceUrl` | `sourceUrl` | linux.do 原帖链接。 |
| `distributionSystem` | `distributionSystem` | `sub2api` / `new_api_proxy` / `other`。 |
| `billingMode` | `billingMode` | `metered_credit -> metered_usd_quota`。 |
| `cnyPerUsdCredit` | `declaredCnyPerUsdAllowance` | 每 `$1` 美元额度售价。 |
| `availableCreditUsd` | `declaredMaxUsdAllowancePerIntent` | 单次意向美元额度上限。 |
| `quotaExpiresAt` | `quotaExpiresAt` | 转成 ISO/RFC3339。 |
| `minimumPurchaseCny` | `minimumIntentCny` | 默认 `20`。 |
| `maximumPurchaseCny` | `maximumIntentCny` | 默认 `300`。 |
| `usageVisibility` | `usageVisibility` | `merchant_confirmed -> merchant_reported`。 |
| `distributionSystemNote` | `publicAccessNote` | 公开接入说明。 |
| `merchantNote` | `merchantNote` | 商户备注。 |
| `deliveryModes` | `accessModes` | `api_key_endpoint -> buyer_dedicated_sub_key`；`sub2api_panel_account -> buyer_dedicated_panel_subaccount`。 |
| `selectedModels` | `models` | 只提交 enabled 模型。 |
| `paymentOptions` | `paymentOptions` | 作为 order settings 的一部分 PATCH。 |

## 8. 后端服务状态流

创建服务时，后端初始状态：

- `review_status = draft`
- `publication_status = offline`
- `moderation_status = clear`

提交审核：

- `SubmitAPIServiceForReview` 要求当前用户已绑定 linux.do。
- 当前策略会自动批准：
  - `review_status = approved`
  - `publication_status = offline`
  - `approved_at = now`

发布：

- `publish` 要求：
  - `review_status = approved`
  - `publication_status = offline`
  - `moderation_status = clear`
  - 商户联系方式可用
- 成功后：
  - `publication_status = online`

接单设置：

- `UpdateOrderSettings` 写入：
  - `accepting_orders`
  - `payment_window_minutes`
  - `payment_options`
- 后端允许付款窗口 `3-15` 分钟，但前端发布页固定要求 `10` 分钟。
- 后端支持的收款方式只有 `wechat` / `alipay`。

## 9. 公开列表/详情可见条件

公开列表查询使用 `publicAPIServiceOrderablePredicate()`。

API 服务进入公开可接单列表必须同时满足：

- `review_status = approved`
- `publication_status = online`
- `moderation_status = clear`
- `accepting_orders = true`
- `payment_window_minutes BETWEEN 3 AND 15`
- 如果是 `metered_usd_quota`，`quota_expires_at > now()`
- 至少有一种启用的支持收款方式

前端 `mapBackendAPIService()` 会把后端字段映射成详情页和列表使用的 `ApiService`：

- `declaredCnyPerUsdAllowance` -> `creditPerCny = 1 / cnyPerUsd`
- `declaredMaxUsdAllowancePerIntent` -> `balance`
- `quotaExpiresAt` -> `expiresAt` / `onlineExpiresAt`
- `isOrderable` -> `publiclyOrderable`
- `sourceUrl` -> 列表和详情的“原帖已绑定 / 待补充原帖”

因此，即使页面 toast 说“已进入公开服务列表”，如果接单设置 PATCH 失败、额度过期、收款方式不完整或状态流中断，公开列表仍可能看不到该服务。

## 10. Mock/local 路径差异

如果 `shouldUseRealBackend()` 为 false，`submitApiService()` 会走前端 mock store：

- 直接生成 `api-${Date.now()}`。
- `status === 'reviewing'` 时直接把 `state` 设为 `online`。
- 只要有完整启用的收款方式，就认为 `publiclyOrderable = true`。
- 保存 `apiServicePaymentSnapshotStore[id]`。
- 不经过真实后端的 linux.do 绑定、版本锁、审核状态迁移、数据库约束。

所以讨论页面改版时，要区分“mock 能跑通”和“真实后端状态流能公开接单”。

## 11. 当前值得讨论的设计点

这些不是修改建议，只是现状里最容易影响改版讨论的逻辑边界：

- 发布页名为“发布 API 额度”，但代码里仍叫 `ApiServicePublishPage`，后端也叫 API service。
- 页面保留了很多类型字段，但 `applySimplifiedApiQuotaDefaults()` 会把套餐、生图、手工核对、售后承诺等能力固定或清空。
- 收款方式只能在“我的中心”配置，本页只是读取并快照，不支持就地编辑。
- 前端允许选择公开个人身份，但真实后端适配器当前强制 `store_alias`。
- `new_api_proxy` 在类型和后端契约中存在，但发布页 UI 当前没有单独暴露。
- 付款窗口后端允许 `3-15` 分钟，发布页固定 `10` 分钟。
- 公开可接单状态取决于后端多条件谓词，不只是发布按钮成功。
- 原帖链接是可选增信，不是发布前置条件。
- GPT / Claude 通过模型大类强制拆分发布。

