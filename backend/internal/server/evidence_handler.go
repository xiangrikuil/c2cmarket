package server

import (
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/evidence"
	"c2c-market/backend/internal/module/idempotency"

	"github.com/go-chi/chi/v5"
)

const evidenceMultipartOverhead int64 = 1 * 1024 * 1024

type evidenceUploadResponse struct {
	Items []evidence.PublicAsset `json:"items"`
}

type evidenceQuarantineRequest struct {
	Reason string `json:"reason"`
}

func (s *Server) handleUploadAPIOrderDisputeEvidence(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	if s.evidence == nil || !s.evidence.UploadEnabled() {
		writeProblem(w, r, evidence.CapabilityUnavailableError())
		return
	}

	maxBodyBytes := int64(evidence.MaxFilesPerUpload)*evidence.MaxFileBytes + evidenceMultipartOverhead
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	if err := r.ParseMultipartForm(maxBodyBytes); err != nil {
		writeProblem(w, r, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Invalid evidence upload", "图片上传格式无效或请求体过大。", "files", "invalid_multipart", "请选择 1 至 3 张、单张不超过 5 MiB 的图片。"))
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	if !validEvidenceMultipartFields(r.MultipartForm) {
		writeProblem(w, r, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Invalid evidence upload", "图片上传包含未支持或重复的字段。", "files", "unknown_part", "只允许单个 kind、单个 redactionConfirmed 和 files 字段。"))
		return
	}

	files := multipartFiles(r.MultipartForm)
	if len(files) == 0 || len(files) > evidence.MaxFilesPerUpload {
		writeProblem(w, r, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Invalid evidence upload", "每次必须上传 1 至 3 张图片。", "files", "invalid_count", "每次必须上传 1 至 3 张图片。"))
		return
	}
	readers := make([]io.Reader, 0, len(files))
	opened := make([]multipart.File, 0, len(files))
	defer func() {
		for _, file := range opened {
			_ = file.Close()
		}
	}()
	for _, header := range files {
		file, err := header.Open()
		if err != nil {
			writeProblem(w, r, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Invalid evidence upload", "图片内容无法读取。", "files", "unreadable", "请重新选择图片后重试。"))
			return
		}
		opened = append(opened, file)
		readers = append(readers, file)
	}

	assets, appErr := s.evidence.Upload(r.Context(), evidence.UploadInput{
		APIOrderID:         chi.URLParam(r, "id"),
		UploaderUserID:     user.ID,
		Kind:               r.FormValue("kind"),
		Files:              readers,
		RedactionConfirmed: strings.TrimSpace(r.FormValue("redactionConfirmed")) == "true",
	})
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	writeJSON(w, http.StatusCreated, evidenceUploadResponse{Items: assets})
}

func multipartFiles(form *multipart.Form) []*multipart.FileHeader {
	if form == nil {
		return nil
	}
	return append([]*multipart.FileHeader(nil), form.File["files"]...)
}

func validEvidenceMultipartFields(form *multipart.Form) bool {
	if form == nil {
		return false
	}
	for name := range form.Value {
		if name != "kind" && name != "redactionConfirmed" {
			return false
		}
	}
	for name := range form.File {
		if name != "files" {
			return false
		}
	}
	return len(form.Value["kind"]) == 1 && len(form.Value["redactionConfirmed"]) <= 1
}

func (s *Server) handleMyDisputeEvidenceContent(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	s.streamDisputeEvidence(w, r, chi.URLParam(r, "id"), user.ID, false)
}

func (s *Server) handleAdminDisputeEvidenceContent(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	if !user.IsAdmin {
		writeProblem(w, r, domain.NewError(http.StatusForbidden, domain.CodePermissionDenied, "Admin permission required", "需要管理员权限。"))
		return
	}
	s.streamDisputeEvidence(w, r, chi.URLParam(r, "id"), user.ID, true)
}

func (s *Server) handleAdminQuarantineDisputeEvidence(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	user, _, appErr := s.requireSessionAndCSRF(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	if !user.IsAdmin {
		writeProblem(w, r, domain.NewError(http.StatusForbidden, domain.CodePermissionDenied, "Admin permission required", "需要管理员权限。"))
		return
	}
	if s.evidence == nil || !s.evidence.Enabled() {
		writeProblem(w, r, evidence.CapabilityUnavailableError())
		return
	}
	body, request, appErr := decodeStrictJSON[evidenceQuarantineRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	version, appErr := requireIfMatchVersion(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	assetID := chi.URLParam(r, "id")
	routeKey := "POST /api/v1/admin/dispute-evidence/{id}/quarantine:" + assetID
	completion, appErr := s.evidence.AdminQuarantineWithIdempotency(
		r.Context(),
		user,
		routeKey,
		r.Header.Get("Idempotency-Key"),
		requestHash(r.Method, routeKey, body),
		evidence.AdminQuarantineInput{
			AssetID:         assetID,
			ExpectedVersion: version,
			Reason:          request.Reason,
			RequestID:       requestIDFrom(r),
		},
		evidenceQuarantineCompletionBuilder,
	)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	restoreEvidenceQuarantineETag(&completion)
	writeNoStoreIdempotencyCompletion(w, completion)
}

func evidenceQuarantineCompletionBuilder(result evidence.AdminQuarantineResult) (idempotency.Completion, *domain.AppError) {
	body, err := json.Marshal(result)
	if err != nil {
		return idempotency.Completion{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "图片证据隔离响应编码失败。")
	}
	return idempotency.Completion{
		Status:       http.StatusOK,
		ContentType:  "application/json; charset=utf-8",
		Body:         body,
		ResourceType: "api_order_evidence",
		ResourceID:   result.ID,
		Headers: map[string]string{
			"ETag": `"` + strconv.FormatInt(result.Version, 10) + `"`,
		},
	}, nil
}

func restoreEvidenceQuarantineETag(completion *idempotency.Completion) {
	if completion == nil || len(completion.Body) == 0 || completion.Headers != nil && completion.Headers["ETag"] != "" {
		return
	}
	var result evidence.AdminQuarantineResult
	if err := json.Unmarshal(completion.Body, &result); err != nil || result.Version < 1 {
		return
	}
	if completion.Headers == nil {
		completion.Headers = make(map[string]string)
	}
	completion.Headers["ETag"] = `"` + strconv.FormatInt(result.Version, 10) + `"`
}

func (s *Server) streamDisputeEvidence(w http.ResponseWriter, r *http.Request, assetID, viewerUserID string, admin bool) {
	if s.evidence == nil || !s.evidence.Enabled() {
		writeProblem(w, r, evidence.CapabilityUnavailableError())
		return
	}
	object, appErr := s.evidence.Open(r.Context(), assetID, viewerUserID, admin)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	defer object.Body.Close()
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Disposition", "inline; filename=dispute-evidence")
	w.Header().Set("Content-Type", object.ContentType)
	if object.Size > 0 {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", object.Size))
	}
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, object.Body)
}
