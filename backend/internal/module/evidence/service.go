package evidence

import (
	"context"
	"errors"
	"net/http"
	"path"
	"strings"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/auth"
	"c2c-market/backend/internal/module/idempotency"

	"github.com/google/uuid"
)

const UnboundRetention = 24 * time.Hour

type Service struct {
	repo                     Repository
	objects                  ObjectStore
	now                      func() time.Time
	idempotency              *idempotency.Service
	transactionalIdempotency bool
	uploadEnabled            bool
}

func NewService(repo Repository, objects ObjectStore, now func() time.Time) *Service {
	return NewServiceWithUploadCapability(repo, objects, now, true)
}

func NewServiceWithUploadCapability(repo Repository, objects ObjectStore, now func() time.Time, uploadEnabled bool) *Service {
	if now == nil {
		now = time.Now
	}
	var idempotencyRepo idempotency.Repository
	if candidate, ok := repo.(idempotency.Repository); ok {
		idempotencyRepo = candidate
	}
	return &Service{
		repo: repo, objects: objects, now: now,
		idempotency:              idempotency.NewService(idempotencyRepo, now),
		transactionalIdempotency: idempotencyRepo != nil,
		uploadEnabled:            uploadEnabled,
	}
}

func (s *Service) Enabled() bool {
	return s != nil && s.repo != nil && s.objects != nil
}

func (s *Service) UploadEnabled() bool {
	return s.Enabled() && s.uploadEnabled
}

func (s *Service) Upload(ctx context.Context, input UploadInput) ([]PublicAsset, *domain.AppError) {
	if !s.UploadEnabled() {
		return nil, CapabilityUnavailableError()
	}
	input.APIOrderID = strings.TrimSpace(input.APIOrderID)
	input.UploaderUserID = strings.TrimSpace(input.UploaderUserID)
	input.Kind = strings.TrimSpace(input.Kind)
	if input.APIOrderID == "" || input.UploaderUserID == "" || !IsAllowedKind(input.Kind) {
		return nil, validationError("kind", "invalid", "请选择允许的证据类型。")
	}
	if len(input.Files) == 0 || len(input.Files) > MaxFilesPerUpload {
		return nil, validationError("files", "invalid_count", "每次必须上传 1 至 3 张图片。")
	}
	if !input.RedactionConfirmed {
		return nil, validationError("redactionConfirmed", "required", "请确认图片已遮挡凭据、完整账号和二维码。")
	}

	type prepared struct {
		asset Asset
		image ProcessedImage
	}
	now := s.now().UTC()
	ready := make([]prepared, 0, len(input.Files))
	for _, file := range input.Files {
		if file == nil {
			return nil, validationError("files", "invalid", "图片内容无效。")
		}
		processed, err := ProcessImage(file)
		if err != nil {
			return nil, imageValidationError(err)
		}
		id := uuid.NewString()
		expiresAt := now.Add(UnboundRetention)
		asset := Asset{
			ID: id, APIOrderID: input.APIOrderID, UploaderUserID: input.UploaderUserID,
			Kind: input.Kind, ObjectKey: objectKey(id, processed.MIME, now),
			OutputMIME: processed.MIME, ByteSize: int64(len(processed.Bytes)),
			Width: processed.Width, Height: processed.Height, SHA256: processed.SHA256,
			Status: "ready", CreatedAt: now, ReadyAt: &now, UnboundExpiresAt: &expiresAt, Version: 1,
		}
		ready = append(ready, prepared{asset: asset, image: processed})
	}

	written := make([]string, 0, len(ready))
	for _, item := range ready {
		if err := s.objects.Put(ctx, item.asset.ObjectKey, item.image.MIME, item.image.Bytes); err != nil {
			s.deleteObjectsBestEffort(ctx, written)
			return nil, domain.NewError(http.StatusServiceUnavailable, domain.CodeCapabilityUnavailable, "Evidence storage unavailable", "图片证据存储暂时不可用，请稍后重试。")
		}
		written = append(written, item.asset.ObjectKey)
	}
	assets := make([]Asset, 0, len(ready))
	for _, item := range ready {
		assets = append(assets, item.asset)
	}
	if appErr := s.repo.CreateReadyAssets(ctx, assets); appErr != nil {
		s.deleteObjectsBestEffort(ctx, written)
		return nil, appErr
	}
	result := make([]PublicAsset, 0, len(assets))
	for _, asset := range assets {
		result = append(result, PublicAsset{
			ID: asset.ID, Kind: asset.Kind, MIME: asset.OutputMIME,
			ByteSize: asset.ByteSize, Width: asset.Width, Height: asset.Height,
			CreatedAt: asset.CreatedAt, ContentPath: "/api/v1/me/dispute-evidence/" + asset.ID + "/content",
			Version: asset.Version,
		})
	}
	return result, nil
}

