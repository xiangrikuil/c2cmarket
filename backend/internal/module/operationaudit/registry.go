package operationaudit

import (
	"strings"

	"github.com/google/uuid"
)

type ActionDefinition struct {
	SourceKind    string
	Action        string
	TargetType    string
	Domain        string
	ActionLabel   string
	Outcome       string
	Summary       string
	DetailPattern string
}

// actionRegistry 是统一审计读取器的唯一展示白名单。数据库中新出现的动作不会自动暴露。
var actionRegistry = []ActionDefinition{
	{SourceAdmin, "user.account_status_changed", "user", DomainAccount, "账号状态变更", OutcomeStatusChanged, "管理员变更了账号状态", ""},
	{SourceAdmin, "user.admin_permission_changed", "user", DomainAccount, "管理员权限变更", OutcomeStatusChanged, "管理员变更了账号权限", ""},
	{SourceAdmin, "student_registration.updated", "student_registration_setting", DomainInstitution, "学生注册设置变更", OutcomeStatusChanged, "管理员变更了学生邮箱注册设置", "/admin/student-registration"},
	{SourceAdmin, "student_institution_domain.created", "student_institution_domain", DomainInstitution, "院校域名新增", OutcomeSucceeded, "管理员新增了院校邮箱域名", "/admin/student-registration"},
	{SourceAdmin, "student_institution_domain.updated", "student_institution_domain", DomainInstitution, "院校域名更新", OutcomeStatusChanged, "管理员更新了院校邮箱域名", "/admin/student-registration"},
	{SourceAdmin, "student_institution_domain.enabled", "student_institution_domain", DomainInstitution, "院校域名启用", OutcomeStatusChanged, "管理员启用了院校邮箱域名", "/admin/student-registration"},
	{SourceAdmin, "student_institution_domain.disabled", "student_institution_domain", DomainInstitution, "院校域名停用", OutcomeStatusChanged, "管理员停用了院校邮箱域名", "/admin/student-registration"},
	{SourceAdmin, "official_price_record.create", "official_price_record", DomainAPIService, "官方价格新增", OutcomeSucceeded, "管理员新增了官方价格记录", ""},
	{SourceAdmin, "official_price_record.update", "official_price_record", DomainAPIService, "官方价格更新", OutcomeStatusChanged, "管理员更新了官方价格记录", ""},
	{SourceAdmin, "official_price_record.take_down", "official_price_record", DomainAPIService, "官方价格下架", OutcomeStatusChanged, "管理员下架了官方价格记录", ""},
	{SourceAdmin, "official_price_lead.approve", "official_price_lead", DomainAPIService, "官方价格线索通过", OutcomeStatusChanged, "管理员通过了官方价格线索", ""},
	{SourceAdmin, "api_service.approved", "api_service", DomainAPIService, "API 服务审核通过", OutcomeStatusChanged, "管理员通过了 API 服务审核", ""},
	{SourceAdmin, "api_service.changes_requested", "api_service", DomainAPIService, "API 服务要求修改", OutcomeStatusChanged, "管理员要求修改 API 服务", ""},
	{SourceAdmin, "api_service.rejected", "api_service", DomainAPIService, "API 服务审核拒绝", OutcomeStatusChanged, "管理员拒绝了 API 服务审核", ""},
	{SourceAdmin, "api_service.suspended", "api_service", DomainAPIService, "API 服务暂停", OutcomeStatusChanged, "管理员暂停了 API 服务", ""},
	{SourceAdmin, "api_service.restored", "api_service", DomainAPIService, "API 服务恢复", OutcomeStatusChanged, "管理员恢复了 API 服务", ""},
	{SourceAdmin, "api_service.removed", "api_service", DomainAPIService, "API 服务移除", OutcomeStatusChanged, "管理员移除了 API 服务", ""},
	{SourceAdmin, "api_service_promotion.created", "api_service_promotion", DomainAPIService, "推广创建", OutcomeSucceeded, "管理员创建了 API 服务推广", ""},
	{SourceAdmin, "api_service_promotion.stopped", "api_service_promotion", DomainAPIService, "推广停止", OutcomeStatusChanged, "管理员停止了 API 服务推广", ""},
	{SourceAdmin, "promotion_reward.campaign_updated", "promotion_reward_campaign", DomainAPIService, "推广活动设置变更", OutcomeStatusChanged, "管理员更新了推广活动设置", ""},
	{SourceAdmin, "referral.revoked", "referral_relation", DomainAccount, "邀请关系撤销", OutcomeStatusChanged, "管理员撤销了邀请关系", ""},
	{SourceAdmin, "promotion_coupon.granted", "promotion_coupon", DomainAPIService, "推广券发放", OutcomeSucceeded, "管理员发放了推广券", ""},
	{SourceAdmin, "promotion_coupon.revoked", "promotion_coupon", DomainAPIService, "推广券撤销", OutcomeStatusChanged, "管理员撤销了推广券", ""},

	{SourceModeration, "triage", "report", DomainModeration, "举报分流", OutcomeStatusChanged, "管理员处理了举报分流", ""},
	{SourceModeration, "request_info", "report", DomainModeration, "举报补充信息", OutcomeStatusChanged, "管理员要求补充举报信息", ""},
	{SourceModeration, "reject", "report", DomainModeration, "举报驳回", OutcomeStatusChanged, "管理员驳回了举报", ""},
	{SourceModeration, "open_dispute", "report", DomainModeration, "举报转纠纷", OutcomeStatusChanged, "管理员将举报转为纠纷", ""},
	{SourceModeration, "close", "report", DomainModeration, "举报关闭", OutcomeStatusChanged, "管理员关闭了举报", ""},
	{SourceModeration, "request_info", "dispute_case", DomainModeration, "纠纷补充信息", OutcomeStatusChanged, "管理员要求补充纠纷信息", ""},
	{SourceModeration, "resolve", "dispute_case", DomainModeration, "纠纷裁决", OutcomeStatusChanged, "管理员裁决了纠纷", ""},
	{SourceModeration, "close", "dispute_case", DomainModeration, "纠纷关闭", OutcomeStatusChanged, "管理员关闭了纠纷", ""},
	{SourceModeration, "mark_overdue", "dispute_case", DomainModeration, "补救逾期确认", OutcomeStatusChanged, "管理员确认纠纷补救已逾期", ""},
	{SourceModeration, "approve", "appeal", DomainModeration, "申诉通过", OutcomeStatusChanged, "管理员通过了申诉", ""},
	{SourceModeration, "reject", "appeal", DomainModeration, "申诉驳回", OutcomeStatusChanged, "管理员驳回了申诉", ""},

	{SourceDomain, "user.student_identity_assigned", "user", DomainIdentity, "高校邮箱身份建立", OutcomeSucceeded, "用户完成了高校邮箱身份验证", ""},
	{SourceDomain, "user.linuxdo_linked", "user", DomainIdentity, "Linux.do 身份关联", OutcomeSucceeded, "用户关联了 Linux.do 身份", ""},
	{SourceDomain, "carpool_listing.created", "carpool_listing", DomainCarpool, "拼车车源创建", OutcomeSucceeded, "车主创建了拼车车源", ""},
	{SourceDomain, "carpool_listing.updated", "carpool_listing", DomainCarpool, "拼车车源更新", OutcomeStatusChanged, "车主更新了拼车车源", ""},
	{SourceDomain, "carpool_listing.published", "carpool_listing", DomainCarpool, "拼车车源发布", OutcomeStatusChanged, "车主发布了拼车车源", ""},
	{SourceDomain, "carpool_listing.rejected", "carpool_listing", DomainCarpool, "拼车车源审核拒绝", OutcomeStatusChanged, "管理员拒绝了拼车车源", ""},
	{SourceDomain, "carpool_listing.changes_requested", "carpool_listing", DomainCarpool, "拼车车源要求修改", OutcomeStatusChanged, "管理员要求修改拼车车源", ""},
	{SourceDomain, "carpool_listing.paused", "carpool_listing", DomainCarpool, "拼车车源下架", OutcomeStatusChanged, "管理员下架了拼车车源", ""},
	{SourceDomain, "carpool_listing.resumed", "carpool_listing", DomainCarpool, "拼车车源恢复", OutcomeStatusChanged, "车主恢复了拼车车源", ""},
	{SourceDomain, "carpool_listing.recruitment_updated", "carpool_listing", DomainCarpool, "拼车招募状态更新", OutcomeStatusChanged, "车主更新了拼车招募状态", ""},
	{SourceDomain, "carpool_application.created", "carpool_application", DomainCarpool, "拼车申请创建", OutcomeSucceeded, "用户提交了拼车申请", ""},
	{SourceDomain, "carpool_application.conditions_confirmed", "carpool_application", DomainCarpool, "拼车条件确认", OutcomeStatusChanged, "买家确认了最新车源条件", ""},
	{SourceDomain, "carpool_application.rejected", "carpool_application", DomainCarpool, "拼车申请拒绝", OutcomeStatusChanged, "车主拒绝了拼车申请", ""},
	{SourceDomain, "carpool_application.cancelled_by_buyer", "carpool_application", DomainCarpool, "拼车申请取消", OutcomeStatusChanged, "买家取消了拼车申请", ""},
	{SourceDomain, "carpool_application.joined", "carpool_application", DomainCarpool, "拼车确认上车", OutcomeStatusChanged, "车主确认上车并建立了有效成员关系", ""},
	{SourceDomain, "carpool_membership.left", "carpool_membership", DomainCarpool, "拼车退出", OutcomeStatusChanged, "买家退出了拼车", ""},
	{SourceDomain, "carpool_membership.removed", "carpool_membership", DomainCarpool, "拼车成员移除", OutcomeStatusChanged, "车主移除了拼车成员", ""},
	{SourceDomain, "api_purchase_intent.created", "api_purchase_intent", DomainAPIOrder, "购买意向创建", OutcomeSucceeded, "买家创建了 API 购买意向", ""},
	{SourceDomain, "api_purchase_intent.contacted", "api_purchase_intent", DomainAPIOrder, "购买意向已联系", OutcomeStatusChanged, "商家标记购买意向已联系", ""},
	{SourceDomain, "api_purchase_intent.buyer_cancelled", "api_purchase_intent", DomainAPIOrder, "购买意向取消", OutcomeStatusChanged, "买家取消了购买意向", ""},
	{SourceDomain, "api_purchase_intent.owner_closed", "api_purchase_intent", DomainAPIOrder, "购买意向关闭", OutcomeStatusChanged, "商家关闭了购买意向", ""},
	{SourceDomain, "api_service.created", "api_service", DomainAPIService, "API 服务创建", OutcomeSucceeded, "商家创建了 API 服务", ""},
	{SourceDomain, "api_service.updated", "api_service", DomainAPIService, "API 服务更新", OutcomeStatusChanged, "商家更新了 API 服务", ""},
	{SourceDomain, "api_service.probe_binding_changed", "api_service", DomainAPIService, "API 服务探针变更", OutcomeStatusChanged, "商家变更了 API 服务探针连接", ""},
	{SourceDomain, "api_service.order_settings_changed", "api_service", DomainAPIService, "API 服务接单设置变更", OutcomeStatusChanged, "商家变更了 API 服务接单设置", ""},
	{SourceDomain, "api_service.review_submitted", "api_service", DomainAPIService, "API 服务提交审核", OutcomeStatusChanged, "商家提交了 API 服务审核", ""},
	{SourceDomain, "api_service.published", "api_service", DomainAPIService, "API 服务发布", OutcomeStatusChanged, "商家发布了 API 服务", ""},
	{SourceDomain, "api_service.paused", "api_service", DomainAPIService, "API 服务暂停", OutcomeStatusChanged, "商家暂停了 API 服务", ""},
	{SourceDomain, "api_service.resumed", "api_service", DomainAPIService, "API 服务恢复", OutcomeStatusChanged, "商家恢复了 API 服务", ""},
	{SourceDomain, "api_service.revision_started", "api_service", DomainAPIService, "API 服务修订开始", OutcomeStatusChanged, "商家开始修订 API 服务", ""},
	{SourceDomain, "api_quota_batch.created", "api_quota_batch", DomainAPIQuota, "额度批次创建", OutcomeSucceeded, "商家创建了额度批次", ""},
	{SourceDomain, "api_quota_batch.published", "api_quota_batch", DomainAPIQuota, "额度批次发布", OutcomeStatusChanged, "商家发布了额度批次", ""},
	{SourceDomain, "api_quota_batch.paused", "api_quota_batch", DomainAPIQuota, "额度批次暂停", OutcomeStatusChanged, "商家暂停了额度批次", ""},
	{SourceDomain, "api_quota_batch.resumed", "api_quota_batch", DomainAPIQuota, "额度批次恢复", OutcomeStatusChanged, "商家恢复了额度批次", ""},
	{SourceDomain, "api_quota_batch.archived", "api_quota_batch", DomainAPIQuota, "额度批次归档", OutcomeStatusChanged, "商家归档了额度批次", ""},
	{SourceDomain, "api_quota_batch.inventory_expired", "api_quota_batch", DomainAPIQuota, "额度库存到期", OutcomeStatusChanged, "系统将过期额度库存失效", ""},
	{SourceDomain, "api_quota_offer.created", "api_quota_offer", DomainAPIQuota, "额度规格创建", OutcomeSucceeded, "商家创建了额度规格", ""},
	{SourceDomain, "api_quota_offer.published", "api_quota_offer", DomainAPIQuota, "额度规格发布", OutcomeStatusChanged, "商家发布了额度规格", ""},
	{SourceDomain, "api_quota_offer.archived", "api_quota_offer", DomainAPIQuota, "额度规格归档", OutcomeStatusChanged, "商家归档了额度规格", ""},
	{SourceDomain, "api_quota_offer.credentials_imported", "api_quota_offer", DomainAPIQuota, "额度凭据导入", OutcomeSucceeded, "商家导入了额度凭据", ""},
	{SourceDomain, "api_quota_sale_round.created", "api_quota_sale_round", DomainAPIQuota, "额度场次创建", OutcomeSucceeded, "商家创建了额度销售场次", ""},
	{SourceDomain, "api_quota_sale_round.cancelled", "api_quota_sale_round", DomainAPIQuota, "额度场次取消", OutcomeStatusChanged, "商家取消了额度销售场次", ""},
	{SourceDomain, "api_quota_sale_round.expired", "api_quota_sale_round", DomainAPIQuota, "额度场次到期", OutcomeStatusChanged, "系统将过期额度销售场次关闭", ""},
	{SourceDomain, "contact_method.created", "contact_method", DomainContact, "联系方式创建", OutcomeSucceeded, "用户创建了联系方式", ""},
	{SourceDomain, "contact_method.updated", "contact_method", DomainContact, "联系方式更新", OutcomeStatusChanged, "用户更新了联系方式配置", ""},
	{SourceDomain, "contact_method.default_changed", "contact_method", DomainContact, "默认联系方式变更", OutcomeStatusChanged, "用户变更了默认联系方式", ""},
	{SourceDomain, "contact_method.verified", "contact_method", DomainContact, "联系方式验证", OutcomeStatusChanged, "用户验证了联系方式", ""},
	{SourceDomain, "contact_method.disabled", "contact_method", DomainContact, "联系方式停用", OutcomeStatusChanged, "用户停用了联系方式", ""},

	{SourceAPIOrder, "api_order.created", "api_order", DomainAPIOrder, "订单创建", OutcomeSucceeded, "买家创建了 API 订单", "/admin/api-orders/{id}"},
	{SourceAPIOrder, "api_order.payment_submitted", "api_order", DomainAPIOrder, "付款已提交", OutcomeStatusChanged, "买家提交了付款信息", "/admin/api-orders/{id}"},
	{SourceAPIOrder, "api_order.payment_issue_reported", "api_order", DomainAPIOrder, "付款异常上报", OutcomeStatusChanged, "商家上报了付款异常", "/admin/api-orders/{id}"},
	{SourceAPIOrder, "api_order.payment_confirmed", "api_order", DomainAPIOrder, "付款已确认", OutcomeStatusChanged, "商家确认了付款", "/admin/api-orders/{id}"},
	{SourceAPIOrder, "api_order.delivery_submitted", "api_order", DomainAPIOrder, "交付已提交", OutcomeStatusChanged, "商家提交了订单交付", "/admin/api-orders/{id}"},
	{SourceAPIOrder, "api_order.completed", "api_order", DomainAPIOrder, "订单完成", OutcomeStatusChanged, "买家确认了订单完成", "/admin/api-orders/{id}"},
	{SourceAPIOrder, "api_order.cancelled", "api_order", DomainAPIOrder, "订单取消", OutcomeStatusChanged, "订单已取消", "/admin/api-orders/{id}"},
	{SourceAPIOrder, "api_order.payment_timeout_cancelled", "api_order", DomainAPIOrder, "付款超时取消", OutcomeStatusChanged, "系统因付款超时取消了订单", "/admin/api-orders/{id}"},
	{SourceAPIOrder, "api_order.dispute_opened", "api_order", DomainAPIOrder, "订单纠纷发起", OutcomeStatusChanged, "订单参与方发起了纠纷", "/admin/api-orders/{id}"},
	{SourceAPIOrder, "api_order.dispute_remedy_awaiting", "api_order", DomainAPIOrder, "等待纠纷补救", OutcomeStatusChanged, "订单进入等待补救状态", "/admin/api-orders/{id}"},
	{SourceAPIOrder, "api_order.dispute_remedy_claimed", "api_order", DomainAPIOrder, "纠纷补救已声明", OutcomeStatusChanged, "责任方声明已完成补救", "/admin/api-orders/{id}"},
	{SourceAPIOrder, "api_order.dispute_remedy_contested", "api_order", DomainAPIOrder, "纠纷补救被异议", OutcomeStatusChanged, "受益方对补救提出异议", "/admin/api-orders/{id}"},
	{SourceAPIOrder, "api_order.dispute_closed", "api_order", DomainAPIOrder, "订单纠纷关闭", OutcomeStatusChanged, "订单纠纷已关闭", "/admin/api-orders/{id}"},
	{SourceAPIOrder, "api_order.delivery_review_reminder_sent", "api_order", DomainAPIOrder, "交付确认提醒", OutcomeSucceeded, "系统发送了交付确认提醒", "/admin/api-orders/{id}"},
	{SourceAPIOrder, "api_order.auto_completed", "api_order", DomainAPIOrder, "订单自动完成", OutcomeStatusChanged, "系统自动完成了订单", "/admin/api-orders/{id}"},

	{SourceContactSessionAccess, "contact_session.accessed", "contact_session", DomainContact, "联系窗口读取", OutcomeAccessed, "用户读取了联系窗口中的联系方式", ""},
	{SourceAPIIntentContactAccess, "api_purchase_intent.contact_accessed", "api_purchase_intent", DomainContact, "购买意向联系方式读取", OutcomeAccessed, "订单参与方读取了购买意向联系方式", ""},
	{SourceAPIOrderAccess, "api_order.payment_instructions_accessed", "api_order", DomainAPIOrder, "付款指引读取", OutcomeAccessed, "买家读取了订单付款指引", "/admin/api-orders/{id}"},

	{SourceProbe, "created", "api_probe_connection", DomainProbe, "探针创建", OutcomeSucceeded, "商家创建了探针连接", ""},
	{SourceProbe, "updated", "api_probe_connection", DomainProbe, "探针更新", OutcomeStatusChanged, "商家更新了探针连接配置", ""},
	{SourceProbe, "model_changed", "api_probe_connection", DomainProbe, "探针模型变更", OutcomeStatusChanged, "商家变更了探针模型", ""},
	{SourceProbe, "verify_succeeded", "api_probe_connection", DomainProbe, "探针验证成功", OutcomeStatusChanged, "探针连接验证成功", ""},
	{SourceProbe, "verify_failed", "api_probe_connection", DomainProbe, "探针验证失败", OutcomeStatusChanged, "探针连接验证失败", ""},
	{SourceProbe, "enabled", "api_probe_connection", DomainProbe, "探针启用", OutcomeStatusChanged, "探针连接已启用", ""},
	{SourceProbe, "disabled", "api_probe_connection", DomainProbe, "探针停用", OutcomeStatusChanged, "探针连接已停用", ""},
	{SourceProbe, "deleted", "api_probe_connection", DomainProbe, "探针删除", OutcomeStatusChanged, "探针连接已删除", ""},
}

