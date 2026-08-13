package core

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/accountgovernance"
	"c2c-market/backend/internal/module/announcement"
	"c2c-market/backend/internal/module/apiintent"
	"c2c-market/backend/internal/module/apimarket"
	"c2c-market/backend/internal/module/apiorder"
	"c2c-market/backend/internal/module/apipromotion"
	"c2c-market/backend/internal/module/apiquota"
	"c2c-market/backend/internal/module/auth"
	"c2c-market/backend/internal/module/carpool"
	"c2c-market/backend/internal/module/catalog"
	"c2c-market/backend/internal/module/contact"
	"c2c-market/backend/internal/module/favorite"
	"c2c-market/backend/internal/module/feedback"
	"c2c-market/backend/internal/module/growth"
	"c2c-market/backend/internal/module/idempotency"
	"c2c-market/backend/internal/module/modelaudit"
	"c2c-market/backend/internal/module/notification"
	"c2c-market/backend/internal/module/officialprice"
	"c2c-market/backend/internal/module/operationaudit"
	"c2c-market/backend/internal/module/profile"
	"c2c-market/backend/internal/module/promotionreward"
	"c2c-market/backend/internal/module/report"
	"c2c-market/backend/internal/module/reputation"
	"c2c-market/backend/internal/module/review"
	"c2c-market/backend/internal/module/search"
)

type AuthRepository = auth.Repository

type IdempotencyRepository = idempotency.Repository

type OfficialPriceRepository = officialprice.Repository

type CatalogRepository = catalog.Repository

type APIServiceRepository = apimarket.Repository

type APIPurchaseIntentRepository = apiintent.Repository

type APIOrderRepository = apiorder.Repository

type APIPromotionRepository = apipromotion.Repository

type APIQuotaRepository = apiquota.Repository

type AnnouncementRepository = announcement.Repository

type NotificationRepository = notification.Repository

type CarpoolRepository = carpool.Repository

type ContactRepository = contact.Repository

type ProfileRepository = profile.Repository

type FeedbackRepository = feedback.Repository

type FavoriteRepository = favorite.Repository

type ReviewRepository = review.Repository

type SearchRepository = search.Repository

type ReportRepository = report.Repository

type ReputationRepository = reputation.Repository

type ModelAuditRepository = modelaudit.Repository

type GrowthRepository = growth.Repository

type PromotionRewardRepository = promotionreward.Repository

type OperationAuditRepository = operationaudit.Repository

type AccountGovernanceRepository = accountgovernance.Repository

type Persistence interface {
	AuthRepository
	IdempotencyRepository
	OfficialPriceRepository
	CatalogRepository
	APIServiceRepository
	APIPurchaseIntentRepository
	APIOrderRepository
	APIPromotionRepository
	APIQuotaRepository
	AnnouncementRepository
	NotificationRepository
	CarpoolRepository
	ContactRepository
	ProfileRepository
	FeedbackRepository
	FavoriteRepository
	ReviewRepository
	SearchRepository
	ReportRepository
	ReputationRepository
	ModelAuditRepository
	GrowthRepository
	PromotionRewardRepository
	OperationAuditRepository
	AccountGovernanceRepository
}

type Repositories struct {
	Auth              AuthRepository
	Idempotency       IdempotencyRepository
	OfficialPrice     OfficialPriceRepository
	Catalog           CatalogRepository
	APIService        APIServiceRepository
	APIPurchaseIntent APIPurchaseIntentRepository
	APIOrder          APIOrderRepository
	APIPromotion      APIPromotionRepository
	APIQuota          APIQuotaRepository
	Announcement      AnnouncementRepository
	Notification      NotificationRepository
	Carpool           CarpoolRepository
	Contact           ContactRepository
	Profile           ProfileRepository
	Feedback          FeedbackRepository
	Favorite          FavoriteRepository
	Review            ReviewRepository
	Search            SearchRepository
	Report            ReportRepository
	Reputation        ReputationRepository
	ModelAudit        ModelAuditRepository
	Growth            GrowthRepository
	PromotionReward   PromotionRewardRepository
	OperationAudit    OperationAuditRepository
	AccountGovernance AccountGovernanceRepository
}

func RepositoriesFromPersistence(persistence Persistence) Repositories {
	if persistence == nil {
		return Repositories{}
	}
	return Repositories{
		Auth:              persistence,
		Idempotency:       persistence,
		OfficialPrice:     persistence,
		Catalog:           persistence,
		APIService:        persistence,
		APIPurchaseIntent: persistence,
		APIOrder:          persistence,
		APIPromotion:      persistence,
		APIQuota:          persistence,
		Announcement:      persistence,
		Notification:      persistence,
		Carpool:           persistence,
		Contact:           persistence,
		Profile:           persistence,
		Feedback:          persistence,
		Favorite:          persistence,
		Review:            persistence,
		Search:            persistence,
		Report:            persistence,
		Reputation:        persistence,
		ModelAudit:        persistence,
		Growth:            persistence,
		PromotionReward:   persistence,
		OperationAudit:    persistence,
		AccountGovernance: persistence,
	}
}

func hashOpaqueToken(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func internalPersistenceError() *domain.AppError {
	return domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "持久化操作失败。")
}
