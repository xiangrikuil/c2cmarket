package catalog

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/auth"
	"c2c-market/backend/internal/module/idempotency"
	"c2c-market/backend/internal/platform/modelsdev"

	"github.com/google/uuid"
)

var modelsDevProviderAllowlist = map[string]bool{
	"openai": true, "anthropic": true, "xai": true,
}

var unsupportedModelsDevKeyParts = []string{
	"embedding", "realtime", "audio", "tts", "transcribe", "whisper",
	"image", "dall-e", "sora", "veo", "video", "moderation",
}

func (s *Service) PreviewAPIModelSync(ctx context.Context, user auth.User, input APIModelSyncPreviewInput) (APIModelSyncPreview, *domain.AppError) {
	if appErr := requireAdmin(user); appErr != nil {
		return APIModelSyncPreview{}, appErr
	}
	providers, appErr := s.AdminAPIModelProviders(ctx, user)
	if appErr != nil {
		return APIModelSyncPreview{}, appErr
	}
	selected, appErr := selectedModelsDevProviders(providers, input.ProviderIDs)
	if appErr != nil {
		return APIModelSyncPreview{}, appErr
	}
	models, appErr := s.AdminAPIModels(ctx, user)
	if appErr != nil {
		return APIModelSyncPreview{}, appErr
	}
	s.mu.Lock()
	source := s.modelsDev
	s.mu.Unlock()
	if source == nil {
		return APIModelSyncPreview{}, modelsDevUnavailableError()
	}
	external, err := source.Fetch(ctx)
	if err != nil {
		if errors.Is(err, modelsdev.ErrInvalidData) {
			return APIModelSyncPreview{}, domain.NewError(http.StatusBadGateway, domain.CodeExternalSourceUnavailable, "Invalid models.dev response", "models.dev 返回的数据格式暂时无法识别，请稍后重试。")
		}
		return APIModelSyncPreview{}, modelsDevUnavailableError()
	}
	fetchedAt := s.now().UTC()
	preview := buildAPIModelSyncPreview(selected, models, external, fetchedAt)
	return preview, nil
}

func (s *Service) ApplyAPIModelSyncWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input APIModelSyncApplyInput, buildCompletion APIModelSyncCompletionBuilder) (idempotency.Completion, *domain.AppError) {
	if appErr := requireAdmin(user); appErr != nil {
		return idempotency.Completion{}, appErr
	}
	items, appErr := normalizeAPIModelSyncSelections(input.Items)
	if appErr != nil {
		return idempotency.Completion{}, appErr
	}
	if s.idempotency == nil || buildCompletion == nil {
		return idempotency.Completion{}, internalCatalogError()
	}
	entry, appErr := s.idempotency.Begin(ctx, user.ID, routeKey, strings.TrimSpace(key), requestHash)
	if appErr != nil {
		return idempotency.Completion{}, appErr
	}
	if entry.State == "completed" {
		return idempotency.CompletionFromEntry(entry), nil
	}
	mutation := APIModelSyncMutationInput{OperatorID: user.ID, Items: items}
	if s.repo != nil {
		_, completion, appErr := s.repo.AdminApplyAPIModelSyncWithIdempotency(ctx, *entry, mutation, s.now().UTC(), buildCompletion)
		if appErr != nil {
			s.idempotency.Cancel(ctx, entry)
			return idempotency.Completion{}, appErr
		}
		return completion, nil
	}
	result, appErr := s.applyAPIModelSyncInMemory(mutation)
	if appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	completion, appErr := buildCompletion(result)
	if appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	if appErr := s.idempotency.Complete(ctx, entry, completion.Status, completion.ContentType, completion.Body, completion.ResourceType, completion.ResourceID); appErr != nil {
		return idempotency.Completion{}, appErr
	}
	return completion, nil
}

