package report

import (
	"sync"
	"testing"

	"c2c-market/backend/internal/domain"
)

func TestResolveAppealSourceEnforcesEligibilityAndDerivesTarget(t *testing.T) {
	reportSource := Report{
		ID:                  "report-1",
		ReporterUserID:      "reporter-1",
		TargetType:          TargetAPIPurchaseIntent,
		TargetID:            "intent-1",
		CanonicalTargetType: TargetAPIOrder,
		CanonicalTargetID:   "order-1",
		Status:              ReportStatusRejected,
	}
	disputeSource := DisputeCase{
		ID:                 "dispute-1",
		ReportID:           reportSource.ID,
		TargetType:         TargetAPIOrder,
		TargetID:           "order-1",
		PrimaryUserID:      "reporter-1",
		CounterpartyUserID: "counterparty-1",
		SubjectUserID:      "subject-1",
		Status:             DisputeStatusResolved,
	}

	source, appErr := ResolveAppealSource("subject-1", nil, &disputeSource)
	if appErr != nil {
		t.Fatalf("resolve dispute subject source: %v", appErr)
	}
	if source.TargetType != TargetAPIOrder || source.TargetID != "order-1" {
		t.Fatalf("unexpected derived dispute target: %#v", source)
	}

	reportOnly, appErr := ResolveAppealSource("reporter-1", &reportSource, nil)
	if appErr != nil {
		t.Fatalf("resolve reporter source: %v", appErr)
	}
	if reportOnly.TargetType != TargetAPIOrder || reportOnly.TargetID != "order-1" {
		t.Fatalf("expected canonical report target, got %#v", reportOnly)
	}

	for _, testCase := range []struct {
		name       string
		userID     string
		report     *Report
		dispute    *DisputeCase
		expectCode string
	}{
		{name: "non reporter", userID: "outsider-1", report: &reportSource, expectCode: domain.CodeObjectNotFound},
		{name: "non subject participant", userID: "reporter-1", dispute: &disputeSource, expectCode: domain.CodePermissionDenied},
		{name: "non participant", userID: "outsider-1", dispute: &disputeSource, expectCode: domain.CodeObjectNotFound},
		{name: "missing source", userID: "reporter-1", expectCode: domain.CodeValidationFailed},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			_, appErr := ResolveAppealSource(testCase.userID, testCase.report, testCase.dispute)
			if appErr == nil || appErr.Code != testCase.expectCode {
				t.Fatalf("expected %s, got %#v", testCase.expectCode, appErr)
			}
		})
	}
}

func TestResolveAppealSourceAuthorizesBeforeComparingDualSourceLinks(t *testing.T) {
	reportSource := Report{
		ID:             "report-1",
		DisputeID:      "dispute-1",
		ReporterUserID: "reporter-1",
		Status:         ReportStatusDisputeOpened,
	}
	disputeSource := DisputeCase{
		ID:            "dispute-2",
		ReportID:      "report-2",
		PrimaryUserID: "participant-1",
		Status:        DisputeStatusResolved,
	}
	if _, appErr := ResolveAppealSource("outsider-1", &reportSource, &disputeSource); appErr == nil || appErr.Code != domain.CodeObjectNotFound {
		t.Fatalf("unauthorized dual sources must not reveal link state, got %#v", appErr)
	}
}

func TestResolveAppealSourceAllowsEitherParticipantWhenDisputeHasNoSubject(t *testing.T) {
	disputeSource := DisputeCase{
		ID:                 "dispute-1",
		TargetType:         TargetAPIOrder,
		TargetID:           "order-1",
		PrimaryUserID:      "primary-1",
		CounterpartyUserID: "counterparty-1",
		Status:             DisputeStatusClosed,
	}
	for _, userID := range []string{"primary-1", "counterparty-1"} {
		if _, appErr := ResolveAppealSource(userID, nil, &disputeSource); appErr != nil {
			t.Fatalf("participant %s must be allowed: %v", userID, appErr)
		}
	}
}

func TestResolveAppealSourceEnforcesFinalSourceState(t *testing.T) {
	reportSource := Report{
		ID:             "report-1",
		ReporterUserID: "reporter-1",
		TargetType:     TargetPublicUser,
		TargetID:       "target-1",
		Status:         ReportStatusSubmitted,
	}
	if _, appErr := ResolveAppealSource("reporter-1", &reportSource, nil); appErr == nil || appErr.Code != domain.CodeInvalidStateTransition {
		t.Fatalf("active report must not be appealable, got %#v", appErr)
	}

	reportSource.Status = ReportStatusClosed
	reportSource.DisputeID = "dispute-1"
	if _, appErr := ResolveAppealSource("reporter-1", &reportSource, nil); appErr == nil || appErr.Code != domain.CodeInvalidStateTransition {
		t.Fatalf("report with dispute must require dispute source, got %#v", appErr)
	}

	disputeSource := DisputeCase{
		ID:            "dispute-1",
		TargetType:    TargetPublicUser,
		TargetID:      "target-1",
		PrimaryUserID: "reporter-1",
		Status:        DisputeStatusOpen,
	}
	if _, appErr := ResolveAppealSource("reporter-1", nil, &disputeSource); appErr == nil || appErr.Code != domain.CodeInvalidStateTransition {
		t.Fatalf("active dispute must not be appealable, got %#v", appErr)
	}
}