func (s *Service) AdminQuarantineWithIdempotency(
	ctx context.Context,
	user auth.User,
	routeKey, key, requestHash string,
	input AdminQuarantineInput,
	buildCompletion AdminQuarantineCompletionBuilder,
) (idempotency.Completion, *domain.AppError) {
	if !s.Enabled() {
		return idempotency.Completion{}, CapabilityUnavailableError()
	}
	if !user.IsAdmin {
		return idempotency.Completion{}, domain.NewError(http.StatusForbidden, domain.CodePermissionDenied, "Admin permission required", "需要管理员权限。")
	}
	input.AssetID = strings.TrimSpace(input.AssetID)
	input.AdminUserID = user.ID
	input.Reason = strings.TrimSpace(input.Reason)
	input.RequestID = strings.TrimSpace(input.RequestID)
	if _, err := uuid.Parse(input.AssetID); err != nil {
		return idempotency.Completion{}, validationError("id", "invalid", "图片证据 ID 无效。")
	}
	if input.ExpectedVersion < 1 {
		return idempotency.Completion{}, validationError("If-Match", "invalid", "图片证据版本无效。")
	}
	if len([]rune(input.Reason)) < 2 || len([]rune(input.Reason)) > 800 {
		return idempotency.Completion{}, validationError("reason", "invalid_length", "隔离原因需为 2 至 800 个字符。")
	}
	if strings.ContainsRune(input.Reason, '\x00') {
		return idempotency.Completion{}, validationError("reason", "control_character", "隔离原因包含非法字符。")
	}
	if domain.LooksLikeSecretContent(input.Reason) {
		return idempotency.Completion{}, domain.NewFieldError(
			http.StatusUnprocessableEntity,
			domain.CodeSecretContentDetected,
			"Secret content detected",
			"隔离原因不能包含凭据、对象路径或图片内容。",
			"reason",
			"secret_content",
			"请只填写不含敏感内容的违规类型和审核依据。",
		)
	}
	entry, appErr := s.idempotency.Begin(ctx, user.ID, routeKey, key, requestHash)
	if appErr != nil {
		return idempotency.Completion{}, appErr
	}
	if entry.State == "completed" {
		return idempotency.CompletionFromEntry(entry), nil
	}
	_, completion, appErr := s.repo.QuarantineAssetWithIdempotency(ctx, *entry, input, s.now().UTC(), buildCompletion)
	if appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	if !s.transactionalIdempotency {
		if appErr := s.idempotency.Complete(ctx, entry, completion.Status, completion.ContentType, completion.Body, completion.ResourceType, completion.ResourceID); appErr != nil {
			s.idempotency.Cancel(ctx, entry)
			return idempotency.Completion{}, appErr
		}
	}
	return completion, nil
}

func (s *Service) Open(ctx context.Context, assetID, viewerUserID string, admin bool) (Object, *domain.AppError) {
	if !s.Enabled() {
		return Object{}, CapabilityUnavailableError()
	}
	asset, appErr := s.repo.AuthorizedAsset(ctx, strings.TrimSpace(assetID), strings.TrimSpace(viewerUserID), admin)
	if appErr != nil {
		return Object{}, appErr
	}
	object, err := s.objects.Get(ctx, asset.ObjectKey)
	if errors.Is(err, ErrObjectNotFound) {
		return Object{}, domain.NewError(http.StatusGone, domain.CodeObjectNotFound, "Evidence content unavailable", "该证据图片已按保留规则销毁或存储内容缺失。")
	}
	if err != nil {
		return Object{}, domain.NewError(http.StatusServiceUnavailable, domain.CodeCapabilityUnavailable, "Evidence storage unavailable", "图片证据暂时无法读取，请稍后重试。")
	}
	return object, nil
}

func (s *Service) Cleanup(ctx context.Context, batchSize int) (CleanupResult, *domain.AppError) {
	if !s.Enabled() {
		return CleanupResult{}, nil
	}
	if batchSize < 1 {
		return CleanupResult{}, validationError("batchSize", "invalid", "清理批次必须大于零。")
	}
	candidates, appErr := s.repo.ClaimDestroyCandidates(ctx, s.now().UTC(), batchSize)
	if appErr != nil {
		return CleanupResult{}, appErr
	}
	result := CleanupResult{Claimed: len(candidates)}
	for _, candidate := range candidates {
		if err := s.objects.Delete(ctx, candidate.ObjectKey); err != nil {
			result.Failed++
			continue
		}
		if appErr := s.repo.MarkDestroyed(ctx, candidate.ID, s.now().UTC()); appErr != nil {
			result.Failed++
			continue
		}
		result.Destroyed++
	}
	return result, nil
}

func (s *Service) deleteObjectsBestEffort(ctx context.Context, keys []string) {
	for _, key := range keys {
		_ = s.objects.Delete(ctx, key)
	}
}

func objectKey(id, mime string, now time.Time) string {
	extension := ".png"
	if mime == "image/jpeg" {
		extension = ".jpg"
	}
	return path.Join("api-order-evidence", now.Format("2006/01"), id+extension)
}

func imageValidationError(err error) *domain.AppError {
	message := "图片必须是 JPEG、PNG 或 WebP，单张不超过 5 MiB，尺寸不超过 4096 x 4096。"
	if errors.Is(err, ErrQRCodeDetected) {
		message = "图片中检测到二维码。请移除收款、登录、MFA 或恢复码二维码后重试。"
	}
	return validationError("files", "invalid_image", message)
}

func validationError(field, code, message string) *domain.AppError {
	return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Invalid evidence", message, field, code, message)
}

func CapabilityUnavailableError() *domain.AppError {
	return domain.NewError(http.StatusServiceUnavailable, domain.CodeCapabilityUnavailable, "Evidence capability unavailable", "图片证据功能暂未启用；文本纠纷仍可正常提交。")
}