func buildAPIModelSyncPreview(providers []APIModelProvider, localModels []APIModelCatalog, external modelsdev.Catalog, fetchedAt time.Time) APIModelSyncPreview {
	localByKey := make(map[string]APIModelCatalog, len(localModels))
	for _, model := range localModels {
		localByKey[model.ModelKey] = model
	}
	items := make([]APIModelSyncItem, 0)
	for _, provider := range providers {
		externalProvider, ok := externalProviderByCode(external, provider.Code)
		if !ok {
			items = append(items, unavailableProviderItem(provider))
			for _, local := range localModels {
				if local.ProviderID == provider.ID {
					items = append(items, sourceMissingItem(local))
				}
			}
			continue
		}
		rawKeys := make(map[string]bool, len(externalProvider.Models))
		for mapKey, externalModel := range externalProvider.Models {
			modelKey := strings.TrimSpace(externalModel.ID)
			if modelKey == "" {
				modelKey = strings.TrimSpace(mapKey)
			}
			if modelKey != "" {
				rawKeys[modelKey] = true
			}
			item, include := apiModelSyncItemFromExternal(provider, modelKey, externalModel, fetchedAt)
			if !include {
				continue
			}
			if local, exists := localByKey[modelKey]; exists {
				if local.ProviderID != provider.ID {
					item.Status = APIModelSyncStatusUnavailable
					item.ReasonCode = "MODEL_KEY_PROVIDER_CONFLICT"
					item.Reason = "该模型标识已属于其他提供商。"
				} else {
					applyLocalModelToSyncItem(&item, local)
					if apiModelSyncPricePayloadChanged(local, item) {
						item.Status = APIModelSyncStatusPriceChanged
					} else {
						item.Status = APIModelSyncStatusUnchanged
					}
				}
			}
			item.Fingerprint = apiModelSyncSelectionFingerprint(selectionFromPreviewItem(item))
			items = append(items, item)
		}
		for _, local := range localModels {
			if local.ProviderID == provider.ID && !rawKeys[local.ModelKey] {
				items = append(items, sourceMissingItem(local))
			}
		}
	}
	sortAPIModelSyncItems(items)
	preview := APIModelSyncPreview{FetchedAt: fetchedAt, Items: items}
	for _, item := range items {
		switch item.Status {
		case APIModelSyncStatusNew:
			preview.Counts.New++
		case APIModelSyncStatusPriceChanged:
			preview.Counts.PriceChanged++
		case APIModelSyncStatusUnchanged:
			preview.Counts.Unchanged++
		case APIModelSyncStatusSourceMissing:
			preview.Counts.SourceMissing++
		case APIModelSyncStatusUnavailable:
			preview.Counts.Unavailable++
		}
	}
	preview.Fingerprint = previewFingerprint(items)
	return preview
}

func apiModelSyncItemFromExternal(provider APIModelProvider, modelKey string, model modelsdev.Model, fetchedAt time.Time) (APIModelSyncItem, bool) {
	item := APIModelSyncItem{
		CandidateKey: provider.Code + ":" + modelKey,
		Status:       APIModelSyncStatusNew,
		ProviderID:   provider.ID,
		ProviderCode: provider.Code,
		Provider:     provider.DisplayName,
		ModelKey:     modelKey,
		SourceURL:    modelsdev.SourceURL,
	}
	if !supportedModelsDevModel(modelKey, model) {
		return APIModelSyncItem{}, false
	}
	if modelKey == "" || len(modelKey) > 120 {
		item.Status = APIModelSyncStatusUnavailable
		item.ReasonCode = "INVALID_MODEL_KEY"
		item.Reason = "来源模型标识为空或过长。"
		return item, true
	}
	capabilities := map[string]bool{"text": true}
	if model.Reasoning {
		capabilities["reasoning"] = true
	}
	if model.Attachment || stringSliceContains(model.Modalities.Input, "image") {
		capabilities["vision"] = true
	}
	for _, capability := range apiModelCapabilityOrder {
		if capabilities[capability] {
			item.Capabilities = append(item.Capabilities, capability)
		}
	}
	if model.Cost == nil {
		item.Status = APIModelSyncStatusUnavailable
		item.ReasonCode = "PRICE_MISSING"
		item.Reason = "来源未提供可映射的 token 价格。"
		return item, true
	}
	var appErr *domain.AppError
	item.InputPricePerMillion, appErr = normalizeAPIModelPrice("inputTokenPrice", model.Cost.Input.String())
	if appErr == nil {
		item.CachedInputPricePerMillion, appErr = normalizeAPIModelPrice("cachedInputTokenPrice", model.Cost.CacheRead.String())
	}
	if appErr == nil {
		item.OutputPricePerMillion, appErr = normalizeAPIModelPrice("outputTokenPrice", model.Cost.Output.String())
	}
	if appErr != nil || item.InputPricePerMillion == "" && item.OutputPricePerMillion == "" {
		item.Status = APIModelSyncStatusUnavailable
		item.ReasonCode = "PRICE_INVALID"
		item.Reason = "来源价格缺失或格式不受支持。"
		return item, true
	}
	lastUpdated := strings.TrimSpace(model.LastUpdated)
	if lastUpdated == "" {
		lastUpdated = fetchedAt.Format("2006-01-02")
	}
	item.SourceVersion = "models.dev:" + lastUpdated
	return item, true
}

