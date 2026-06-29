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
		SELECT id::text, code, display_name, sort_order, active
		FROM product_categories
		WHERE active = true
		ORDER BY sort_order ASC, display_name ASC
	`)
	if err != nil {
		return nil, internalStoreError()
	}
	defer rows.Close()

	categories := []catalog.ProductCategory{}
	for rows.Next() {
		var category catalog.ProductCategory
		if err := rows.Scan(&category.ID, &category.Code, &category.DisplayName, &category.SortOrder, &category.Active); err != nil {
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
		SELECT id::text, code, display_name, sort_order, active
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
		SELECT id::text, code, display_name, sort_order, active
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
		INSERT INTO product_categories (code, display_name, sort_order, active)
		VALUES ($1, $2, $3, $4)
		RETURNING id::text, code, display_name, sort_order, active
	`, input.Form.Code, input.Form.DisplayName, input.Form.SortOrder, input.Form.Active), &category)
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
	var category catalog.ProductCategory
	err := scanProductCategory(s.pool.QueryRow(ctx, `
		UPDATE product_categories
		SET code = $2, display_name = $3, sort_order = $4, active = $5
		WHERE id = $1
		RETURNING id::text, code, display_name, sort_order, active
	`, input.ID, input.Form.Code, input.Form.DisplayName, input.Form.SortOrder, input.Form.Active), &category)
	if errors.Is(err, pgx.ErrNoRows) {
		return catalog.ProductCategory{}, productCategoryNotFound()
	}
	if err != nil {
		if isUniqueViolation(err) {
			return catalog.ProductCategory{}, domain.NewFieldError(http.StatusConflict, domain.CodeValidationFailed, "Product category code unavailable", "分类 code 已被占用。", "code", "unavailable", "分类 code 已被占用。")
		}
		return catalog.ProductCategory{}, internalStoreError()
	}
	return category, nil
}

