package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"sync"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/idempotency"
)

type studentRegistrationTestSender struct {
	mu sync.Mutex
}

func (sender *studentRegistrationTestSender) SendVerificationCode(context.Context, string, string, time.Time) *domain.AppError {
	return nil
}

func (sender *studentRegistrationTestSender) SendRegistrationSuccess(context.Context, string, string, string, time.Time) *domain.AppError {
	sender.mu.Lock()
	sender.mu.Unlock()
	return nil
}

func (sender *studentRegistrationTestSender) ExposeDevCode() bool { return true }

func TestStudentRegistrationAdminAndRegistrationLifecycle(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC)
	service := NewServiceWithRegistrationEmailSender(nil, func() time.Time { return now }, &studentRegistrationTestSender{})
	admin, _, appErr := service.CreateDevSession(ctx, "student-admin", true)
	if appErr != nil {
		t.Fatalf("CreateDevSession(admin) error = %v", appErr)
	}

	if _, appErr := service.AdminStudentRegistration(ctx, User{Capabilities: []string{CapabilityAdminAccess}}); appErr == nil || appErr.Code != domain.CodeCapabilityRequired {
		t.Fatalf("stale capability projection authorized admin read: %v", appErr)
	}

	createdCompletion, appErr := service.CreateStudentInstitutionDomainWithIdempotency(
		ctx, admin, "POST /student-domains", "create-example", "create-example-hash",
		StudentInstitutionDomainCreateInput{Domain: "example.edu", InstitutionName: "Example University", Enabled: true, Reason: "批准测试院校", RequestID: "req-create"},
		studentInstitutionTestCompletion(http.StatusCreated),
	)
	if appErr != nil || createdCompletion.Status != http.StatusCreated {
		t.Fatalf("create institution completion=%+v error=%v", createdCompletion, appErr)
	}
	replayed, appErr := service.CreateStudentInstitutionDomainWithIdempotency(
		ctx, admin, "POST /student-domains", "create-example", "create-example-hash",
		StudentInstitutionDomainCreateInput{Domain: "example.edu", InstitutionName: "Example University", Enabled: true, Reason: "批准测试院校", RequestID: "req-create"},
		studentInstitutionTestCompletion(http.StatusCreated),
	)
	if appErr != nil || !reflect.DeepEqual(replayed.Body, createdCompletion.Body) {
		t.Fatalf("idempotent domain replay completion=%+v error=%v", replayed, appErr)
	}
	if _, appErr := service.CreateStudentInstitutionDomainWithIdempotency(
		ctx, admin, "POST /student-domains", "invalid-domain", "invalid-domain-hash",
		StudentInstitutionDomainCreateInput{Domain: "*.example.edu", InstitutionName: "Wildcard", Enabled: true, Reason: "非法测试", RequestID: "req-invalid"},
		studentInstitutionTestCompletion(http.StatusCreated),
	); appErr == nil || appErr.Code != domain.CodeValidationFailed {
		t.Fatalf("wildcard institution domain accepted: %v", appErr)
	}

	settingCompletion, appErr := service.UpdateAdminStudentRegistrationWithIdempotency(
		ctx, admin, "PATCH /student-registration", "enable-registration", "enable-registration-hash",
		StudentRegistrationSettingUpdate{Enabled: true, ExpectedVersion: 1, Reason: "开启受控测试", RequestID: "req-enable"},
		studentRegistrationTestCompletion,
	)
	if appErr != nil || settingCompletion.Status != http.StatusOK {
		t.Fatalf("enable registration completion=%+v error=%v", settingCompletion, appErr)
	}
	config, appErr := service.StudentRegistrationConfig(ctx)
	if appErr != nil || !config.Enabled || config.Version != 2 || len(config.Institutions) != 1 {
		t.Fatalf("unexpected registration config=%+v error=%v", config, appErr)
	}

	occupied, _, appErr := service.CreateDevSession(ctx, "occupied-student", false)
	if appErr != nil || occupied.ID == "" {
		t.Fatalf("create occupied username fixture: %v", appErr)
	}
	challenge, appErr := service.StartEmailRegistration(ctx, EmailRegistrationStartInput{Email: " learner@EXAMPLE.EDU "})
	if appErr != nil || challenge.DevCode == "" || challenge.ExpiresAt.Sub(now) != 15*time.Minute {
		t.Fatalf("start registration challenge=%+v error=%v", challenge, appErr)
	}
	confirm := EmailRegistrationConfirmInput{Email: challenge.Email, Code: challenge.DevCode, Username: occupied.Username, Password: "CorrectHorse1!"}
	if _, _, appErr := service.ConfirmEmailRegistration(ctx, confirm); appErr == nil || appErr.Code != domain.CodeUsernameUnavailable {
		t.Fatalf("expected retryable username conflict, got %v", appErr)
	}
	confirm.Username = "student-buyer"
	student, session, appErr := service.ConfirmEmailRegistration(ctx, confirm)
	if appErr != nil {
		t.Fatalf("confirm registration: %v", appErr)
	}
	if student.StudentClaim == nil || student.LinuxDoBinding != nil || !reflect.DeepEqual(ProjectCapabilities(student), []string{CapabilityAPIOrderCreate}) || !session.NewRegistration {
		t.Fatalf("unexpected student registration result user=%+v session=%+v", student, session)
	}
	loggedIn, _, appErr := service.LoginWithPassword(ctx, "LEARNER@example.edu", confirm.Password)
	if appErr != nil || loggedIn.ID != student.ID {
		t.Fatalf("login by immutable claim email user=%+v error=%v", loggedIn, appErr)
	}
	if _, appErr := service.StartEmailRegistration(ctx, EmailRegistrationStartInput{Email: challenge.Email}); appErr == nil || appErr.Code != domain.CodeStudentEmailClaimed {
		t.Fatalf("claimed student email was reusable: %v", appErr)
	}
}