func supportedModelsDevModel(modelKey string, model modelsdev.Model) bool {
	lowerKey := strings.ToLower(modelKey)
	for _, part := range unsupportedModelsDevKeyParts {
		if strings.Contains(lowerKey, part) {
			return false
		}
	}
	if !stringSliceContains(model.Modalities.Input, "text") || !stringSliceContains(model.Modalities.Output, "text") {
		return false
	}
	for _, output := range model.Modalities.Output {
		switch strings.ToLower(strings.TrimSpace(output)) {
		case "image", "audio", "video":
			return false
		}
	}
	return true
}

func selectedModelsDevProviders(providers []APIModelProvider, providerIDs []string) ([]APIModelProvider, *domain.AppError) {
	if len(providerIDs) == 0 {
		return nil, fieldError(http.StatusUnprocessableEntity, "providerIds", "请至少选择一个官方提供商。")
	}
	byID := make(map[string]APIModelProvider, len(providers))
	for _, provider := range providers {
		byID[provider.ID] = provider
	}
	selected := make([]APIModelProvider, 0, len(providerIDs))
	seen := make(map[string]bool, len(providerIDs))
	for _, id := range providerIDs {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		provider, ok := byID[id]
		if !ok || !modelsDevProviderAllowlist[provider.Code] {
			return nil, fieldError(http.StatusUnprocessableEntity, "providerIds", "所选提供商暂不支持 models.dev 同步。")
		}
		seen[id] = true
		selected = append(selected, provider)
	}
	if len(selected) == 0 || len(selected) > len(modelsDevProviderAllowlist) {
		return nil, fieldError(http.StatusUnprocessableEntity, "providerIds", "请选择有效的官方提供商。")
	}
	sort.Slice(selected, func(i, j int) bool { return selected[i].SortOrder < selected[j].SortOrder })
	return selected, nil
}

