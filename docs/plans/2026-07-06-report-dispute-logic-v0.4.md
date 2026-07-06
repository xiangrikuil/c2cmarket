# 举报 / 人工介入 / 纠纷逻辑 v0.4.1

## Decision

项目当前未上线，不需要生产数据兼容式增量迁移。

本方案按长期可维护目标重整现有 `report` 模块和数据库契约：允许整理旧 migration，让 fresh database 一次建出目标结构；不允许并行保留旧举报系统和新举报系统。

v0.4.1 是实现对齐版本：继续使用现有 `reports`、`dispute_cases`、`appeals` 和 Go `DisputeCase` 模型，不新增平行 `disputes` 表或第二套举报模型。当前错误码统一使用 `ACTIVE_REPORT_EXISTS`；数据库列名使用现有 `reporter_user_id`；HTTP DTO 使用 camelCase，例如 `publicResultCode`。

## Positioning

这是平台的人工介入、纠纷记录、信用记录和公开摘要机制。

它不是退款系统、赔付系统、托管系统、担保系统、站外支付裁决系统、凭据交付系统或 API Key 验真系统。

平台只记录脱敏说明、处理状态、处理结论、公开摘要、必要目标快照和审计记录。第一版不支持附件上传，只支持脱敏文字说明。

平台不接收密码、API Key、token、session、cookie、恢复码、完整付款凭据、付款二维码、银行卡完整号、身份证完整号、截图附件或任何凭据材料。

## Core Objects

`report` 负责收件、校验、分诊、要求补充、拒绝、关闭和升级为正式纠纷。

`dispute_case` 负责正式人工介入、纠纷处理结论、公开结果代码和公开摘要。

`appeal` 负责对纠纷结论或明确的管理限制动作发起申诉。

`moderation_audit_logs` 负责记录管理动作的审计轨迹，包括 report 分诊/要求补充/拒绝/升级/关闭、dispute 要求补充/处理完成/关闭、appeal 通过/拒绝，以及未来账号限制、服务下架、接单限制等独立动作。现有 `dispute_events` 可继续作为案件内部事件流，但不要用它替代跨域审计。

## States

Report:

- `submitted`
- `triaged`
- `needs_info`
- `rejected`
- `dispute_opened`
- `closed`

`report.needs_info` 只表示尚未升级纠纷前，管理员要求举报人补充脱敏说明。

Dispute:

- `open`
- `waiting_info`
- `resolved`
- `closed`

`dispute.waiting_info` 表示正式纠纷已经打开，等待举报人、被处理方或其他参与方补充说明。

Appeal:

- `submitted`
- `approved`
- `rejected`

## Public Result Codes

`dispute_cases.public_result_code` 使用稳定枚举，`public_result` 是管理员填写的公开安全展示文案。

首阶段支持：

- `no_action`
- `contact_invalid`
- `impersonation_confirmed`
- `description_mismatch`
- `rule_or_seat_issue`
- `api_delivery_issue`
- `other_resolved`

暂不把 `account_limited`、`service_removed` 或接单限制类结论放入 dispute 自动结果。此类动作未来必须作为独立 moderation action 执行，并单独审计和申诉。

## Target Types

支持以下目标类型：

- `public_user`
- `contact_snapshot`
- `carpool_application`
- `carpool_membership`
- `api_purchase_intent`
- `api_order`

每条 report 同时保存提交目标和规范化目标：

- `target_type`
- `target_id`
- `canonical_target_type`
- `canonical_target_id`

管理端处理和重复提交判断以 canonical target 为准，提交目标用于追踪用户从哪里发起。

## Single-Mainline Rules

API：

- API intent 尚未生成 order 时，canonical target 为 `api_purchase_intent`。
- API intent 已生成 order 时，canonical target 自动规范化为 `api_order`。

拼车：

- carpool application 尚未成行时，canonical target 为 `carpool_application`。
- carpool application 已生成 membership 时，canonical target 自动规范化为 `carpool_membership`。

后端负责规范化。前端可以从当前页面提交目标，但不能决定最终 canonical target。

## Reason Codes

请求体使用稳定枚举，前端负责展示中文 label：

- `unreachable`
- `contact_invalid`
- `impersonation`
- `description_mismatch`
- `seat_rule_dispute`
- `api_quota_dispute`
- `order_delivery_dispute`
- `other`

## Permissions

所有权限必须由服务端校验。

- `public_user`：登录用户；不能举报自己。
- `contact_snapshot`：必须是对应业务记录参与方，并且当前状态允许查看或历史上曾经向该用户披露过联系方式。
- `carpool_application`：只能由申请人或车主发起。
- `carpool_membership`：只能由该 membership 的买家/成员或车主发起。
- `api_purchase_intent`：只能由该 intent 的买家或商户发起。
- `api_order`：只能由订单买家或商户发起。

前端按钮隐藏只是体验优化，不是权限边界。

## Target Snapshot

