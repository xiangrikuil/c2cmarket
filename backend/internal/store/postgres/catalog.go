package postgres

import (
	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/catalog"
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"net/http"
	"strings"
	"time"
)

func (s *Store) ListProductCategories(ctx context.Context) ([]catalog.ProductCategory, *domain.AppError) {
	if s == nil || s.pool == nil {
		return nil, internalStoreError()
	}
	rows, err := s.pool.Query(ctx, `
			SELECT `+productCategoryColumns+`
				FROM product_categories
				WHERE status = 'active'
			ORDER BY sort_order ASC, display_name ASC
		`)
	if err != nil {
		return nil, internalStoreError()
	}
	defer rows.Close()

	categories := []catalog.ProductCategory{}
	for rows.Next() {
		var category catalog.ProductCategory
		if err := scanProductCategory(rows, &category); err != nil {
			return nil, internalStoreError()
		}
		categories = append(categories, category)
	}
	if err := rows.Err(); err != nil {
		return nil, internalStoreError()
	}
	return categories, nil
}

func (s *Store) GetProductCategory(ctx context.Context, categoryID string) (catalog.ProductCategory, *domain.AppError) {
	if s == nil || s.pool == nil {
		return catalog.ProductCategory{}, internalStoreError()
	}
	var category catalog.ProductCategory
	err := scanProductCategory(s.pool.QueryRow(ctx, `
			SELECT `+productCategoryColumns+`
		FROM product_categories
		WHERE id = $1
	`, categoryID), &category)
	if errors.Is(err, pgx.ErrNoRows) {
		return catalog.ProductCategory{}, productCategoryNotFound()
	}
	if err != nil {
		return catalog.ProductCategory{}, internalStoreError()
	}
	return category, nil
}

func (s *Store) AdminListProductCategories(ctx context.Context) ([]catalog.ProductCategory, *domain.AppError) {
	if s == nil || s.pool == nil {
		return nil, internalStoreError()
	}
	rows, err := s.pool.Query(ctx, `
			SELECT `+productCategoryColumns+`
		FROM product_categories
		ORDER BY sort_order ASC, display_name ASC
	`)
	if err != nil {
		return nil, internalStoreError()
	}
	defer rows.Close()
	return scanProductCategories(rows)
}

func (s *Store) AdminGetProductCategory(ctx context.Context, categoryID string) (catalog.ProductCategory, *domain.AppError) {
	return s.GetProductCategory(ctx, categoryID)
}

func (s *Store) AdminCreateProductCategory(ctx context.Context, input catalog.ProductCategoryMutationInput) (catalog.ProductCategory, *domain.AppError) {
	if s == nil || s.pool == nil {
		return catalog.ProductCategory{}, internalStoreError()
	}
	var category catalog.ProductCategory
	err := scanProductCategory(s.pool.QueryRow(ctx, `
			INSERT INTO product_categories (code, display_name, icon_data_url, sort_order, status)
			VALUES ($1, $2, $3, $4, 'active')
			RETURNING `+productCategoryReturningColumns+`
		`, input.Form.Code, input.Form.DisplayName, input.Form.IconDataURL, input.Form.SortOrder), &category)
	if err != nil {
		if isUniqueViolation(err) {
			return catalog.ProductCategory{}, domain.NewFieldError(http.StatusConflict, domain.CodeValidationFailed, "Product category code unavailable", "分类 code 已被占用。", "code", "unavailable", "分类 code 已被占用。")
		}
		return catalog.ProductCategory{}, internalStoreError()
	}
	return category, nil
}

func (s *Store) AdminUpdateProductCategory(ctx context.Context, input catalog.ProductCategoryMutationInput) (catalog.ProductCategory, *domain.AppError) {
	if s == nil || s.pool == nil {
		return catalog.ProductCategory{}, internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return catalog.ProductCategory{}, internalStoreError()
	}
	defer rollback(ctx, tx)

	var currentCode, lockReason string
	var identityLocked bool
	err = tx.QueryRow(ctx, `
		SELECT code,
		       (core_key IS NOT NULL OR EXISTS (SELECT 1 FROM product_plans ref WHERE ref.category_id = product_categories.id)),
		       CASE
		         WHEN core_key IS NOT NULL THEN '核心目录身份由系统管理，不能修改。'
		         WHEN EXISTS (SELECT 1 FROM product_plans ref WHERE ref.category_id = product_categories.id) THEN '分类已被套餐引用，身份字段不能修改。'
		         ELSE ''
		       END
		FROM product_categories
		WHERE id = $1
		FOR UPDATE
	`, input.ID).Scan(&currentCode, &identityLocked, &lockReason)
	if errors.Is(err, pgx.ErrNoRows) {
		return catalog.ProductCategory{}, productCategoryNotFound()
	}
	if err != nil {
		return catalog.ProductCategory{}, internalStoreError()
	}
	if identityLocked && currentCode != input.Form.Code {
		return catalog.ProductCategory{}, catalogIdentityLockedError(lockReason)
	}

	var category catalog.ProductCategory
	err = scanProductCategory(tx.QueryRow(ctx, `
		UPDATE product_categories
			SET code = $2, display_name = $3, icon_data_url = $4, sort_order = $5
			WHERE id = $1
			RETURNING `+productCategoryReturningColumns+`
		`, input.ID, input.Form.Code, input.Form.DisplayName, input.Form.IconDataURL, input.Form.SortOrder), &category)
	if errors.Is(err, pgx.ErrNoRows) {
		return catalog.ProductCategory{}, productCategoryNotFound()
	}
	if err != nil {
		if isUniqueViolation(err) {
			return catalog.ProductCategory{}, domain.NewFieldError(http.StatusConflict, domain.CodeValidationFailed, "Product category code unavailable", "分类 code 已被占用。", "code", "unavailable", "分类 code 已被占用。")
		}
		return catalog.ProductCategory{}, internalStoreError()
	}
	if err := tx.Commit(ctx); err != nil {
		return catalog.ProductCategory{}, internalStoreError()
	}
	return category, nil
}