func normalizeAPIModelSyncSelections(items []APIModelSyncSelection) ([]APIModelSyncSelection, *domain.AppError) {
	if len(items) == 0 || len(items) > 200 {
		return nil, fieldError(http.StatusUnprocessableEntity, "items", "请选择 1 到 200 个可应用的模型变化。")
	}
	normalized := make([]APIModelSyncSelection, 0, len(items))
	seen := make(map[string]bool, len(items))
	for _, item := range items {
		item.ProviderID = strings.TrimSpace(item.ProviderID)
		item.ProviderCode = strings.TrimSpace(item.ProviderCode)
		item.ModelKey = strings.TrimSpace(item.ModelKey)
		item.SourceURL = strings.TrimSpace(item.SourceURL)
		item.SourceVersion = strings.TrimSpace(item.SourceVersion)
		item.LocalModelID = strings.TrimSpace(item.LocalModelID)
		item.LocalPriceVersionID = strings.TrimSpace(item.LocalPriceVersionID)
		if item.Status != APIModelSyncStatusNew && item.Status != APIModelSyncStatusPriceChanged {
			return nil, fieldError(http.StatusUnprocessableEntity, "items", "只能应用新增模型或价格变化。")
		}
		if !modelsDevProviderAllowlist[item.ProviderCode] || item.ProviderID == "" || item.ModelKey == "" || item.SourceURL != modelsdev.SourceURL || !strings.HasPrefix(item.SourceVersion, "models.dev:") {
			return nil, fieldError(http.StatusUnprocessableEntity, "items", "同步候选项来源无效。")
		}
		if item.Status == APIModelSyncStatusNew && item.LocalModelID != "" || item.Status == APIModelSyncStatusPriceChanged && item.LocalModelID == "" {
			return nil, fieldError(http.StatusUnprocessableEntity, "items", "同步候选项与本地模型状态不一致。")
		}
		form, appErr := normalizeAPIModelInput(APIModelInput{
			ProviderID: item.ProviderID, ModelKey: item.ModelKey, Capabilities: item.Capabilities,
			SourceURL: item.SourceURL, SourceVersion: item.SourceVersion,
			InputTokenPrice: item.InputPricePerMillion, CachedInputTokenPrice: item.CachedInputPricePerMillion,
			OutputTokenPrice: item.OutputPricePerMillion,
		})
		if appErr != nil {
			return nil, appErr
		}
		item.ProviderID = form.ProviderID
		item.ModelKey = form.ModelKey
		item.Capabilities = form.Capabilities
		item.SourceURL = form.SourceURL
		item.SourceVersion = form.SourceVersion
		item.InputPricePerMillion = form.InputTokenPrice
		item.CachedInputPricePerMillion = form.CachedInputTokenPrice
		item.OutputPricePerMillion = form.OutputTokenPrice
		if item.Fingerprint != apiModelSyncSelectionFingerprint(item) {
			return nil, fieldError(http.StatusUnprocessableEntity, "items", "同步候选项校验失败，请重新获取预览。")
		}
		key := item.ProviderCode + ":" + item.ModelKey
		if seen[key] {
			return nil, fieldError(http.StatusUnprocessableEntity, "items", "同步候选项包含重复模型。")
		}
		seen[key] = true
		normalized = append(normalized, item)
	}
	return normalized, nil
}

