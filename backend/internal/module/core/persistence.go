package core

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/announcement"
	"c2c-market/backend/internal/module/apiintent"
	"c2c-market/backend/internal/module/apimarket"
	"c2c-market/backend/internal/module/apiorder"
	"c2c-market/backend/internal/module/auth"
	"c2c-market/backend/internal/module/carpool"
	"c2c-market/backend/internal/module/catalog"
	"c2c-market/backend/internal/module/contact"
	"c2c-market/backend/internal/module/demand"
	"c2c-market/backend/internal/module/favorite"
	"c2c-market/backend/internal/module/feedback"
	"c2c-market/backend/internal/module/idempotency"
	"c2c-market/backend/internal/module/notification"
	"c2c-market/backend/internal/module/officialprice"
	"c2c-market/backend/internal/module/profile"
	"c2c-market/backend/internal/module/report"
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

type AnnouncementRepository = announcement.Repository

type NotificationRepository = notification.Repository

type CarpoolRepository = carpool.Repository

type ContactRepository = contact.Repository

type ProfileRepository = profile.Repository

type DemandRepository = demand.Repository

type FeedbackRepository = feedback.Repository

type FavoriteRepository = favorite.Repository

type ReviewRepository = review.Repository

type SearchRepository = search.Repository

type ReportRepository = report.Repository

type Persistence interface {
	AuthRepository
	IdempotencyRepository
	OfficialPriceRepository
	CatalogRepository
	APIServiceRepository
	APIPurchaseIntentRepository
	APIOrderRepository
	AnnouncementRepository
	NotificationRepository
	CarpoolRepository
	ContactRepository
	ProfileRepository
	DemandRepository
	FeedbackRepository
	FavoriteRepository
	ReviewRepository
	SearchRepository
	ReportRepository
}

type Repositories struct {
	Auth              AuthRepository
	Idempotency       IdempotencyRepository
	OfficialPrice     OfficialPriceRepository
	Catalog           CatalogRepository
	APIService        APIServiceRepository
	APIPurchaseIntent APIPurchaseIntentRepository
	APIOrder          APIOrderRepository
	Announcement      AnnouncementRepository
	Notification      NotificationRepository
	Carpool           CarpoolRepository
	Contact           ContactRepository
	Profile           ProfileRepository
	Demand            DemandRepository
	Feedback          FeedbackRepository
	Favorite          FavoriteRepository
	Review            ReviewRepository
	Search            SearchRepository
	Report            ReportRepository
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
		Announcement:      persistence,
		Notification:      persistence,
		Carpool:           persistence,
		Contact:           persistence,
		Profile:           persistence,
		Demand:            persistence,
		Feedback:          persistence,
		Favorite:          persistence,
		Review:            persistence,
		Search:            persistence,
		Report:            persistence,
	}
}

func hashOpaqueToken(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func internalPersistenceError() *domain.AppError {
	return domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "持久化操作失败。")
}