func (s *Store) ListProductPlans(ctx context.Context, categoryCode string) ([]catalog.ProductPlan, *domain.AppError) {
	if s == nil || s.pool == nil {
		return nil, internalStoreError()
	}
	args := []any{}
	query := `
		SELECT ` + productPlanColumns + `
		FROM product_plans p
		JOIN product_categories c ON c.id = p.category_id
			WHERE p.status = 'active' AND c.status = 'active'
	`
	if strings.TrimSpace(categoryCode) != "" {
		args = append(args, strings.ToLower(strings.TrimSpace(categoryCode)))
		query += ` AND c.code = $1`
	}
	query += ` ORDER BY c.sort_order ASC, p.sort_order ASC, p.display_name ASC`

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, internalStoreError()
	}
	defer rows.Close()
	return scanProductPlans(rows)
}

func (s *Store) GetProductPlan(ctx context.Context, planID string) (catalog.ProductPlan, *domain.AppError) {
	if s == nil || s.pool == nil {
		return catalog.ProductPlan{}, internalStoreError()
	}
	var plan catalog.ProductPlan
	err := scanProductPlan(s.pool.QueryRow(ctx, `
		SELECT `+productPlanColumns+`
		FROM product_plans p
		JOIN product_categories c ON c.id = p.category_id
			WHERE p.id = $1
	`, planID), &plan)
	if errors.Is(err, pgx.ErrNoRows) {
		return catalog.ProductPlan{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Product plan not found", "产品套餐不存在。")
	}
	if err != nil {
		return catalog.ProductPlan{}, internalStoreError()
	}
	return plan, nil
}

func (s *Store) AdminListProductPlans(ctx context.Context, categoryCode string) ([]catalog.ProductPlan, *domain.AppError) {
	if s == nil || s.pool == nil {
		return nil, internalStoreError()
	}
	args := []any{}
	query := `
		SELECT ` + productPlanColumns + `
		FROM product_plans p
		JOIN product_categories c ON c.id = p.category_id
		WHERE true
	`
	if strings.TrimSpace(categoryCode) != "" {
		args = append(args, strings.ToLower(strings.TrimSpace(categoryCode)))
		query += ` AND c.code = $1`
	}
	query += ` ORDER BY c.sort_order ASC, p.sort_order ASC, p.display_name ASC`

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, internalStoreError()
	}
	defer rows.Close()
	return scanProductPlans(rows)
}

func (s *Store) AdminGetProductPlan(ctx context.Context, planID string) (catalog.ProductPlan, *domain.AppError) {
	if s == nil || s.pool == nil {
		return catalog.ProductPlan{}, internalStoreError()
	}
	var plan catalog.ProductPlan
	err := scanProductPlan(s.pool.QueryRow(ctx, `
		SELECT `+productPlanColumns+`
		FROM product_plans p
		JOIN product_categories c ON c.id = p.category_id
		WHERE p.id = $1
	`, planID), &plan)
	if errors.Is(err, pgx.ErrNoRows) {
		return catalog.ProductPlan{}, productPlanNotFound()
	}
	if err != nil {
		return catalog.ProductPlan{}, internalStoreError()
	}
	return plan, nil
}

func (s *Store) AdminCreateProductPlan(ctx context.Context, input catalog.ProductPlanMutationInput) (catalog.ProductPlan, *domain.AppError) {
	if s == nil || s.pool == nil {
		return catalog.ProductPlan{}, internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return catalog.ProductPlan{}, internalStoreError()
	}
	defer rollback(ctx, tx)

	var plan catalog.ProductPlan
	err = scanProductPlan(tx.QueryRow(ctx, `
		WITH changed AS (
			INSERT INTO product_plans (
			  category_id, provider_code, slug, display_name, description,
			  publish_policy, access_mode, provider_policy_status, risk_level,
			  risk_ack_required, risk_notice_code, policy_version, policy_note,
				  quota_label, quota_unit, quota_period, status, allow_custom_variant,
			  sort_order, policy_updated_at, policy_updated_by_user_id, updated_at
			)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, 1, $12, $13, $14, $15, 'active', $16, $17, now(), $18, now())
			RETURNING *
		)
		SELECT `+productPlanChangedColumns+`
		FROM changed
		JOIN product_categories c ON c.id = changed.category_id
	`, input.Form.CategoryID, input.Form.ProviderCode, input.Form.Slug, input.Form.DisplayName, input.Form.Description,
		input.Form.PublishPolicy, input.Form.AccessMode, input.Form.ProviderPolicyStatus, input.Form.RiskLevel,
		input.Form.RiskAckRequired, nullText(input.Form.RiskNoticeCode), input.Form.PolicyNote,
		input.Form.QuotaLabel, input.Form.QuotaUnit, input.Form.QuotaPeriod,
		input.Form.AllowCustomVariant, input.Form.SortOrder, nullUUID(input.OperatorID)), &plan)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return catalog.ProductPlan{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Product category not found", "产品分类不存在。", "categoryId", "not_found", "产品分类不存在。")
		}
		if isUniqueViolation(err) {
			return catalog.ProductPlan{}, domain.NewFieldError(http.StatusConflict, domain.CodeValidationFailed, "Product plan slug unavailable", "套餐 slug 已被占用。", "slug", "unavailable", "套餐 slug 已被占用。")
		}
		return catalog.ProductPlan{}, internalStoreError()
	}
	if appErr := insertProductPlanPolicyHistory(ctx, tx, plan, input.OperatorID, policyHistoryReason(input.Form.PolicyNote)); appErr != nil {
		return catalog.ProductPlan{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return catalog.ProductPlan{}, internalStoreError()
	}
	return plan, nil
}

