package search

import (
	"context"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/apimarket"
	"c2c-market/backend/internal/module/carpool"
	"c2c-market/backend/internal/module/demand"
	"c2c-market/backend/internal/module/officialprice"
)

const (
	MaxKeywordRunes = 80
	DefaultPerType  = 8
)

type PublicReader interface {
	PublicOfficialPriceRecords(ctx context.Context) ([]officialprice.Record, *domain.AppError)
	PublicCarpoolListings(ctx context.Context, page domain.PageRequest) (domain.Page[carpool.Listing], *domain.AppError)
	PublicDemands(ctx context.Context) ([]demand.Demand, *domain.AppError)
	PublicAPIServices(ctx context.Context, filter apimarket.PublicServiceFilter) ([]apimarket.Service, *domain.AppError)
}

type Service struct {
	repo      Repository
	reader    PublicReader
	perType   int
	typeOrder map[string]int
}

func NewService(repo Repository, reader PublicReader) *Service {
	return &Service{
		repo:    repo,
		reader:  reader,
		perType: DefaultPerType,
		typeOrder: map[string]int{
			TypeOfficialPrice: 0,
			TypeCarpool:       1,
			TypeDemand:        2,
			TypeAPIService:    3,
			TypeUser:          4,
			TypeMerchant:      5,
		},
	}
}

func (s *Service) Search(ctx context.Context, keyword string) ([]Result, *domain.AppError) {
	normalized, appErr := NormalizeKeyword(keyword)
	if appErr != nil {
		return nil, appErr
	}
	if normalized == "" {
		return []Result{}, nil
	}
	if s.repo != nil {
		items, appErr := s.repo.Search(ctx, normalized, s.perType)
		if appErr != nil {
			return nil, appErr
		}
		sortResults(items, s.typeOrder)
		return items, nil
	}
	return s.searchInMemory(ctx, normalized)
}

func NormalizeKeyword(keyword string) (string, *domain.AppError) {
	normalized := strings.Join(strings.Fields(strings.TrimSpace(keyword)), " ")
	if len([]rune(normalized)) > MaxKeywordRunes {
		return "", domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Search keyword too long", "搜索关键词过长。", "q", "too_long", "搜索关键词不能超过 80 个字符。")
	}
	return normalized, nil
}

func (s *Service) searchInMemory(ctx context.Context, keyword string) ([]Result, *domain.AppError) {
	if s.reader == nil {
		return []Result{}, nil
	}
	q := strings.ToLower(keyword)
	results := []Result{}

	officialRecords, appErr := s.reader.PublicOfficialPriceRecords(ctx)
	if appErr != nil {
		return nil, appErr
	}
	for _, item := range officialRecords {
		if !matches(q, item.ProductPlanID, item.RegionCode, item.Channel, item.OpeningMethod, item.OriginalAmount, item.NormalizedMonthlyCNY) {
			continue
		}
		results = append(results, Result{
			ID:       "official-" + item.ID,
			Type:     TypeOfficialPrice,
			Title:    "官方价格 " + item.ProductPlanID,
			Subtitle: item.RegionCode + " · " + item.Channel + " · ¥" + item.NormalizedMonthlyCNY + "/月",
			Badge:    item.Status,
			To:       "/official-prices/" + item.ID,
			RankTime: item.CreatedAt,
		})
	}

	carpoolCursor := ""
	for {
		carpoolPage, appErr := s.reader.PublicCarpoolListings(ctx, domain.PageRequest{Limit: 100, Cursor: carpoolCursor})
		if appErr != nil {
			return nil, appErr
		}
		for _, item := range carpoolPage.Items {
			if !matches(q, item.Title, item.Summary, item.AccessArrangement, item.PriceMonthlyCNY) {
				continue
			}
			results = append(results, Result{
				ID:       "carpool-" + item.ID,
				Type:     TypeCarpool,
				Title:    item.Title,
				Subtitle: "¥" + item.PriceMonthlyCNY + "/月 · 可用席位 " + strconv.Itoa(item.AvailableSeats),
				Badge:    item.Status,
				To:       "/carpools/" + item.ID,
				RankTime: item.UpdatedAt,
			})
		}
		if carpoolPage.NextCursor == nil {
			break
		}
		carpoolCursor = *carpoolPage.NextCursor
	}

	demands, appErr := s.reader.PublicDemands(ctx)
	if appErr != nil {
		return nil, appErr
	}
	for _, item := range demands {
		if !matches(q, item.Title, item.RegionCode, item.OwnerPreference, item.PublisherUsername, item.PublisherName) {
			continue
		}
		results = append(results, Result{
			ID:       "demand-" + item.ID,
			Type:     TypeDemand,
			Title:    item.Title,
			Subtitle: item.RegionCode + " · 预算 ¥" + item.MaxPriceCNY + "/月 · " + item.PublisherName,
			Badge:    item.Status,
			To:       "/demands/" + item.ID,
			RankTime: item.UpdatedAt,
		})
	}

	services, appErr := s.reader.PublicAPIServices(ctx, apimarket.PublicServiceFilter{})
	if appErr != nil {
		return nil, appErr
	}
	for _, item := range services {
		models := serviceModelNames(item)
		terms := append([]string{item.Title, item.ShortDescription, item.MerchantDisplayName}, models...)
		if item.MerchantIdentityMode == "public_profile" {
			terms = append(terms, item.MerchantProfileSlug)
		}
		if !matches(q, terms...) {
			continue
		}
		results = append(results, Result{
			ID:       "api-" + item.ID,
			Type:     TypeAPIService,
			Title:    item.Title,
			Subtitle: merchantName(item) + " · " + strings.Join(limitStrings(models, 3), " / "),
			Badge:    "在线",
			To:       "/api-market/" + item.ID,
			RankTime: item.UpdatedAt,
		})
	}

	results = limitByType(results, s.perType, s.typeOrder)
	sortResults(results, s.typeOrder)
	return results, nil
}

func matches(q string, values ...string) bool {
	for _, value := range values {
		if strings.Contains(strings.ToLower(value), q) {
			return true
		}
	}
	return false
}

func sortResults(items []Result, typeOrder map[string]int) {
	sort.SliceStable(items, func(i, j int) bool {
		li, lok := typeOrder[items[i].Type]
		lj, jok := typeOrder[items[j].Type]
		if !lok {
			li = 99
		}
		if !jok {
			lj = 99
		}
		if li != lj {
			return li < lj
		}
		if !items[i].RankTime.Equal(items[j].RankTime) {
			return items[i].RankTime.After(items[j].RankTime)
		}
		return items[i].ID < items[j].ID
	})
}

func limitByType(items []Result, limit int, typeOrder map[string]int) []Result {
	sortResults(items, typeOrder)
	counts := map[string]int{}
	result := make([]Result, 0, len(items))
	for _, item := range items {
		if counts[item.Type] >= limit {
			continue
		}
		counts[item.Type]++
		result = append(result, item)
	}
	return result
}

func serviceModelNames(service apimarket.Service) []string {
	models := make([]string, 0, len(service.Models))
	for _, model := range service.Models {
		if model.Enabled {
			models = append(models, model.ModelNameSnapshot)
		}
	}
	return models
}

func merchantName(service apimarket.Service) string {
	if strings.TrimSpace(service.MerchantDisplayName) != "" {
		return service.MerchantDisplayName
	}
	return "API 商户"
}

func limitStrings(values []string, limit int) []string {
	if len(values) <= limit {
		return values
	}
	return values[:limit]
}