func (s *Service) applyAPIModelSyncInMemory(input APIModelSyncMutationInput) (APIModelBulkMutationResult, *domain.AppError) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, item := range input.Items {
		provider, ok := s.apiProviders[item.ProviderID]
		if !ok || provider.Code != item.ProviderCode {
			return APIModelBulkMutationResult{}, fieldError(http.StatusUnprocessableEntity, "items", "同步候选项的提供商已变化。")
		}
		if item.Status == APIModelSyncStatusNew {
			if s.apiModelKeyExistsLocked("", item.ModelKey) {
				return APIModelBulkMutationResult{}, domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Model catalog changed", "模型目录已变化，请重新获取同步预览。")
			}
			continue
		}
		model, ok := s.apiModels[item.LocalModelID]
		if !ok || model.ModelKey != item.ModelKey || model.ProviderID != item.ProviderID || model.CurrentPriceVersionID != item.LocalPriceVersionID {
			return APIModelBulkMutationResult{}, domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Model price changed", "模型价格已变化，请重新获取同步预览。")
		}
	}
	result := APIModelBulkMutationResult{IDs: make([]string, 0, len(input.Items))}
	now := s.now().UTC()
	nextSort := s.nextAPIModelSortOrderLocked()
	for _, item := range input.Items {
		if item.Status == APIModelSyncStatusNew {
			lifecycle := activeLifecycle("")
			lifecycle.Status = lifecycleStatusFromActive(item.Active)
			lifecycle.EffectiveStatus = lifecycle.Status
			model := APIModelCatalog{
				Lifecycle: lifecycle,
				ID:        uuid.NewString(), ProviderID: item.ProviderID, ProviderCode: item.ProviderCode,
				Provider: s.apiProviders[item.ProviderID].DisplayName, ProviderCategory: s.apiProviders[item.ProviderID].ProviderCategory,
				ProviderActive: s.apiProviders[item.ProviderID].Active, ModelKey: item.ModelKey,
				Capabilities: append([]string(nil), item.Capabilities...), Active: item.Active, SortOrder: nextSort,
				CurrentPriceVersionID: uuid.NewString(), CurrentPriceSourceURL: item.SourceURL,
				CurrentPriceSourceVersion: item.SourceVersion, CurrentPriceValidFrom: &now,
				InputPricePerMillion: item.InputPricePerMillion, CachedInputPricePerMillion: item.CachedInputPricePerMillion,
				OutputPricePerMillion: item.OutputPricePerMillion, CreatedAt: now, UpdatedAt: now,
			}
			s.apiModels[model.ID] = model
			s.apiModelOrder = append(s.apiModelOrder, model.ID)
			result.Created++
			result.IDs = append(result.IDs, model.ID)
			nextSort += 10
			continue
		}
		model := s.apiModels[item.LocalModelID]
		model.CurrentPriceVersionID = uuid.NewString()
		model.CurrentPriceSourceURL = item.SourceURL
		model.CurrentPriceSourceVersion = item.SourceVersion
		model.CurrentPriceValidFrom = &now
		model.InputPricePerMillion = item.InputPricePerMillion
		model.CachedInputPricePerMillion = item.CachedInputPricePerMillion
		model.OutputPricePerMillion = item.OutputPricePerMillion
		model.UpdatedAt = now
		s.apiModels[model.ID] = model
		result.Updated++
		result.IDs = append(result.IDs, model.ID)
	}
	return result, nil
}

func (s *Service) nextAPIModelSortOrderLocked() int {
	maxOrder := 0
	for _, model := range s.apiModels {
		if model.SortOrder > maxOrder {
			maxOrder = model.SortOrder
		}
	}
	return (maxOrder/10 + 1) * 10
}

func externalProviderByCode(catalog modelsdev.Catalog, code string) (modelsdev.Provider, bool) {
	if provider, ok := catalog[code]; ok {
		return provider, true
	}
	for key, provider := range catalog {
		if strings.EqualFold(strings.TrimSpace(provider.ID), code) || strings.EqualFold(strings.TrimSpace(key), code) {
			return provider, true
		}
	}
	return modelsdev.Provider{}, false
}

func apiModelSyncPricePayloadChanged(local APIModelCatalog, item APIModelSyncItem) bool {
	return local.CurrentPriceSourceURL != item.SourceURL ||
		local.CurrentPriceSourceVersion != item.SourceVersion ||
		local.InputPricePerMillion != item.InputPricePerMillion ||
		local.CachedInputPricePerMillion != item.CachedInputPricePerMillion ||
		local.OutputPricePerMillion != item.OutputPricePerMillion
}

func applyLocalModelToSyncItem(item *APIModelSyncItem, local APIModelCatalog) {
	item.LocalModelID = local.ID
	item.LocalPriceVersionID = local.CurrentPriceVersionID
	item.LocalInputPricePerMillion = local.InputPricePerMillion
	item.LocalCachedInputPricePerMillion = local.CachedInputPricePerMillion
	item.LocalOutputPricePerMillion = local.OutputPricePerMillion
	item.LocalSourceURL = local.CurrentPriceSourceURL
	item.LocalSourceVersion = local.CurrentPriceSourceVersion
}