func TestResolveAppealSourceRequiresBidirectionalCaseLink(t *testing.T) {
	reportSource := Report{
		ID:             "report-1",
		DisputeID:      "dispute-1",
		ReporterUserID: "subject-1",
		TargetType:     TargetPublicUser,
		TargetID:       "target-1",
		Status:         ReportStatusDisputeOpened,
	}
	disputeSource := DisputeCase{
		ID:            "dispute-1",
		ReportID:      "report-1",
		PrimaryUserID: "subject-1",
		SubjectUserID: "subject-1",
		TargetType:    TargetAPIOrder,
		TargetID:      "order-1",
		Status:        DisputeStatusResolved,
	}

	if _, appErr := ResolveAppealSource("subject-1", &reportSource, &disputeSource); appErr != nil {
		t.Fatalf("matching report and dispute must be allowed: %v", appErr)
	}

	disputeSource.ReportID = "report-2"
	if _, appErr := ResolveAppealSource("subject-1", &reportSource, &disputeSource); appErr == nil || appErr.Code != domain.CodeValidationFailed {
		t.Fatalf("forward mismatch must fail, got %#v", appErr)
	}
	disputeSource.ReportID = reportSource.ID
	reportSource.DisputeID = "dispute-2"
	if _, appErr := ResolveAppealSource("subject-1", &reportSource, &disputeSource); appErr == nil || appErr.Code != domain.CodeValidationFailed {
		t.Fatalf("reverse mismatch must fail, got %#v", appErr)
	}
}

func TestCanAppealDisputeMatchesCreationEligibility(t *testing.T) {
	base := DisputeCase{
		PrimaryUserID:      "primary-1",
		CounterpartyUserID: "counterparty-1",
		SubjectUserID:      "counterparty-1",
		Status:             DisputeStatusResolved,
	}
	for _, testCase := range []struct {
		name   string
		item   DisputeCase
		userID string
		want   bool
	}{
		{name: "subject can appeal resolved dispute", item: base, userID: "counterparty-1", want: true},
		{name: "non subject cannot appeal", item: base, userID: "primary-1", want: false},
		{name: "outsider cannot appeal", item: base, userID: "outsider-1", want: false},
		{name: "open dispute cannot be appealed", item: func() DisputeCase { item := base; item.Status = DisputeStatusOpen; return item }(), userID: "counterparty-1", want: false},
		{name: "participant can appeal without subject", item: func() DisputeCase { item := base; item.SubjectUserID = ""; return item }(), userID: "primary-1", want: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			if got := CanAppealDispute(testCase.item, testCase.userID); got != testCase.want {
				t.Fatalf("CanAppealDispute() = %v, want %v", got, testCase.want)
			}
		})
	}
}

func TestCreateAppealMemorySerializesSubmittedAppealCheck(t *testing.T) {
	service := NewService(nil, nil, nil)
	service.disputes["dispute-1"] = DisputeCase{
		ID:            "dispute-1",
		TargetType:    TargetAPIOrder,
		TargetID:      "order-1",
		PrimaryUserID: "subject-1",
		SubjectUserID: "subject-1",
		Status:        DisputeStatusResolved,
	}
	input := CreateAppealInput{AppellantUserID: "subject-1", DisputeID: "dispute-1", Title: "复核", Statement: "请求复核纠纷结果。"}

	start := make(chan struct{})
	results := make(chan *domain.AppError, 2)
	var waitGroup sync.WaitGroup
	for range 2 {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			<-start
			_, appErr := service.createAppealMemory(input)
			results <- appErr
		}()
	}
	close(start)
	waitGroup.Wait()
	close(results)

	created := 0
	conflicts := 0
	for appErr := range results {
		switch {
		case appErr == nil:
			created++
		case appErr.Code == domain.CodeInvalidStateTransition:
			conflicts++
		default:
			t.Fatalf("unexpected create result: %#v", appErr)
		}
	}
	if created != 1 || conflicts != 1 {
		t.Fatalf("expected one create and one conflict, got created=%d conflicts=%d", created, conflicts)
	}
}

func TestValidateAppealOutcomeSubject(t *testing.T) {
	appeal := Appeal{AppellantUserID: "subject-1"}
	if appErr := ValidateAppealOutcomeSubject(appeal, "subject-1"); appErr != nil {
		t.Fatalf("matching subject must be allowed: %v", appErr)
	}
	for _, testCase := range []struct {
		name    string
		appeal  Appeal
		subject string
	}{
		{name: "mismatch", appeal: appeal, subject: "other-1"},
		{name: "empty appellant", appeal: Appeal{}, subject: "subject-1"},
		{name: "empty outcome subject", appeal: appeal, subject: ""},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			if appErr := ValidateAppealOutcomeSubject(testCase.appeal, testCase.subject); appErr == nil || appErr.Code != domain.CodeInvalidStateTransition {
				t.Fatalf("invalid subject mapping must be rejected, got %#v", appErr)
			}
		})
	}
}
