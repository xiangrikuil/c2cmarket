package auth

import (
	"net/http"
	"sort"

	"c2c-market/backend/internal/domain"
)

const (
	CapabilityAPIOrderCreate    = "api_order.create"
	CapabilityCarpoolApply      = "carpool.apply"
	CapabilityCarpoolPublish    = "carpool.publish"
	CapabilityAPIServicePublish = "api_service.publish"
	CapabilityAPIQuotaPublish   = "api_quota.publish"
	CapabilityAPIProbeManage    = "api_probe.manage"
	CapabilityAdminAccess       = "admin.access"
)

var AllCapabilities = []string{
	CapabilityAdminAccess,
	CapabilityAPIOrderCreate,
	CapabilityAPIProbeManage,
	CapabilityAPIQuotaPublish,
	CapabilityAPIServicePublish,
	CapabilityCarpoolApply,
	CapabilityCarpoolPublish,
}

// ProjectCapabilities derives the complete global business-capability set
// from current durable identity facts. Session rows never store this result.
func ProjectCapabilities(user User) []string {
	set := make(map[string]struct{}, len(AllCapabilities))
	if user.LinuxDoBinding != nil && user.LinuxDoBinding.Bound {
		set[CapabilityAPIOrderCreate] = struct{}{}
		set[CapabilityCarpoolApply] = struct{}{}
		set[CapabilityCarpoolPublish] = struct{}{}
		set[CapabilityAPIServicePublish] = struct{}{}
		set[CapabilityAPIQuotaPublish] = struct{}{}
		set[CapabilityAPIProbeManage] = struct{}{}
	} else if user.StudentClaim != nil {
		set[CapabilityAPIOrderCreate] = struct{}{}
	}
	if user.IsAdmin && !isStudentOnlyIdentity(user) {
		set[CapabilityAdminAccess] = struct{}{}
	}

	capabilities := make([]string, 0, len(set))
	for capability := range set {
		capabilities = append(capabilities, capability)
	}
	sort.Strings(capabilities)
	return capabilities
}

func HydrateCapabilities(user User) User {
	if isStudentOnlyIdentity(user) {
		// 高校邮箱账号必须先绑定 Linux.do，才能让持久管理员标记生效。
		user.IsAdmin = false
	}
	user.Capabilities = ProjectCapabilities(user)
	return user
}

func isStudentOnlyIdentity(user User) bool {
	return user.StudentClaim != nil && (user.LinuxDoBinding == nil || !user.LinuxDoBinding.Bound)
}

func HasCapability(user User, capability string) bool {
	return HasProjectedCapability(ProjectCapabilities(user), capability)
}

func HasProjectedCapability(capabilities []string, capability string) bool {
	for _, current := range capabilities {
		if current == capability {
			return true
		}
	}
	return false
}

func RequireProjectedCapability(capabilities []string, capability string) *domain.AppError {
	if HasProjectedCapability(capabilities, capability) {
		return nil
	}
	return capabilityRequiredError(capability)
}

func RequireCapability(user User, capability string) *domain.AppError {
	if HasCapability(user, capability) {
		return nil
	}
	return capabilityRequiredError(capability)
}

func capabilityRequiredError(capability string) *domain.AppError {
	return domain.NewFieldError(
		http.StatusForbidden,
		domain.CodeCapabilityRequired,
		"Capability required",
		"当前账号缺少执行该操作所需的能力。",
		"capability",
		"required",
		capability,
	)
}
