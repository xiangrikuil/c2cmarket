package core

import (
	"context"
	"net/http"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/apiorder"
	"c2c-market/backend/internal/module/reputation"
)

type sellerPublishActionChecker struct {
	reputation *reputation.Service
	orders     *apiorder.Service
}

func (c *sellerPublishActionChecker) CheckActionAllowed(ctx context.Context, userID, role, action string) *domain.AppError {
	if c == nil {
		return nil
	}
	if c.reputation != nil {
		if appErr := c.reputation.CheckActionAllowed(ctx, userID, role, action); appErr != nil {
			return appErr
		}
	}
	if role != reputation.RoleSeller || action != reputation.ActionAPIServicePublish || c.orders == nil {
		return nil
	}
	active, appErr := c.orders.HasActiveDisputeForSeller(ctx, userID)
	if appErr != nil {
		return appErr
	}
	if !active {
		return nil
	}
	return domain.NewError(
		http.StatusConflict,
		domain.CodeActiveAPIOrderDispute,
		"Active API order dispute",
		"当前存在未解决的 API 订单纠纷，暂不能发布或恢复 API 服务与额度，也不会接收新订单。请先完成纠纷处理。",
	)
}
