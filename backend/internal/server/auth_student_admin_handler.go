package server

import (
	"encoding/json"
	"net/http"
	"strconv"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/auth"
	"c2c-market/backend/internal/module/idempotency"

	"github.com/go-chi/chi/v5"
)

type adminStudentRegistrationResponse struct {
	Enabled bool  `json:"enabled"`
	Version int64 `json:"version"`
}

type adminStudentRegistrationUpdateRequest struct {
	Enabled         *bool  `json:"enabled"`
	ExpectedVersion int64  `json:"expectedVersion"`
	Reason          string `json:"reason"`
}

type adminStudentInstitutionDomainResponse struct {
	ID              string `json:"id"`
	Domain          string `json:"domain"`
	InstitutionName string `json:"institutionName"`
	Enabled         bool   `json:"enabled"`
	Version         int64  `json:"version"`
	CreatedAt       string `json:"createdAt"`
	UpdatedAt       string `json:"updatedAt"`
}

type adminStudentInstitutionDomainCreateRequest struct {
	Domain          string `json:"domain"`
	InstitutionName string `json:"institutionName"`
	Enabled         *bool  `json:"enabled"`
	Reason          string `json:"reason"`
}

type adminStudentInstitutionDomainUpdateRequest struct {
	InstitutionName string `json:"institutionName"`
	Enabled         *bool  `json:"enabled"`
	ExpectedVersion int64  `json:"expectedVersion"`
	Reason          string `json:"reason"`
}

func (s *Server) handleAdminStudentRegistration(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	config, appErr := s.app.AdminStudentRegistration(r.Context(), user)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	setETag(w, config.Version)
	writeJSON(w, http.StatusOK, toAdminStudentRegistrationResponse(config))
}