func (s *Store) AdminSetProductCategoryActive(ctx context.Context, input catalog.ProductCategoryMutationInput, active bool) (catalog.ProductCategory, *domain.AppError) {
	if s == nil || s.pool == nil {
		return catalog.ProductCategory{}, internalStoreError()
	}
	var category catalog.ProductCategory
	err := scanProductCategory(s.pool.QueryRow(ctx, `
		UPDATE product_categories
		SET active = $2
		WHERE id = $1
		RETURNING id::text, code, display_name, sort_order, active
	`, input.ID, active), &category)
	if errors.Is(err, pgx.ErrNoRows) {
		return catalog.ProductCategory{}, productCategoryNotFound()
	}
	if err != nil {
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
		WHERE p.active = true AND c.active = true
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
		WHERE p.id = $1 AND p.active = true AND c.active = true
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
			  quota_label, quota_unit, quota_period, active, allow_custom_variant,
			  sort_order, policy_updated_at, policy_updated_by_user_id, updated_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, 1, $12, $13, $14, $15, $16, $17, $18, now(), $19, now())
			RETURNING *
		)
		SELECT `+productPlanChangedColumns+`
		FROM changed
		JOIN product_categories c ON c.id = changed.category_id
	`, input.Form.CategoryID, input.Form.ProviderCode, input.Form.Slug, input.Form.DisplayName, input.Form.Description,
		input.Form.PublishPolicy, input.Form.AccessMode, input.Form.ProviderPolicyStatus, input.Form.RiskLevel,
		input.Form.RiskAckRequired, nullText(input.Form.RiskNoticeCode), input.Form.PolicyNote,
		input.Form.QuotaLabel, input.Form.QuotaUnit, input.Form.QuotaPeriod,
		input.Form.Active, input.Form.AllowCustomVariant, input.Form.SortOrder, nullUUID(input.OperatorID)), &plan)
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
			    active = $18,
			    allow_custom_variant = $19,
			    sort_order = $20,
			    policy_updated_at = CASE WHEN $13 THEN now() ELSE policy_updated_at END,
			    policy_updated_by_user_id = CASE WHEN $13 THEN $21 ELSE policy_updated_by_user_id END,
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
		input.Form.Active, input.Form.AllowCustomVariant, input.Form.SortOrder,
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

func (s *Store) AdminSetProductPlanActive(ctx context.Context, input catalog.ProductPlanMutationInput, active bool) (catalog.ProductPlan, *domain.AppError) {
	if s == nil || s.pool == nil {
		return catalog.ProductPlan{}, internalStoreError()
	}
	var plan catalog.ProductPlan
	err := scanProductPlan(s.pool.QueryRow(ctx, `
		WITH changed AS (
			UPDATE product_plans
			SET active = $2, updated_at = now()
			WHERE id = $1
			RETURNING *
		)
		SELECT `+productPlanChangedColumns+`
		FROM changed
		JOIN product_categories c ON c.id = changed.category_id
	`, input.ID, active), &plan)
	if errors.Is(err, pgx.ErrNoRows) {
		return catalog.ProductPlan{}, productPlanNotFound()
	}
	if err != nil {
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
		  provider_category, code, display_name, active, sort_order, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, now())
		RETURNING `+apiModelProviderColumns+`
	`, input.Form.ProviderCategory, input.Form.Code, input.Form.DisplayName, input.Form.Active, input.Form.SortOrder), &provider)
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
	var provider catalog.APIModelProvider
	err := scanAPIModelProvider(s.pool.QueryRow(ctx, `
		UPDATE api_model_providers
		SET provider_category = $2,
		    code = $3,
		    display_name = $4,
		    active = $5,
		    sort_order = $6,
		    updated_at = now()
		WHERE id = $1
		RETURNING `+apiModelProviderColumns+`
	`, input.ID, input.Form.ProviderCategory, input.Form.Code, input.Form.DisplayName, input.Form.Active, input.Form.SortOrder), &provider)
	if errors.Is(err, pgx.ErrNoRows) {
		return catalog.APIModelProvider{}, apiModelProviderNotFound()
	}
	if err != nil {
		if isUniqueViolation(err) {
			return catalog.APIModelProvider{}, domain.NewFieldError(http.StatusConflict, domain.CodeValidationFailed, "API model provider code unavailable", "提供商 code 已被占用。", "code", "unavailable", "提供商 code 已被占用。")
		}
		return catalog.APIModelProvider{}, internalStoreError()
	}
	return provider, nil
}

