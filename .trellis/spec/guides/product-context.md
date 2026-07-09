# C2CMarket Product Context

Date: 2026-06-17
Author: Codex
Source: initial project PRD captured during Trellis bootstrap.
External references:
- OpenAI Terms of Use: `https://openai.com/policies/terms-of-use/`
- OpenAI Services Agreement: `https://openai.com/policies/services-agreement/`
- OpenAI Account Sharing Policy: `https://help.openai.com/en/articles/10471989-openai-account-sharing-policy`

## Positioning

C2CMarket is an AI official price intelligence and community matching platform for linux.do users. It organizes official low-price intelligence, subscription carpool listings, demand posts, API service listings, intent records, fulfillment records, reviews, disputes, and admin review queues.

The product is not a payment platform, escrow platform, account custody service, API proxy, or generalized token delivery system.

## Hard Product Boundaries

The platform must not:

- Process in-platform payments.
- Provide escrow or guaranteed transaction custody.
- Store third-party account passwords, plaintext native account passwords, API keys, Sub2API keys, session tokens, refresh tokens, or unredacted sensitive credentials, except for the API order one-time delivery credential boundary described below.
- Automatically deliver tokens, accounts, or keys.
- Proxy API traffic.
- Present linux.do binding as linux.do official endorsement.
- Encourage or facilitate third-party account credential sharing, API key transfer, or token/key resale.
- Store or auto-deliver third-party passwords, API keys, Sub2API keys, sessions, cookies, refresh tokens, access tokens, MFA codes, recovery codes, or panel owner credentials, except that a seller may submit one buyer-specific, revocable API order delivery credential after payment is confirmed.

Preferred wording:

- `已绑定 linux.do`
- `信任等级3`
- `原帖已绑定`
- `近期确认`

Avoid wording:

- `linux.do 官方认证`
- `GPT token 交易`
- `Token 买卖`
- `自动 token 交付`
- `自动发货`
- `平台担保`
- `主账号密码`

For API quota service UI, prefer:

- `接入方式`
- `站外确认`
- `购买意向`
- `提交购买意向并查看商户联系方式`
- `商户已预先同意接收合规意向`
- `美元额度售价`
- `可售美元额度`
- `意向额度上限`

Rationale: API quota public pages must not imply that C2CMarket delivers, stores, or transfers API keys, account tokens, endpoint secrets, or account credentials before an order exists. When an API service is approved, online, clear, and orderable, the owner has pre-consented to receive compliant purchase intents; a successful API intent creation may immediately disclose the frozen merchant contact to that buyer. After the buyer creates an API order, submits off-platform payment, and the seller confirms receipt, the seller may submit a one-time structured delivery credential for that order. That credential is limited to buyer-specific, revocable API Key + API Base URL or initial login account fields. It is not automatic delivery, escrow, platform verification, API proxying, or a general chat/file-transfer feature. Subscription carpool contact windows remain separate and still use the carpool-specific reservation flow.

API order delivery credential wording:

- Use `交付凭证`, `确认已交付`, `买家专属、可撤销的接入信息`, and `提交后不可修改`.
- Do not use `自动发货`, `平台担保`, `平台验真`, `主账号密码`, `Cookie/Session/Token 交付`, or copy that implies C2CMarket tests the API.
- The credential may be shown only in buyer/seller order detail and action responses. Public API service pages, lists, admin summaries, notifications, events, logs, and reports must not include raw API keys or passwords.
- If a delivered key is wrong, rotated, or needs replacement, buyer and seller handle it through the displayed contact methods off-platform; V1 does not maintain station-internal credential edits or history.

Sub2API quota vocabulary:

- Internal mock fields such as `creditPerCny`, `availableCreditUsd`, `balance`, and historical `purchasedCredit` represent the merchant-declared dollar-denominated quota cap that a buyer may request to purchase.
- User-facing UI must not describe this as platform-issued `Credits`, platform balance, cashback, prepaid value, or anything the platform grants after payment.
- Use copy such as `¥0.80 / $1`, `可售 $500 美元额度`, `本次意向额度上限 $20 美元额度`.
- Model price tables must only show models supported by the current service listing, not the whole platform model catalog.

## Subscription Carpool Product Classification

首页热门套餐行情应作为产品分类入口，而不是具体车源推荐表。分类口径使用 `GPT / Claude / Cursor / Gemini / Perplexity / 其他`，点击分类进入 `/carpools?category=<category>` 后再查看具体车源。

分类页可以展示二级套餐筛选，例如 `ChatGPT Business`、`ChatGPT Plus`、`ChatGPT Pro 5x Web`、`ChatGPT Pro 20x Web`。最新产品决议允许 ChatGPT Plus、ChatGPT Pro、ChatGPT Pro 展示变体和 ChatGPT Business 作为拼车目录项发布，但必须区分“平台允许发布”和“服务提供商规则风险披露”。