func sourceMissingItem(local APIModelCatalog) APIModelSyncItem {
	item := APIModelSyncItem{
		CandidateKey: local.ProviderCode + ":" + local.ModelKey, Status: APIModelSyncStatusSourceMissing,
		ReasonCode: "SOURCE_MISSING", Reason: "models.dev 本次未返回该模型，本地数据保持不变。",
		ProviderID: local.ProviderID, ProviderCode: local.ProviderCode, Provider: local.Provider,
		ModelKey: local.ModelKey, Capabilities: append([]string(nil), local.Capabilities...),
	}
	applyLocalModelToSyncItem(&item, local)
	return item
}

func unavailableProviderItem(provider APIModelProvider) APIModelSyncItem {
	return APIModelSyncItem{
		CandidateKey: provider.Code + ":provider", Status: APIModelSyncStatusUnavailable,
		ReasonCode: "PROVIDER_MISSING", Reason: "models.dev 本次未返回该提供商。",
		ProviderID: provider.ID, ProviderCode: provider.Code, Provider: provider.DisplayName,
	}
}

func selectionFromPreviewItem(item APIModelSyncItem) APIModelSyncSelection {
	return APIModelSyncSelection{
		Status: item.Status, ProviderID: item.ProviderID, ProviderCode: item.ProviderCode,
		ModelKey: item.ModelKey, Capabilities: append([]string(nil), item.Capabilities...),
		SourceURL: item.SourceURL, SourceVersion: item.SourceVersion,
		InputPricePerMillion: item.InputPricePerMillion, CachedInputPricePerMillion: item.CachedInputPricePerMillion,
		OutputPricePerMillion: item.OutputPricePerMillion, LocalModelID: item.LocalModelID,
		LocalPriceVersionID: item.LocalPriceVersionID,
	}
}

func apiModelSyncSelectionFingerprint(item APIModelSyncSelection) string {
	canonical := struct {
		Status, ProviderID, ProviderCode, ModelKey, SourceURL, SourceVersion string
		Capabilities                                                         []string
		Input, CachedInput, Output, LocalModelID, LocalPriceVersionID        string
	}{
		item.Status, item.ProviderID, item.ProviderCode, item.ModelKey, item.SourceURL, item.SourceVersion,
		append([]string(nil), item.Capabilities...), item.InputPricePerMillion, item.CachedInputPricePerMillion,
		item.OutputPricePerMillion, item.LocalModelID, item.LocalPriceVersionID,
	}
	payload, _ := json.Marshal(canonical)
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func previewFingerprint(items []APIModelSyncItem) string {
	hash := sha256.New()
	for _, item := range items {
		hash.Write([]byte(item.CandidateKey))
		hash.Write([]byte{0})
		hash.Write([]byte(item.Fingerprint))
		hash.Write([]byte{0})
		hash.Write([]byte(item.Status))
		hash.Write([]byte{0})
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func sortAPIModelSyncItems(items []APIModelSyncItem) {
	statusOrder := map[string]int{
		APIModelSyncStatusNew: 0, APIModelSyncStatusPriceChanged: 1, APIModelSyncStatusUnchanged: 2,
		APIModelSyncStatusSourceMissing: 3, APIModelSyncStatusUnavailable: 4,
	}
	sort.Slice(items, func(i, j int) bool {
		if statusOrder[items[i].Status] != statusOrder[items[j].Status] {
			return statusOrder[items[i].Status] < statusOrder[items[j].Status]
		}
		if items[i].ProviderCode != items[j].ProviderCode {
			return items[i].ProviderCode < items[j].ProviderCode
		}
		return items[i].ModelKey < items[j].ModelKey
	})
}

func stringSliceContains(values []string, expected string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), expected) {
			return true
		}
	}
	return false
}

func modelsDevUnavailableError() *domain.AppError {
	return domain.NewError(http.StatusBadGateway, domain.CodeExternalSourceUnavailable, "models.dev unavailable", "models.dev 暂时不可用，请稍后重试。")
}

func internalCatalogError() *domain.AppError {
	return domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "API 模型目录服务暂时不可用。")
}