func (s *Store) AdminSetAPIModelProviderActive(ctx context.Context, input catalog.APIModelProviderMutationInput, active bool) (catalog.APIModelProvider, *domain.AppError) {
	if s == nil || s.pool == nil {
		return catalog.APIModelProvider{}, internalStoreError()
	}
	var provider catalog.APIModelProvider
	err := scanAPIModelProvider(s.pool.QueryRow(ctx, `
		UPDATE api_model_providers
		SET active = $2, updated_at = now()
		WHERE id = $1
		RETURNING `+apiModelProviderColumns+`
	`, input.ID, active), &provider)
	if errors.Is(err, pgx.ErrNoRows) {
		return catalog.APIModelProvider{}, apiModelProviderNotFound()
	}
	if err != nil {
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
		WHERE active = true AND provider_active = true
		ORDER BY sort_order ASC, display_name ASC
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
		WHERE id = $1 AND active = true AND provider_active = true
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
		ORDER BY provider_category ASC, sort_order ASC, display_name ASC, id ASC
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
		  provider_id, model_key, display_name, capabilities,
		  active, sort_order, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, now())
		RETURNING id::text
	`, input.Form.ProviderID, input.Form.ModelKey, input.Form.DisplayName,
		input.Form.Capabilities, input.Form.Active, input.Form.SortOrder).Scan(&modelID)
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

	_, err = tx.Exec(ctx, `
		UPDATE api_model_catalog
		SET provider_id = $2,
		    model_key = $3,
		    display_name = $4,
		    capabilities = $5,
		    active = $6,
		    sort_order = $7,
		    updated_at = now()
		WHERE id = $1
	`, input.ID, input.Form.ProviderID, input.Form.ModelKey,
		input.Form.DisplayName, input.Form.Capabilities, input.Form.Active, input.Form.SortOrder)
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

func (s *Store) AdminSetAPIModelActive(ctx context.Context, input catalog.APIModelMutationInput, active bool) (catalog.APIModelCatalog, *domain.AppError) {
	if s == nil || s.pool == nil {
		return catalog.APIModelCatalog{}, internalStoreError()
	}
	var model catalog.APIModelCatalog
	err := scanAPIModel(s.pool.QueryRow(ctx, `
		WITH changed AS (
			UPDATE api_model_catalog
			SET active = $2, updated_at = now()
			WHERE id = $1
			RETURNING id
		)
		SELECT `+apiModelColumns+`
		FROM `+apiModelViewSource+`
		JOIN changed ON changed.id = api_model_view.id
	`, input.ID, active), &model)
	if errors.Is(err, pgx.ErrNoRows) {
		return catalog.APIModelCatalog{}, apiModelNotFound()
	}
	if err != nil {
		return catalog.APIModelCatalog{}, internalStoreError()
	}
	return model, nil
}

const productPlanColumns = `
	p.id::text, p.category_id::text, c.code, p.provider_code, p.slug, p.display_name,
	p.description, p.publish_policy, p.access_mode, p.provider_policy_status,
	p.risk_level, p.risk_ack_required, COALESCE(p.risk_notice_code, ''),
	p.policy_version, p.policy_note, p.active, p.allow_custom_variant, p.sort_order,
	p.quota_label, p.quota_unit, p.quota_period, p.created_at, p.updated_at
`

const productPlanChangedColumns = `
	changed.id::text, changed.category_id::text, c.code, changed.provider_code, changed.slug, changed.display_name,
	changed.description, changed.publish_policy, changed.access_mode, changed.provider_policy_status,
	changed.risk_level, changed.risk_ack_required, COALESCE(changed.risk_notice_code, ''),
	changed.policy_version, changed.policy_note, changed.active, changed.allow_custom_variant, changed.sort_order,
	changed.quota_label, changed.quota_unit, changed.quota_period, changed.created_at, changed.updated_at
`

const apiModelProviderColumns = `
	id::text, provider_category, code, display_name, active, sort_order, created_at, updated_at
`

const apiModelColumns = `
	id::text, provider_id::text, provider_category, provider_code, provider, provider_active, model_key, display_name, capabilities, active,
	sort_order, COALESCE(current_price_version_id::text, ''), COALESCE(current_price_source_url, ''),
	COALESCE(current_price_source_version, ''), current_price_valid_from,
	COALESCE(input_price_per_million::text, ''), COALESCE(cached_input_price_per_million::text, ''),
	COALESCE(output_price_per_million::text, ''), created_at, updated_at
`

const apiModelViewSource = `(
	SELECT catalog.*,
	       provider.provider_category,
	       provider.code AS provider_code,
	       provider.display_name AS provider,
	       provider.active AS provider_active,
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
		&category.SortOrder,
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
		&plan.Active,
		&plan.AllowCustomVariant,
		&plan.SortOrder,
		&plan.QuotaLabel,
		&plan.QuotaUnit,
		&plan.QuotaPeriod,
		&plan.CreatedAt,
		&plan.UpdatedAt,
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
		&provider.Active,
		&provider.SortOrder,
		&provider.CreatedAt,
		&provider.UpdatedAt,
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
		&model.ProviderActive,
		&model.ModelKey,
		&model.DisplayName,
		&model.Capabilities,
		&model.Active,
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