func ActionRegistry() []ActionDefinition {
	return append([]ActionDefinition(nil), actionRegistry...)
}

func LookupAction(sourceKind, action, targetType string) (ActionDefinition, bool) {
	sourceKind = strings.TrimSpace(sourceKind)
	action = strings.TrimSpace(action)
	targetType = strings.TrimSpace(targetType)
	for _, definition := range actionRegistry {
		if definition.SourceKind == sourceKind && definition.Action == action && definition.TargetType == targetType {
			return definition, true
		}
	}
	return ActionDefinition{}, false
}

func BuildDetailPath(definition ActionDefinition, targetID string) string {
	if definition.DetailPattern == "" {
		return ""
	}
	if !strings.Contains(definition.DetailPattern, "{id}") {
		return definition.DetailPattern
	}
	targetID = strings.TrimSpace(targetID)
	if targetID == "" {
		return ""
	}
	if _, err := uuid.Parse(targetID); err != nil {
		return ""
	}
	return strings.ReplaceAll(definition.DetailPattern, "{id}", targetID)
}

func AllowedActorKinds(definition ActionDefinition) []string {
	switch definition.SourceKind {
	case SourceAdmin, SourceModeration:
		return []string{ActorAdmin}
	case SourceContactSessionAccess, SourceAPIIntentContactAccess, SourceAPIOrderAccess:
		return []string{ActorUser}
	case SourceAPIOrder:
		switch definition.Action {
		case "api_order.payment_timeout_cancelled", "api_order.delivery_review_reminder_sent", "api_order.auto_completed":
			return []string{ActorSystem}
		default:
			return []string{ActorUser}
		}
	case SourceProbe:
		return []string{ActorUser, ActorAdmin, ActorSystem}
	case SourceDomain:
		switch definition.Action {
		case "api_quota_batch.inventory_expired", "api_quota_sale_round.expired":
			return []string{ActorSystem}
		case "carpool_listing.rejected", "carpool_listing.changes_requested":
			return []string{ActorAdmin}
		case "carpool_listing.published", "carpool_listing.paused", "carpool_listing.resumed":
			return []string{ActorUser, ActorAdmin}
		default:
			return []string{ActorUser}
		}
	default:
		return nil
	}
}