func (s *Server) handleUpdateAdminStudentRegistration(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	body, request, appErr := decodeStrictJSON[adminStudentRegistrationUpdateRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	if request.Enabled == nil {
		writeProblem(w, r, studentAdminRequestError("enabled", "必须明确指定是否启用学生注册。"))
		return
	}
	version, appErr := requireIfMatchVersion(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	if request.ExpectedVersion != version {
		writeProblem(w, r, studentAdminRequestError("expectedVersion", "expectedVersion 必须与 If-Match 一致。"))
		return
	}
	const routeKey = "PATCH /api/v1/admin/student-registration"
	completion, appErr := s.app.UpdateAdminStudentRegistrationWithIdempotency(
		r.Context(), user, routeKey, r.Header.Get("Idempotency-Key"), requestHash(r.Method, routeKey, body),
		auth.StudentRegistrationSettingUpdate{
			Enabled: *request.Enabled, ExpectedVersion: version,
			Reason: request.Reason, RequestID: requestIDFrom(r),
		}, studentRegistrationCompletionBuilder(),
	)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeIdempotencyCompletion(w, completion)
}

func (s *Server) handleAdminStudentInstitutionDomains(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	items, appErr := s.app.AdminStudentInstitutionDomains(r.Context(), user)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	response := make([]adminStudentInstitutionDomainResponse, 0, len(items))
	for _, item := range items {
		response = append(response, toAdminStudentInstitutionDomainResponse(item))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": response})
}

func (s *Server) handleCreateAdminStudentInstitutionDomain(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	body, request, appErr := decodeStrictJSON[adminStudentInstitutionDomainCreateRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	if request.Enabled == nil {
		writeProblem(w, r, studentAdminRequestError("enabled", "必须明确指定是否启用院校域名。"))
		return
	}
	version, appErr := requireIfMatchVersionAllowZero(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	if version != 0 {
		writeProblem(w, r, studentAdminRequestError("version", "新建院校域名必须使用 If-Match: \"0\"。"))
		return
	}
	const routeKey = "POST /api/v1/admin/student-institution-domains"
	completion, appErr := s.app.CreateStudentInstitutionDomainWithIdempotency(
		r.Context(), user, routeKey, r.Header.Get("Idempotency-Key"), requestHash(r.Method, routeKey, body),
		auth.StudentInstitutionDomainCreateInput{
			Domain: request.Domain, InstitutionName: request.InstitutionName, Enabled: *request.Enabled,
			Reason: request.Reason, RequestID: requestIDFrom(r),
		}, studentInstitutionDomainCompletionBuilder(http.StatusCreated),
	)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeIdempotencyCompletion(w, completion)
}

func (s *Server) handleUpdateAdminStudentInstitutionDomain(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	body, request, appErr := decodeStrictJSON[adminStudentInstitutionDomainUpdateRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	if request.Enabled == nil {
		writeProblem(w, r, studentAdminRequestError("enabled", "必须明确指定是否启用院校域名。"))
		return
	}
	version, appErr := requireIfMatchVersion(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	if request.ExpectedVersion != version {
		writeProblem(w, r, studentAdminRequestError("expectedVersion", "expectedVersion 必须与 If-Match 一致。"))
		return
	}
	id := chi.URLParam(r, "id")
	routeKey := "PATCH /api/v1/admin/student-institution-domains/{id}:" + id
	completion, appErr := s.app.UpdateStudentInstitutionDomainWithIdempotency(
		r.Context(), user, routeKey, r.Header.Get("Idempotency-Key"), requestHash(r.Method, routeKey, body),
		auth.StudentInstitutionDomainUpdateInput{
			ID: id, InstitutionName: request.InstitutionName, Enabled: *request.Enabled,
			ExpectedVersion: version, Reason: request.Reason, RequestID: requestIDFrom(r),
		}, studentInstitutionDomainCompletionBuilder(http.StatusOK),
	)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeIdempotencyCompletion(w, completion)
}

func studentRegistrationCompletionBuilder() auth.StudentRegistrationCompletionBuilder {
	return func(config auth.StudentRegistrationConfig) (idempotency.Completion, *domain.AppError) {
		body, err := json.Marshal(toAdminStudentRegistrationResponse(config))
		if err != nil {
			return idempotency.Completion{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "学生注册配置响应编码失败。")
		}
		return idempotency.Completion{Status: http.StatusOK, ContentType: "application/json; charset=utf-8", Body: body, ResourceType: "student_registration_setting", ResourceID: "00000000-0000-0000-0000-000000000091", Headers: map[string]string{"ETag": `"` + strconv.FormatInt(config.Version, 10) + `"`}}, nil
	}
}

func studentInstitutionDomainCompletionBuilder(status int) auth.StudentInstitutionDomainCompletionBuilder {
	return func(item auth.StudentInstitutionDomain) (idempotency.Completion, *domain.AppError) {
		body, err := json.Marshal(toAdminStudentInstitutionDomainResponse(item))
		if err != nil {
			return idempotency.Completion{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "院校域名响应编码失败。")
		}
		return idempotency.Completion{Status: status, ContentType: "application/json; charset=utf-8", Body: body, ResourceType: "student_institution_domain", ResourceID: item.ID, Headers: map[string]string{"ETag": `"` + strconv.FormatInt(item.Version, 10) + `"`}}, nil
	}
}

func toAdminStudentRegistrationResponse(config auth.StudentRegistrationConfig) adminStudentRegistrationResponse {
	return adminStudentRegistrationResponse{Enabled: config.Enabled, Version: config.Version}
}

func toAdminStudentInstitutionDomainResponse(item auth.StudentInstitutionDomain) adminStudentInstitutionDomainResponse {
	return adminStudentInstitutionDomainResponse{
		ID: item.ID, Domain: item.Domain, InstitutionName: item.InstitutionName,
		Enabled: item.Enabled, Version: item.Version,
		CreatedAt: item.CreatedAt.UTC().Format(timeLayoutRFC3339), UpdatedAt: item.UpdatedAt.UTC().Format(timeLayoutRFC3339),
	}
}

func studentAdminRequestError(field, detail string) *domain.AppError {
	return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Student registration validation failed", detail, field, "invalid", detail)
}
