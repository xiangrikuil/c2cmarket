package report

import (
	"strings"

	"c2c-market/backend/internal/domain"
)

func ResolveAppealSource(appellantUserID string, sourceReport *Report, sourceDispute *DisputeCase) (AppealSource, *domain.AppError) {
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
		if !canAppealDisputeUser(*sourceDispute, appellantUserID) {
			return AppealSource{}, appealSourcePermissionDenied()
		}
		if sourceReport != nil && (sourceDispute.ReportID != sourceReport.ID || sourceReport.DisputeID != sourceDispute.ID) {
			return AppealSource{}, fieldError("source", "举报与纠纷不属于同一案件。")
		}
		if !isAppealableDisputeStatus(sourceDispute.Status) {
			return AppealSource{}, invalidState("纠纷处理完成后才能提交申诉。")
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
	return canAppealDisputeUser(item, userID) && isAppealableDisputeStatus(item.Status)
}

func canAppealDisputeUser(item DisputeCase, userID string) bool {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return false
	}
	if subjectUserID := strings.TrimSpace(item.SubjectUserID); subjectUserID != "" {
		return subjectUserID == userID
	}
	return item.PrimaryUserID == userID || item.CounterpartyUserID == userID
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