func (s *Store) AdminUpdateProductPlan(ctx context.Context, input catalog.ProductPlanMutationInput) (catalog.ProductPlan, *domain.AppError) {
	if s == nil || s.pool == nil {
		return catalog.ProductPlan{}, internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return catalog.ProductPlan{}, internalStoreError()
	}
	defer rollback(ctx, tx)

	var before catalog.ProductPlan
	err = scanProductPlan(tx.QueryRow(ctx, `
		SELECT `+productPlanColumns+`
		FROM product_plans p
		JOIN product_categories c ON c.id = p.category_id
		WHERE p.id = $1
		FOR UPDATE OF p
	`, input.ID), &before)
	if errors.Is(err, pgx.ErrNoRows) {
		return catalog.ProductPlan{}, productPlanNotFound()
	}
	if err != nil {
		return catalog.ProductPlan{}, internalStoreError()
	}
	if before.IdentityLocked && (before.CategoryID != input.Form.CategoryID || before.ProviderCode != input.Form.ProviderCode || before.Slug != input.Form.Slug) {
		return catalog.ProductPlan{}, catalogIdentityLockedError(before.IdentityLockReason)
	}
	var parentActive bool
	if err := tx.QueryRow(ctx, `SELECT status = 'active' FROM product_categories WHERE id = $1 FOR UPDATE`, input.Form.CategoryID).Scan(&parentActive); err != nil || !parentActive {
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return catalog.ProductPlan{}, internalStoreError()
		}
		return catalog.ProductPlan{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Product category unavailable", "产品分类当前不可用。", "categoryId", "unavailable", "产品分类当前不可用。")
	}
	policyChanged := productPlanPolicyChanged(before, input.Form)

	var plan catalog.ProductPlan
	err = scanProductPlan(tx.QueryRow(ctx, `
		WITH changed AS (
			UPDATE product_plans
			SET category_id = $2,
			    provider_code = $3,
			    slug = $4,
			    display_name = $5,
			    description = $6,
			    publish_policy = $7,
			    access_mode = $8,
			    provider_policy_status = $9,
			    risk_level = $10,
			    risk_ack_required = $11,
			    risk_notice_code = $12,
			    policy_version = CASE WHEN $13 THEN policy_version + 1 ELSE policy_version END,
			    policy_note = $14,
			    quota_label = $15,
			    quota_unit = $16,
			    quota_period = $17,
				    allow_custom_variant = $18,
				    sort_order = $19,
			    policy_updated_at = CASE WHEN $13 THEN now() ELSE policy_updated_at END,
				    policy_updated_by_user_id = CASE WHEN $13 THEN $20 ELSE policy_updated_by_user_id END,
			    updated_at = now()
			WHERE id = $1
			RETURNING *
		)
		SELECT `+productPlanChangedColumns+`
		FROM changed
		JOIN product_categories c ON c.id = changed.category_id
	`, input.ID, input.Form.CategoryID, input.Form.ProviderCode, input.Form.Slug, input.Form.DisplayName,
		input.Form.Description, input.Form.PublishPolicy, input.Form.AccessMode, input.Form.ProviderPolicyStatus,
		input.Form.RiskLevel, input.Form.RiskAckRequired, nullText(input.Form.RiskNoticeCode), policyChanged,
		input.Form.PolicyNote, input.Form.QuotaLabel, input.Form.QuotaUnit, input.Form.QuotaPeriod,
		input.Form.AllowCustomVariant, input.Form.SortOrder,
		nullUUID(input.OperatorID)), &plan)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return catalog.ProductPlan{}, productPlanNotFound()
		}
		if isUniqueViolation(err) {
			return catalog.ProductPlan{}, domain.NewFieldError(http.StatusConflict, domain.CodeValidationFailed, "Product plan slug unavailable", "套餐 slug 已被占用。", "slug", "unavailable", "套餐 slug 已被占用。")
		}
		return catalog.ProductPlan{}, internalStoreError()
	}
	if policyChanged {
		if appErr := insertProductPlanPolicyHistory(ctx, tx, plan, input.OperatorID, policyHistoryReason(input.Form.PolicyNote)); appErr != nil {
			return catalog.ProductPlan{}, appErr
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return catalog.ProductPlan{}, internalStoreError()
	}
	return plan, nil
}

func (s *Store) AdminListAPIModelProviders(ctx context.Context) ([]catalog.APIModelProvider, *domain.AppError) {
	if s == nil || s.pool == nil {
		return nil, internalStoreError()
	}
	rows, err := s.pool.Query(ctx, `
		SELECT `+apiModelProviderColumns+`
		FROM api_model_providers
		ORDER BY provider_category ASC, sort_order ASC, display_name ASC, id ASC
	`)
	if err != nil {
		return nil, internalStoreError()
	}
	defer rows.Close()
	return scanAPIModelProviders(rows)
}

func (s *Store) AdminGetAPIModelProvider(ctx context.Context, providerID string) (catalog.APIModelProvider, *domain.AppError) {
	if s == nil || s.pool == nil {
		return catalog.APIModelProvider{}, internalStoreError()
	}
	var provider catalog.APIModelProvider
	err := scanAPIModelProvider(s.pool.QueryRow(ctx, `
		SELECT `+apiModelProviderColumns+`
		FROM api_model_providers
		WHERE id = $1
	`, providerID), &provider)
	if errors.Is(err, pgx.ErrNoRows) {
		return catalog.APIModelProvider{}, apiModelProviderNotFound()
	}
	if err != nil {
		return catalog.APIModelProvider{}, internalStoreError()
	}
	return provider, nil
}

func (s *Store) AdminCreateAPIModelProvider(ctx context.Context, input catalog.APIModelProviderMutationInput) (catalog.APIModelProvider, *domain.AppError) {
	if s == nil || s.pool == nil {
		return catalog.APIModelProvider{}, internalStoreError()
	}
	var provider catalog.APIModelProvider
	err := scanAPIModelProvider(s.pool.QueryRow(ctx, `
		INSERT INTO api_model_providers (
		  provider_category, code, display_name, status, sort_order, updated_at
		)
		VALUES ($1, $2, $3, 'active', $4, now())
		RETURNING `+apiModelProviderColumns+`
	`, input.Form.ProviderCategory, input.Form.Code, input.Form.DisplayName, input.Form.SortOrder), &provider)
	if err != nil {
		if isUniqueViolation(err) {
			return catalog.APIModelProvider{}, domain.NewFieldError(http.StatusConflict, domain.CodeValidationFailed, "API model provider code unavailable", "提供商 code 已被占用。", "code", "unavailable", "提供商 code 已被占用。")
		}
		return catalog.APIModelProvider{}, internalStoreError()
	}
	return provider, nil
}