创建 report 时冻结 `target_snapshot_json`，避免后续业务状态变化导致管理员无法理解当时上下文。

快照建议包含：

- `targetLabel`
- `submittedTargetType`
- `submittedTargetId`
- `canonicalTargetType`
- `canonicalTargetId`
- `participants`
- `reporterRole`
- `primaryRespondentUserId`
- `primaryRespondentUsername`
- `businessStatus`
- `hasOrder`
- `hasMembership`

`participants` 只保存必要的用户 ID、用户名和业务角色，不保存联系方式、支付信息或凭据材料。

快照禁止包含联系方式明文、付款凭据、API Key、聊天原文、密码、token、cookie、session、截图或附件。

## Duplicate Rule

同一用户 + 同一 canonical target 存在未关闭 report 时，不允许重复创建。

未关闭状态：

- `submitted`
- `triaged`
- `needs_info`
- `dispute_opened`

允许重新创建：

- `rejected`
- `closed`

## Moderation Actions

纠纷结论不能自动触发账号限制、服务下架或接单限制。

正确流程：

1. 管理员处理 dispute，写入 public result code 和 public summary。
2. 如需限制账号、下架服务或限制接单，管理员必须执行独立 moderation action。
3. 独立动作必须写入审计日志，记录 before/after JSON 和内部原因。
4. 如果实现任何限制/下架/接单限制动作，必须同时实现 appeal。

核心原则：纠纷结论不是自动处罚。

## API Shape

优先保持当前路由族：

- `POST /api/v1/reports`
- `GET /api/v1/me/reports`
- `POST /api/v1/me/appeals`
- `GET /api/v1/me/appeals`
- `GET /api/v1/admin/reports`
- `POST /api/v1/admin/reports/{id}/triage`
- `POST /api/v1/admin/reports/{id}/request-info`
- `POST /api/v1/admin/reports/{id}/reject`
- `POST /api/v1/admin/reports/{id}/open-dispute`
- `GET /api/v1/admin/disputes`
- `POST /api/v1/admin/disputes/{id}/request-info`
- `POST /api/v1/admin/disputes/{id}/resolve`
- `POST /api/v1/admin/disputes/{id}/close`

创建 report 的后端流程：

1. 检查登录和 CSRF。
2. 严格 JSON 解码。
3. 校验 reason code。
4. 校验 description 4-1000 字。
5. 拦截敏感内容。
6. 解析目标。
7. 规范化 canonical target。
8. 校验参与方权限。
9. 拒绝自举报。
10. 检查重复 active report。
11. 冻结 target snapshot。
12. 事务写入 report 和事件/审计记录。

重复 active report 必须同时有服务层预检查和数据库部分唯一索引保护。并发命中唯一索引时返回 `409 ACTIVE_REPORT_EXISTS`，不要泄漏数据库 constraint 名称。

## User-Facing Copy

按钮文案：

- 公开主页：`举报用户`
- 联系方式卡片：`举报联系方式`
- 拼车申请详情：`申请人工介入`
- 拼车详情：`申请人工介入`
- API 意向详情：`申请人工介入`
- API 订单详情：`申请人工介入`
- 管理端：`举报工单` / `纠纷案件` / `申诉记录`

提交提示：

```txt
请用脱敏方式描述问题。不要提交密码、API Key、token、session、cookie、恢复码、完整付款凭据、付款截图或其他敏感信息。

平台会根据平台内记录和脱敏说明进行人工处理。平台不承诺追回付款，不代赔，不托管，不裁决站外支付，也不验真 API Key。
```

## Public Display

公开主页只展示管理员写的公开摘要和公开结果，不展示原始举报说明。

可以展示：

- 公开纠纷数量
- 处理结果类型
- 处理时间
- 公开摘要

不能展示：

- 举报人
- 管理员
- 联系方式
- 付款信息
- 内部备注
- 原始说明
- 目标快照完整内容
- 双方详细沟通内容

## First Stage

第一阶段做干净核心闭环：

- clean report/dispute schema
- target types and canonical target fields
- `target_snapshot_json`
- stable reason codes
- sensitive content rejection
- server-side permission resolver
- duplicate active report blocking
- API intent -> order normalization
- carpool application -> membership normalization
- user entry points for carpool/API manual intervention
- admin triage / needs-info / reject / open-dispute / resolve / close
- OpenAPI and frontend API mapper alignment

暂不做限制/下架/接单处罚动作，除非同时实现 appeal。

## Second Stage

- appeal UI and full moderation-action appeal flow
- independent account/service/orderability moderation actions
- public dispute summary expansion
- public dispute statistics
- IP-based rate limiting
- malicious report marking
- duplicate report merge tooling

## Final Principle

report 负责收件和分诊，dispute 负责正式纠纷结论，moderation action 负责实际限制动作，appeal 负责对结论或限制动作申诉。所有敏感信息拒收，所有权限后端校验，所有管理动作留审计。