func TestStudentRegistrationChecksSwitchAgainAtConfirmation(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 8, 12, 11, 0, 0, 0, time.UTC)
	service := NewServiceWithRegistrationEmailSender(nil, func() time.Time { return now }, &studentRegistrationTestSender{})
	admin, _, _ := service.CreateDevSession(ctx, "switch-admin", true)
	_, appErr := service.CreateStudentInstitutionDomainWithIdempotency(ctx, admin, "create-domain", "create-switch-domain", "hash-create-switch-domain", StudentInstitutionDomainCreateInput{Domain: "college.edu", InstitutionName: "College", Enabled: true, Reason: "配置域名"}, studentInstitutionTestCompletion(http.StatusCreated))
	if appErr != nil {
		t.Fatalf("create domain: %v", appErr)
	}
	_, appErr = service.UpdateAdminStudentRegistrationWithIdempotency(ctx, admin, "switch", "enable-switch", "hash-enable-switch", StudentRegistrationSettingUpdate{Enabled: true, ExpectedVersion: 1, Reason: "开启"}, studentRegistrationTestCompletion)
	if appErr != nil {
		t.Fatalf("enable registration: %v", appErr)
	}
	challenge, appErr := service.StartEmailRegistration(ctx, EmailRegistrationStartInput{Email: "student@college.edu"})
	if appErr != nil {
		t.Fatalf("start registration: %v", appErr)
	}
	_, appErr = service.UpdateAdminStudentRegistrationWithIdempotency(ctx, admin, "switch", "disable-switch", "hash-disable-switch", StudentRegistrationSettingUpdate{Enabled: false, ExpectedVersion: 2, Reason: "关闭"}, studentRegistrationTestCompletion)
	if appErr != nil {
		t.Fatalf("disable registration: %v", appErr)
	}
	if _, _, appErr := service.ConfirmEmailRegistration(ctx, EmailRegistrationConfirmInput{Email: challenge.Email, Code: challenge.DevCode, Username: "late-student", Password: "CorrectHorse1!"}); appErr == nil || appErr.Code != domain.CodeEmailRegistrationDisabled {
		t.Fatalf("confirmation ignored disabled switch: %v", appErr)
	}
}

func studentRegistrationTestCompletion(config StudentRegistrationConfig) (idempotency.Completion, *domain.AppError) {
	body, _ := json.Marshal(map[string]any{"enabled": config.Enabled, "version": config.Version})
	return idempotency.Completion{Status: http.StatusOK, ContentType: "application/json", Body: body, ResourceType: "student_registration_setting", ResourceID: studentRegistrationSettingAuditTargetID}, nil
}

func studentInstitutionTestCompletion(status int) StudentInstitutionDomainCompletionBuilder {
	return func(item StudentInstitutionDomain) (idempotency.Completion, *domain.AppError) {
		body, _ := json.Marshal(item)
		return idempotency.Completion{Status: status, ContentType: "application/json", Body: body, ResourceType: "student_institution_domain", ResourceID: item.ID}, nil
	}
}