func (s *Store) AdminUpdateAPIModelProvider(ctx context.Context, input catalog.APIModelProviderMutationInput) (catalog.APIModelProvider, *domain.AppError) {
	if s == nil || s.pool == nil {
		return catalog.APIModelProvider{}, internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return catalog.APIModelProvider{}, internalStoreError()
	}
	defer rollback(ctx, tx)

	var currentCategory, currentCode, lockReason string
	var identityLocked bool
	err = tx.QueryRow(ctx, `
		SELECT provider_category, code,
		       (core_key IS NOT NULL OR EXISTS (SELECT 1 FROM api_model_catalog ref WHERE ref.provider_id = api_model_providers.id)),
		       CASE
		         WHEN core_key IS NOT NULL THEN '核心目录身份由系统管理，不能修改。'
		         WHEN EXISTS (SELECT 1 FROM api_model_catalog ref WHERE ref.provider_id = api_model_providers.id) THEN '提供商已被模型引用，身份字段不能修改。'
		         ELSE ''
		       END
		FROM api_model_providers
		WHERE id = $1
		FOR UPDATE
	`, input.ID).Scan(&currentCategory, &currentCode, &identityLocked, &lockReason)
	if errors.Is(err, pgx.ErrNoRows) {
		return catalog.APIModelProvider{}, apiModelProviderNotFound()
	}
	if err != nil {
		return catalog.APIModelProvider{}, internalStoreError()
	}
	if identityLocked && (currentCategory != input.Form.ProviderCategory || currentCode != input.Form.Code) {
		return catalog.APIModelProvider{}, catalogIdentityLockedError(lockReason)
	}

	var provider catalog.APIModelProvider
	err = scanAPIModelProvider(tx.QueryRow(ctx, `
		UPDATE api_model_providers
		SET provider_category = $2,
		    code = $3,
		    display_name = $4,
		    sort_order = $5,
		    updated_at = now()
		WHERE id = $1
		RETURNING `+apiModelProviderColumns+`
	`, input.ID, input.Form.ProviderCategory, input.Form.Code, input.Form.DisplayName, input.Form.SortOrder), &provider)
	if errors.Is(err, pgx.ErrNoRows) {
		return catalog.APIModelProvider{}, apiModelProviderNotFound()
	}
	if err != nil {
		if isUniqueViolation(err) {
			return catalog.APIModelProvider{}, domain.NewFieldError(http.StatusConflict, domain.CodeValidationFailed, "API model provider code unavailable", "提供商 code 已被占用。", "code", "unavailable", "提供商 code 已被占用。")
		}
		return catalog.APIModelProvider{}, internalStoreError()
	}
	if err := tx.Commit(ctx); err != nil {
		return catalog.APIModelProvider{}, internalStoreError()
	}
	return provider, nil
}

func (s *Store) ListAPIModels(ctx context.Context) ([]catalog.APIModelCatalog, *domain.AppError) {
	if s == nil || s.pool == nil {
		return nil, internalStoreError()
	}
	rows, err := s.pool.Query(ctx, `
		SELECT `+apiModelColumns+`
		FROM `+apiModelViewSource+`
		WHERE status = 'active' AND provider_status = 'active'
		ORDER BY sort_order ASC, model_key ASC
	`)
	if err != nil {
		return nil, internalStoreError()
	}
	defer rows.Close()
	return scanAPIModels(rows)
}

func (s *Store) GetAPIModel(ctx context.Context, modelID string) (catalog.APIModelCatalog, *domain.AppError) {
	if s == nil || s.pool == nil {
		return catalog.APIModelCatalog{}, internalStoreError()
	}
	var model catalog.APIModelCatalog
	err := scanAPIModel(s.pool.QueryRow(ctx, `
		SELECT `+apiModelColumns+`
		FROM `+apiModelViewSource+`
		WHERE id = $1
	`, modelID), &model)
	if errors.Is(err, pgx.ErrNoRows) {
		return catalog.APIModelCatalog{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "API model not found", "API 模型不存在。")
	}
	if err != nil {
		return catalog.APIModelCatalog{}, internalStoreError()
	}
	return model, nil
}

func (s *Store) AdminListAPIModels(ctx context.Context) ([]catalog.APIModelCatalog, *domain.AppError) {
	if s == nil || s.pool == nil {
		return nil, internalStoreError()
	}
	rows, err := s.pool.Query(ctx, `
		SELECT `+apiModelColumns+`
		FROM `+apiModelViewSource+`
		ORDER BY provider_category ASC, sort_order ASC, model_key ASC, id ASC
	`)
	if err != nil {
		return nil, internalStoreError()
	}
	defer rows.Close()
	return scanAPIModels(rows)
}

func (s *Store) AdminGetAPIModel(ctx context.Context, modelID string) (catalog.APIModelCatalog, *domain.AppError) {
	if s == nil || s.pool == nil {
		return catalog.APIModelCatalog{}, internalStoreError()
	}
	var model catalog.APIModelCatalog
	err := scanAPIModel(s.pool.QueryRow(ctx, `
		SELECT `+apiModelColumns+`
		FROM `+apiModelViewSource+`
		WHERE id = $1
	`, modelID), &model)
	if errors.Is(err, pgx.ErrNoRows) {
		return catalog.APIModelCatalog{}, apiModelNotFound()
	}
	if err != nil {
		return catalog.APIModelCatalog{}, internalStoreError()
	}
	return model, nil
}

