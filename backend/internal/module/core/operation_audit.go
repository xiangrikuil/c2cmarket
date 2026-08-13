package core

import (
	"context"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/auth"
	"c2c-market/backend/internal/module/operationaudit"
)

func (s *Service) AdminOperationAuditLogs(ctx context.Context, user auth.User, filter operationaudit.Filter) (domain.Page[operationaudit.Entry], *domain.AppError) {
	return s.operationAudit.AdminOperationAuditLogs(ctx, user, filter)
}
