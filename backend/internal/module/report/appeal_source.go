package report

import (
	"strings"
	"time"

	"c2c-market/backend/internal/domain"
)

func ResolveAppealSource(appellantUserID string, sourceReport *Report, sourceDispute *DisputeCase) (AppealSource, *domain.AppError) {
	return ResolveAppealSourceAt(appellantUserID, sourceReport, sourceDispute, time.Now())
}

func ResolveAppealSourceAt(appellantUserID string, sourceReport *Report, sourceDispute *DisputeCase, now time.Time) (AppealSource, *domain.AppError) {
	appellantUserID = strings.TrimSpace(appellantUserID)
	if sourceReport == nil && sourceDispute == nil {
		return AppealSource{}, fieldError("source", "申诉必须关联举报或纠纷。")
	}
	if sourceReport != nil && sourceReport.ReporterUserID != appellantUserID {
		return AppealSource{}, appealSourceNotFound()
	}
	if sourceDispute != nil {
		if !isDisputeParticipant(*sourceDispute, appellantUserID) {
			return AppealSource{}, appealSourceNotFound()
		}
		if sourceReport != nil && (sourceDispute.ReportID != sourceReport.ID || sourceReport.DisputeID != sourceDispute.ID) {
			return AppealSource{}, fieldError("source", "举报与纠纷不属于同一案件。")
		}
		if sourceDispute.Active || !isAppealableDisputeStatus(sourceDispute.Status) {
			return AppealSource{}, invalidState("纠纷处理完成后才能提交申诉。")
		}
		if strings.TrimSpace(sourceDispute.FinalReason) == "" || sourceDispute.AppealExpiresAt == nil || len(sourceDispute.AdverselyAffectedIDs) == 0 {
			return AppealSource{}, invalidState("纠纷缺少完整终局信息，暂不能提交申诉。")
		}
		if !canAppealDisputeUser(*sourceDispute, appellantUserID) {
			return AppealSource{}, appealSourcePermissionDenied()
		}
		if !now.Before(*sourceDispute.AppealExpiresAt) {
			return AppealSource{}, invalidState("纠纷申诉期限已到。")
		}
		return AppealSource{TargetType: sourceDispute.TargetType, TargetID: sourceDispute.TargetID}, nil
	}
	if strings.TrimSpace(sourceReport.DisputeID) != "" {
		return AppealSource{}, invalidState("该举报已转入纠纷，请通过关联纠纷提交申诉。")
	}
	if !isAppealableReportStatus(sourceReport.Status) {
		return AppealSource{}, invalidState("举报被驳回或关闭后才能提交申诉。")
	}
	return AppealSource{
		TargetType: nonEmpty(sourceReport.CanonicalTargetType, sourceReport.TargetType),
		TargetID:   nonEmpty(sourceReport.CanonicalTargetID, sourceReport.TargetID),
	}, nil
}

func isDisputeParticipant(item DisputeCase, userID string) bool {
	userID = strings.TrimSpace(userID)
	return userID != "" && (item.PrimaryUserID == userID || item.CounterpartyUserID == userID || item.SubjectUserID == userID)
}

func CanAppealDispute(item DisputeCase, userID string) bool {
	return CanAppealDisputeAt(item, userID, time.Now())
}

func CanAppealDisputeAt(item DisputeCase, userID string, now time.Time) bool {
	return !item.Active && strings.TrimSpace(item.FinalReason) != "" && item.AppealExpiresAt != nil &&
		len(item.AdverselyAffectedIDs) > 0 && canAppealDisputeUser(item, userID) &&
		isAppealableDisputeStatus(item.Status) && now.Before(*item.AppealExpiresAt)
}

func canAppealDisputeUser(item DisputeCase, userID string) bool {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return false
	}
	for _, affectedUserID := range item.AdverselyAffectedIDs {
		if strings.TrimSpace(affectedUserID) == userID {
			return true
		}
	}
	return false
}

func isAppealableReportStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case ReportStatusRejected, ReportStatusClosed:
		return true
	default:
		return false
	}
}

func isAppealableDisputeStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case DisputeStatusResolved, DisputeStatusClosed:
		return true
	default:
		return false
	}
}

func ValidateNoSubmittedAppeal(exists bool) *domain.AppError {
	if exists {
		return invalidState("该举报或纠纷已有待处理申诉。")
	}
	return nil
}

func ValidateNoSubmittedAccountGovernanceAppeal(exists bool) *domain.AppError {
	if exists {
		return invalidState("该账号已有待处理的账号治理申诉。")
	}
	return nil
}

func ValidateAppealOutcomeSubject(appeal Appeal, outcomeSubjectUserID string) *domain.AppError {
	appellantUserID := strings.TrimSpace(appeal.AppellantUserID)
	outcomeSubjectUserID = strings.TrimSpace(outcomeSubjectUserID)
	if appellantUserID == "" || outcomeSubjectUserID == "" || appellantUserID != outcomeSubjectUserID {
		return invalidState("只有信誉裁定主体提交的申诉才能反转该裁定。")
	}
	return nil
}

func ValidateAppealAdverseSubject(isAdverselyAffected bool) *domain.AppError {
	if !isAdverselyAffected {
		return invalidState("只有终局中记录的不利主体才能批准该申诉。")
	}
	return nil
}

func ResolveAdverselyAffectedUsers(item DisputeCase, requested []string) ([]string, *domain.AppError) {
	participants := map[string]bool{}
	for _, userID := range []string{item.PrimaryUserID, item.CounterpartyUserID, item.SubjectUserID} {
		if userID = strings.TrimSpace(userID); userID != "" {
			participants[userID] = true
		}
	}
	if len(requested) == 0 {
		if strings.TrimSpace(item.SubjectUserID) != "" {
			requested = []string{item.SubjectUserID}
		} else {
			requested = []string{item.PrimaryUserID, item.CounterpartyUserID}
		}
	}
	result := make([]string, 0, len(requested))
	seen := map[string]bool{}
	for _, userID := range requested {
		userID = strings.TrimSpace(userID)
		if userID == "" || !participants[userID] {
			return nil, fieldError("adverselyAffectedUserIds", "受不利影响用户必须是当前纠纷参与者。")
		}
		if !seen[userID] {
			seen[userID] = true
			result = append(result, userID)
		}
	}
	if len(result) == 0 {
		return nil, fieldError("adverselyAffectedUserIds", "终局必须指定至少一名可申诉参与者。")
	}
	return result, nil
}