func (s *Store) AdminCreateAPIModel(ctx context.Context, input catalog.APIModelMutationInput) (catalog.APIModelCatalog, *domain.AppError) {
	if s == nil || s.pool == nil {
		return catalog.APIModelCatalog{}, internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return catalog.APIModelCatalog{}, internalStoreError()
	}
	defer rollback(ctx, tx)

	var modelID string
	err = tx.QueryRow(ctx, `
		INSERT INTO api_model_catalog (
		  provider_id, model_key, capabilities,
		  status, sort_order, updated_at
		)
		VALUES ($1, $2, $3, 'active', $4, now())
		RETURNING id::text
	`, input.Form.ProviderID, input.Form.ModelKey,
		input.Form.Capabilities, input.Form.SortOrder).Scan(&modelID)
	if err != nil {
		if isUniqueViolation(err) {
			return catalog.APIModelCatalog{}, domain.NewFieldError(http.StatusConflict, domain.CodeValidationFailed, "API model key unavailable", "模型标识已被占用。", "modelKey", "unavailable", "模型标识已被占用。")
		}
		return catalog.APIModelCatalog{}, internalStoreError()
	}
	if apiModelPriceInputPresent(input.Form) {
		if appErr := insertAPIModelPriceVersion(ctx, tx, modelID, input.Form); appErr != nil {
			return catalog.APIModelCatalog{}, appErr
		}
	}
	model, appErr := getAPIModelInTx(ctx, tx, modelID)
	if appErr != nil {
		return catalog.APIModelCatalog{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return catalog.APIModelCatalog{}, internalStoreError()
	}
	return model, nil
}

func (s *Store) AdminUpdateAPIModel(ctx context.Context, input catalog.APIModelMutationInput) (catalog.APIModelCatalog, *domain.AppError) {
	if s == nil || s.pool == nil {
		return catalog.APIModelCatalog{}, internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return catalog.APIModelCatalog{}, internalStoreError()
	}
	defer rollback(ctx, tx)

	var lockedID string
	err = tx.QueryRow(ctx, `
		SELECT id::text
		FROM api_model_catalog
		WHERE id = $1
		FOR UPDATE
	`, input.ID).Scan(&lockedID)
	if errors.Is(err, pgx.ErrNoRows) {
		return catalog.APIModelCatalog{}, apiModelNotFound()
	}
	if err != nil {
		return catalog.APIModelCatalog{}, internalStoreError()
	}
	input.ID = lockedID
	before, appErr := getAPIModelInTx(ctx, tx, input.ID)
	if appErr != nil {
		return catalog.APIModelCatalog{}, appErr
	}
	if before.IdentityLocked && (before.ProviderID != input.Form.ProviderID || before.ModelKey != input.Form.ModelKey) {
		return catalog.APIModelCatalog{}, catalogIdentityLockedError(before.IdentityLockReason)
	}
	var providerActive bool
	if err := tx.QueryRow(ctx, `SELECT status = 'active' FROM api_model_providers WHERE id = $1 FOR UPDATE`, input.Form.ProviderID).Scan(&providerActive); err != nil || !providerActive {
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return catalog.APIModelCatalog{}, internalStoreError()
		}
		return catalog.APIModelCatalog{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "API model provider unavailable", "API 提供商当前不可用。", "providerId", "unavailable", "API 提供商当前不可用。")
	}

	_, err = tx.Exec(ctx, `
		UPDATE api_model_catalog
		SET provider_id = $2,
		    model_key = $3,
		    capabilities = $4,
		    sort_order = $5,
		    updated_at = now()
		WHERE id = $1
	`, input.ID, input.Form.ProviderID, input.Form.ModelKey,
		input.Form.Capabilities, input.Form.SortOrder)
	if err != nil {
		if isUniqueViolation(err) {
			return catalog.APIModelCatalog{}, domain.NewFieldError(http.StatusConflict, domain.CodeValidationFailed, "API model key unavailable", "模型标识已被占用。", "modelKey", "unavailable", "模型标识已被占用。")
		}
		return catalog.APIModelCatalog{}, internalStoreError()
	}

	if apiModelPricePayloadChanged(before, input.Form) {
		var changedAt time.Time
		if err := tx.QueryRow(ctx, `SELECT now()`).Scan(&changedAt); err != nil {
			return catalog.APIModelCatalog{}, internalStoreError()
		}
		if before.CurrentPriceVersionID != "" {
			_, err = tx.Exec(ctx, `
				UPDATE api_model_price_versions
				SET valid_to = $2
				WHERE id = $1 AND valid_to IS NULL
			`, before.CurrentPriceVersionID, changedAt)
			if err != nil {
				return catalog.APIModelCatalog{}, internalStoreError()
			}
		}
		if before.CurrentPriceVersionID != "" || apiModelPriceInputPresent(input.Form) {
			if appErr := insertAPIModelPriceVersionAt(ctx, tx, input.ID, input.Form, changedAt); appErr != nil {
				return catalog.APIModelCatalog{}, appErr
			}
		}
	}

	model, appErr := getAPIModelInTx(ctx, tx, input.ID)
	if appErr != nil {
		return catalog.APIModelCatalog{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return catalog.APIModelCatalog{}, internalStoreError()
	}
	return model, nil
}

const productCategoryColumns = `
	id::text, code, display_name, icon_data_url, sort_order,
	COALESCE(core_key, ''), status, status, 'self', status_changed_at, status_reason,
	COALESCE(status_changed_by::text, ''), version,
	(core_key IS NOT NULL OR EXISTS (SELECT 1 FROM product_plans ref WHERE ref.category_id = product_categories.id)),
	CASE
	  WHEN core_key IS NOT NULL THEN '核心目录身份由系统管理，不能修改。'
	  WHEN EXISTS (SELECT 1 FROM product_plans ref WHERE ref.category_id = product_categories.id) THEN '分类已被套餐引用，身份字段不能修改。'
	  ELSE ''
	END,
	(status = 'active')
`

const productCategoryReturningColumns = `
	id::text, code, display_name, icon_data_url, sort_order,
	COALESCE(core_key, ''), status, status, 'self', status_changed_at, status_reason,
	COALESCE(status_changed_by::text, ''), version,
	(core_key IS NOT NULL),
	CASE WHEN core_key IS NOT NULL THEN '核心目录身份由系统管理，不能修改。' ELSE '' END,
	(status = 'active')
`

const productPlanColumns = `
	p.id::text, p.category_id::text, c.code, p.provider_code, p.slug, p.display_name,
	p.description, p.publish_policy, p.access_mode, p.provider_policy_status,
	p.risk_level, p.risk_ack_required, COALESCE(p.risk_notice_code, ''),
	p.policy_version, p.policy_note, p.allow_custom_variant, p.sort_order,
	p.quota_label, p.quota_unit, p.quota_period, p.created_at, p.updated_at,
	COALESCE(p.core_key, ''), p.status,
	CASE WHEN p.status = 'blocked' OR c.status = 'blocked' THEN 'blocked'
	     WHEN p.status = 'deprecated' OR c.status = 'deprecated' THEN 'deprecated'
	     ELSE 'active' END,
	CASE WHEN (c.status = 'blocked' AND p.status <> 'blocked') OR (c.status = 'deprecated' AND p.status = 'active') THEN 'parent' ELSE 'self' END,
	p.status_changed_at, p.status_reason, COALESCE(p.status_changed_by::text, ''), p.version,
	(p.core_key IS NOT NULL OR EXISTS (SELECT 1 FROM carpool_listings ref WHERE ref.product_plan_id = p.id)
	 OR EXISTS (SELECT 1 FROM official_price_records ref WHERE ref.product_plan_id = p.id)
	 OR EXISTS (SELECT 1 FROM official_price_leads ref WHERE ref.product_plan_id = p.id)),
	CASE
	  WHEN p.core_key IS NOT NULL THEN '核心目录身份由系统管理，不能修改。'
	  WHEN EXISTS (SELECT 1 FROM carpool_listings ref WHERE ref.product_plan_id = p.id) THEN '套餐已被车源引用，身份字段不能修改。'
	  WHEN EXISTS (SELECT 1 FROM official_price_records ref WHERE ref.product_plan_id = p.id) OR EXISTS (SELECT 1 FROM official_price_leads ref WHERE ref.product_plan_id = p.id) THEN '套餐已被价格记录引用，身份字段不能修改。'
	  ELSE ''
	END,
	(p.status = 'active' AND c.status = 'active')
`

const productPlanChangedColumns = `
	changed.id::text, changed.category_id::text, c.code, changed.provider_code, changed.slug, changed.display_name,
	changed.description, changed.publish_policy, changed.access_mode, changed.provider_policy_status,
	changed.risk_level, changed.risk_ack_required, COALESCE(changed.risk_notice_code, ''),
	changed.policy_version, changed.policy_note, changed.allow_custom_variant, changed.sort_order,
	changed.quota_label, changed.quota_unit, changed.quota_period, changed.created_at, changed.updated_at,
	COALESCE(changed.core_key, ''), changed.status,
	CASE WHEN changed.status = 'blocked' OR c.status = 'blocked' THEN 'blocked'
	     WHEN changed.status = 'deprecated' OR c.status = 'deprecated' THEN 'deprecated'
	     ELSE 'active' END,
	CASE WHEN (c.status = 'blocked' AND changed.status <> 'blocked') OR (c.status = 'deprecated' AND changed.status = 'active') THEN 'parent' ELSE 'self' END,
	changed.status_changed_at, changed.status_reason, COALESCE(changed.status_changed_by::text, ''), changed.version,
	(changed.core_key IS NOT NULL),
	CASE WHEN changed.core_key IS NOT NULL THEN '核心目录身份由系统管理，不能修改。' ELSE '' END,
	(changed.status = 'active' AND c.status = 'active')
`

const apiModelProviderColumns = `
	id::text, provider_category, code, display_name, sort_order, created_at, updated_at,
	COALESCE(core_key, ''), status, status, 'self', status_changed_at, status_reason,
	COALESCE(status_changed_by::text, ''), version,
	(core_key IS NOT NULL OR EXISTS (SELECT 1 FROM api_model_catalog ref WHERE ref.provider_id = api_model_providers.id)),
	CASE
	  WHEN core_key IS NOT NULL THEN '核心目录身份由系统管理，不能修改。'
	  WHEN EXISTS (SELECT 1 FROM api_model_catalog ref WHERE ref.provider_id = api_model_providers.id) THEN '提供商已被模型引用，身份字段不能修改。'
	  ELSE ''
	END,
	(status = 'active')
`

const apiModelColumns = `
	id::text, provider_id::text, provider_category, provider_code, provider, provider_status, (provider_status = 'active'), model_key, capabilities,
	sort_order, COALESCE(current_price_version_id::text, ''), COALESCE(current_price_source_url, ''),
	COALESCE(current_price_source_version, ''), current_price_valid_from,
	COALESCE(input_price_per_million::text, ''), COALESCE(cached_input_price_per_million::text, ''),
	COALESCE(output_price_per_million::text, ''), created_at, updated_at,
	COALESCE(core_key, ''), status, effective_status, effective_status_source,
	status_changed_at, status_reason, COALESCE(status_changed_by::text, ''), version,
	identity_locked, identity_lock_reason, (effective_status = 'active')
`

const apiModelViewSource = `(
	SELECT catalog.*,
	       provider.provider_category,
	       provider.code AS provider_code,
	       provider.display_name AS provider,
	       provider.status AS provider_status,
	       CASE WHEN catalog.status = 'blocked' OR provider.status = 'blocked' THEN 'blocked'
	            WHEN catalog.status = 'deprecated' OR provider.status = 'deprecated' THEN 'deprecated'
	            ELSE 'active' END AS effective_status,
	       CASE WHEN (provider.status = 'blocked' AND catalog.status <> 'blocked') OR (provider.status = 'deprecated' AND catalog.status = 'active') THEN 'parent' ELSE 'self' END AS effective_status_source,
	       (catalog.core_key IS NOT NULL OR EXISTS (SELECT 1 FROM api_service_models ref WHERE ref.model_catalog_id = catalog.id) OR EXISTS (SELECT 1 FROM api_model_price_versions ref WHERE ref.model_catalog_id = catalog.id)) AS identity_locked,
	       CASE
	         WHEN catalog.core_key IS NOT NULL THEN '核心目录身份由系统管理，不能修改。'
	         WHEN EXISTS (SELECT 1 FROM api_service_models ref WHERE ref.model_catalog_id = catalog.id) THEN '模型已被 API 服务引用，身份字段不能修改。'
	         WHEN EXISTS (SELECT 1 FROM api_model_price_versions ref WHERE ref.model_catalog_id = catalog.id) THEN '模型已被价格版本引用，身份字段不能修改。'
	         ELSE ''
	       END AS identity_lock_reason,
	       price.id AS current_price_version_id,
	       price.source_url AS current_price_source_url,
	       price.source_version AS current_price_source_version,
	       price.valid_from AS current_price_valid_from,
	       price.input_price_per_million,
	       price.cached_input_price_per_million,
	       price.output_price_per_million
	FROM api_model_catalog catalog
	JOIN api_model_providers provider ON provider.id = catalog.provider_id
	LEFT JOIN LATERAL (
		SELECT *
		FROM api_model_price_versions version
		WHERE version.model_catalog_id = catalog.id
		  AND version.valid_to IS NULL
		ORDER BY version.valid_from DESC
		LIMIT 1
	) price ON true
) api_model_view`

func scanProductPlans(rows pgx.Rows) ([]catalog.ProductPlan, *domain.AppError) {
	plans := []catalog.ProductPlan{}
	for rows.Next() {
		var plan catalog.ProductPlan
		if err := scanProductPlan(rows, &plan); err != nil {
			return nil, internalStoreError()
		}
		plans = append(plans, plan)
	}
	if err := rows.Err(); err != nil {
		return nil, internalStoreError()
	}
	return plans, nil
}

func scanProductCategories(rows pgx.Rows) ([]catalog.ProductCategory, *domain.AppError) {
	categories := []catalog.ProductCategory{}
	for rows.Next() {
		var category catalog.ProductCategory
		if err := scanProductCategory(rows, &category); err != nil {
			return nil, internalStoreError()
		}
		categories = append(categories, category)
	}
	if err := rows.Err(); err != nil {
		return nil, internalStoreError()
	}
	return categories, nil
}

func scanProductCategory(row scanner, category *catalog.ProductCategory) error {
	return row.Scan(
		&category.ID,
		&category.Code,
		&category.DisplayName,
		&category.IconDataURL,
		&category.SortOrder,
		&category.CoreKey,
		&category.Status,
		&category.EffectiveStatus,
		&category.EffectiveStatusSource,
		&category.StatusChangedAt,
		&category.StatusReason,
		&category.StatusChangedBy,
		&category.Version,
		&category.IdentityLocked,
		&category.IdentityLockReason,
		&category.Active,
	)
}

func scanProductPlan(row scanner, plan *catalog.ProductPlan) error {
	return row.Scan(
		&plan.ID,
		&plan.CategoryID,
		&plan.CategoryCode,
		&plan.ProviderCode,
		&plan.Slug,
		&plan.DisplayName,
		&plan.Description,
		&plan.PublishPolicy,
		&plan.AccessMode,
		&plan.ProviderPolicyStatus,
		&plan.RiskLevel,
		&plan.RiskAckRequired,
		&plan.RiskNoticeCode,
		&plan.PolicyVersion,
		&plan.PolicyNote,
		&plan.AllowCustomVariant,
		&plan.SortOrder,
		&plan.QuotaLabel,
		&plan.QuotaUnit,
		&plan.QuotaPeriod,
		&plan.CreatedAt,
		&plan.UpdatedAt,
		&plan.CoreKey,
		&plan.Status,
		&plan.EffectiveStatus,
		&plan.EffectiveStatusSource,
		&plan.StatusChangedAt,
		&plan.StatusReason,
		&plan.StatusChangedBy,
		&plan.Version,
		&plan.IdentityLocked,
		&plan.IdentityLockReason,
		&plan.Active,
	)
}

func productPlanPolicyChanged(current catalog.ProductPlan, input catalog.ProductPlanInput) bool {
	return current.PublishPolicy != input.PublishPolicy ||
		current.AccessMode != input.AccessMode ||
		current.ProviderPolicyStatus != input.ProviderPolicyStatus ||
		current.RiskLevel != input.RiskLevel ||
		current.RiskAckRequired != input.RiskAckRequired ||
		current.RiskNoticeCode != strings.TrimSpace(input.RiskNoticeCode) ||
		current.PolicyNote != input.PolicyNote ||
		current.QuotaLabel != input.QuotaLabel ||
		current.QuotaUnit != input.QuotaUnit ||
		current.QuotaPeriod != input.QuotaPeriod
}

func insertProductPlanPolicyHistory(ctx context.Context, tx pgx.Tx, plan catalog.ProductPlan, operatorID, reason string) *domain.AppError {
	_, err := tx.Exec(ctx, `
		INSERT INTO product_plan_policy_history (
		  product_plan_id, policy_version, publish_policy, access_mode, provider_policy_status,
		  risk_level, risk_ack_required, risk_notice_version_id, enforcement_mode, reason,
		  changed_by_admin_id, effective_at
		)
		SELECT
		  $1, $2, $3, $4, $5,
		  $6, $7, version.id, 'new_actions_only', $8,
		  $9, now()
		FROM (SELECT 1) seed
		LEFT JOIN risk_notices notice ON notice.code = NULLIF($10, '')
		LEFT JOIN risk_notice_versions version
		  ON version.risk_notice_id = notice.id
		  AND version.retired_at IS NULL
	`, plan.ID, plan.PolicyVersion, plan.PublishPolicy, plan.AccessMode, plan.ProviderPolicyStatus,
		plan.RiskLevel, plan.RiskAckRequired, reason, nullUUID(operatorID), plan.RiskNoticeCode)
	if err != nil {
		return internalStoreError()
	}
	return nil
}

func policyHistoryReason(policyNote string) string {
	policyNote = strings.TrimSpace(policyNote)
	if policyNote != "" {
		return policyNote
	}
	return "管理员更新产品套餐策略。"
}

func productCategoryNotFound() *domain.AppError {
	return domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Product category not found", "产品分类不存在。")
}

func productPlanNotFound() *domain.AppError {
	return domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Product plan not found", "产品套餐不存在。")
}

func scanAPIModelProviders(rows pgx.Rows) ([]catalog.APIModelProvider, *domain.AppError) {
	providers := []catalog.APIModelProvider{}
	for rows.Next() {
		var provider catalog.APIModelProvider
		if err := scanAPIModelProvider(rows, &provider); err != nil {
			return nil, internalStoreError()
		}
		providers = append(providers, provider)
	}
	if err := rows.Err(); err != nil {
		return nil, internalStoreError()
	}
	return providers, nil
}

func scanAPIModelProvider(row scanner, provider *catalog.APIModelProvider) error {
	return row.Scan(
		&provider.ID,
		&provider.ProviderCategory,
		&provider.Code,
		&provider.DisplayName,
		&provider.SortOrder,
		&provider.CreatedAt,
		&provider.UpdatedAt,
		&provider.CoreKey,
		&provider.Status,
		&provider.EffectiveStatus,
		&provider.EffectiveStatusSource,
		&provider.StatusChangedAt,
		&provider.StatusReason,
		&provider.StatusChangedBy,
		&provider.Version,
		&provider.IdentityLocked,
		&provider.IdentityLockReason,
		&provider.Active,
	)
}

func scanAPIModels(rows pgx.Rows) ([]catalog.APIModelCatalog, *domain.AppError) {
	models := []catalog.APIModelCatalog{}
	for rows.Next() {
		var model catalog.APIModelCatalog
		if err := scanAPIModel(rows, &model); err != nil {
			return nil, internalStoreError()
		}
		models = append(models, model)
	}
	if err := rows.Err(); err != nil {
		return nil, internalStoreError()
	}
	return models, nil
}

func scanAPIModel(row scanner, model *catalog.APIModelCatalog) error {
	return row.Scan(
		&model.ID,
		&model.ProviderID,
		&model.ProviderCategory,
		&model.ProviderCode,
		&model.Provider,
		&model.ProviderStatus,
		&model.ProviderActive,
		&model.ModelKey,
		&model.Capabilities,
		&model.SortOrder,
		&model.CurrentPriceVersionID,
		&model.CurrentPriceSourceURL,
		&model.CurrentPriceSourceVersion,
		&model.CurrentPriceValidFrom,
		&model.InputPricePerMillion,
		&model.CachedInputPricePerMillion,
		&model.OutputPricePerMillion,
		&model.CreatedAt,
		&model.UpdatedAt,
		&model.CoreKey,
		&model.Status,
		&model.EffectiveStatus,
		&model.EffectiveStatusSource,
		&model.StatusChangedAt,
		&model.StatusReason,
		&model.StatusChangedBy,
		&model.Version,
		&model.IdentityLocked,
		&model.IdentityLockReason,
		&model.Active,
	)
}

func getAPIModelInTx(ctx context.Context, tx pgx.Tx, modelID string) (catalog.APIModelCatalog, *domain.AppError) {
	var model catalog.APIModelCatalog
	err := scanAPIModel(tx.QueryRow(ctx, `
		SELECT `+apiModelColumns+`
		FROM `+apiModelViewSource+`
		WHERE id = $1
	`, modelID), &model)
	if errors.Is(err, pgx.ErrNoRows) {
		return catalog.APIModelCatalog{}, apiModelNotFound()
	}
	if err != nil {
		return catalog.APIModelCatalog{}, internalStoreError()
	}
	return model, nil
}

func insertAPIModelPriceVersion(ctx context.Context, tx pgx.Tx, modelID string, input catalog.APIModelInput) *domain.AppError {
	var validFrom time.Time
	if err := tx.QueryRow(ctx, `SELECT now()`).Scan(&validFrom); err != nil {
		return internalStoreError()
	}
	return insertAPIModelPriceVersionAt(ctx, tx, modelID, input, validFrom)
}

func insertAPIModelPriceVersionAt(ctx context.Context, tx pgx.Tx, modelID string, input catalog.APIModelInput, validFrom time.Time) *domain.AppError {
	_, err := tx.Exec(ctx, `
		INSERT INTO api_model_price_versions (
		  model_catalog_id, source_url, source_version, valid_from,
		  input_price_per_million, cached_input_price_per_million, output_price_per_million
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, modelID, nullText(input.SourceURL), nullText(input.SourceVersion), validFrom,
		nullNumeric(input.InputTokenPrice), nullNumeric(input.CachedInputTokenPrice), nullNumeric(input.OutputTokenPrice))
	if err != nil {
		return internalStoreError()
	}
	return nil
}

func apiModelPriceInputPresent(input catalog.APIModelInput) bool {
	return strings.TrimSpace(input.SourceURL) != "" ||
		strings.TrimSpace(input.SourceVersion) != "" ||
		strings.TrimSpace(input.InputTokenPrice) != "" ||
		strings.TrimSpace(input.CachedInputTokenPrice) != "" ||
		strings.TrimSpace(input.OutputTokenPrice) != ""
}

func apiModelPricePayloadChanged(current catalog.APIModelCatalog, input catalog.APIModelInput) bool {
	return current.CurrentPriceSourceURL != strings.TrimSpace(input.SourceURL) ||
		current.CurrentPriceSourceVersion != strings.TrimSpace(input.SourceVersion) ||
		current.InputPricePerMillion != strings.TrimSpace(input.InputTokenPrice) ||
		current.CachedInputPricePerMillion != strings.TrimSpace(input.CachedInputTokenPrice) ||
		current.OutputPricePerMillion != strings.TrimSpace(input.OutputTokenPrice)
}

func apiModelNotFound() *domain.AppError {
	return domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "API model not found", "API 模型不存在。")
}

func apiModelProviderNotFound() *domain.AppError {
	return domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "API model provider not found", "API 提供商不存在。")
}

func catalogIdentityLockedError(reason string) *domain.AppError {
	if strings.TrimSpace(reason) == "" {
		reason = "目录身份已被业务引用，不能直接修改。"
	}
	return domain.NewError(http.StatusConflict, "CATALOG_IDENTITY_LOCKED", "Catalog identity locked", reason)
}