目录字段应表达：

- `publish_policy`: `allowed`、`info_only`、`blocked`。`allowed` 表示 C2CMarket 当前开放发布和申请闭环；`info_only` 仅允许行情和线索展示；`blocked` 禁止展示为可操作品类。
- `access_mode`: 例如 `personal_account_cost_share`、`provider_member_invitation`、`owner_managed_access`。
- `provider_policy_status`: 例如 `known_restricted`、`possibly_restricted`。
- `risk_level`: 例如 `high`、`elevated`。
- `risk_ack_required`: 是否要求版本化风险确认。
- `risk_notice_code`: 首个代码为 `openai_subscription_carpool`。
- `policy_version`: 发布策略版本；车源发布、治理和上车申请应记录当时版本。

OpenAI 个人套餐不是官方担保的团队席位，不能被描述为“官方支持共享”或“官方授权拼车”。Plus/Pro 应作为高风险个人订阅费用分摊展示，发布和申请都必须确认当前风险告知版本；Business 可作为 provider member invitation 类方案展示，但仍需要风险提示。任何方案都不得要求或鼓励共享主账号、密码、Session、Cookie、token 或其他登录态。

拼车发布必须把车主声明的倍率与每周/月平均美元额度作为结构化字段收集和展示。拼车发布文案可以继续引导车主在买家须知中说明运营细节，例如中转方式、家宽地区、是否支持 Sub2API 托管管理、是否可用 Web 端。这些运营细节是买家预期说明，不是平台收集凭据。用户界面不得要求或暗示填写直接登录凭据、Sub2API 管理员密码、面板所有者凭据、具体 IP 地址、token、Session、Cookie 或 API Key。优先使用 `Sub2API 托管管理：支持 / 不支持，具体方式站外确认`、`家宽地区：仅填写国家或地区，不填写具体 IP` 等表达。

ChatGPT Plus / Pro 当前初始策略为 `publish_policy=allowed`、`access_mode=personal_account_cost_share`、`provider_policy_status=known_restricted`、`risk_level=high`、`risk_ack_required=true`。`allowed` 只代表 C2CMarket 当前开放该品类，不代表服务提供商认可。平台必须保留通过数据库和管理端把该品类调整为 `info_only` 或 `blocked` 的能力；Go、前端和审核逻辑不得用硬编码的 Plus/Pro 分支替代产品策略字段。

车源详情页必须优先展示价格、名额、接入/分摊安排、车主信息、风险提示和当前可申请状态。若车源处于下架、风险确认缺失、存在共享凭据风险或状态不可申请，禁用原因应优先显示这些产品边界风险，而不是被用户已有申请等个人状态覆盖。

风险告知确认要求：

- 默认不勾选。
- 未确认不能发布车源或申请上车。
- 前端提交所见版本，后端重新校验当前版本。
- 风险告知更新后，新提交和新申请必须确认新版本，历史记录保留旧版本快照。

## Current Implementation Direction

Frontend first:

- Vue 3 + Vite + TypeScript.
- Vue Router.
- Pinia for auth/UI state.
- TanStack Query for mock API wrappers.
- Tailwind CSS and shadcn-vue-style components.
- lucide-vue-next icons.

Backend direction:

- Go HTTP backend.
- Future database likely PostgreSQL.
- For now, frontend should use mock data through API wrapper functions so real APIs can replace the wrapper later.

## Core Modules

- Official low-price intelligence.
- Subscription carpool listings.
- Demand posts.
- API service listings.
- Intent and fulfillment records.
- Reviews.
- User badges and linux.do binding.
- Reports and disputes.
- Notifications.
- Admin review.

## Official Price Intelligence Contract

Official price intelligence records one admin-maintained monthly single-account official opening price. It is not a user-submitted lead workflow, carpool seat price, shared-seat price, bulk quantity price, annual commitment price, or an absolute all-market lowest-price guarantee.

Public official price lists should show only active price records maintained by admins. Legacy lead rows may exist as compatibility/audit carriers, but current user-facing product flows must not expose a submit-low-price-lead entry or promise that users can submit official price records.

Preferred wording:

- `官网价格记录`
- `官网价格维护`
- `折合人民币`
- `已验证参考低价`
- `已验证低价记录`
- `官网公开价`

Avoid wording:

- `官方最低价`
- `官方已验证最低`
- `全网最低`

## Implementation Guidance

When turning this product context into Trellis tasks:

- Keep each task narrow and independently verifiable.
- Treat the full PRD as source context, not a single implementation scope.
- Convert narrative rules into explicit acceptance criteria before coding.
- For v0.1, prefer mock UI shell and route coverage unless the task PRD explicitly includes backend contracts.
- Keep compliance-sensitive boundaries visible in UI copy and form validation.
