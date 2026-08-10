package apiorder

import (
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"

	"c2c-market/backend/internal/domain"

	"github.com/google/uuid"
)

const (
	adminOrderCursorVersion = 1

	AdminOrderDateRangeAll    = "all"
	AdminOrderDateRangeToday  = "today"
	AdminOrderDateRange7Days  = "7d"
	AdminOrderDateRange30Days = "30d"

	AdminOrderDisputeAll    = "all"
	AdminOrderDisputeActive = "active"
	AdminOrderDisputeNone   = "none"

	AdminOrderSortUpdatedDesc = "updated_desc"
	AdminOrderSortCreatedDesc = "created_desc"
	AdminOrderSortAmountDesc  = "amount_desc"
	AdminOrderSortAmountAsc   = "amount_asc"
)

var adminOrderAmountPattern = regexp.MustCompile(`^[0-9]+(?:\.[0-9]+)?$`)

type adminOrderCursor struct {
	Version int    `json:"v"`
	Sort    string `json:"s"`
	Value   string `json:"value"`
	ID      string `json:"id"`
}

type AdminOrderCursorPosition struct {
	Value string
	ID    string
}

type AdminOrderFilter struct {
	Query        string
	Statuses     []string
	DateRange    string
	BuyerUserID  string
	SellerUserID string
	APIServiceID string
	Dispute      string
	MinAmount    string
	MaxAmount    string
	Sort         string
}

func IsStatus(value string) bool {
	switch strings.TrimSpace(value) {
	case StatusPendingPayment, StatusPaymentSubmitted, StatusPaymentIssue, StatusPaidConfirmed,
		StatusDeliverySubmitted, StatusCompleted, StatusCancelled:
		return true
	default:
		return false
	}
}

func IsAdminOrderDateRange(value string) bool {
	switch strings.TrimSpace(value) {
	case "", AdminOrderDateRangeAll, AdminOrderDateRangeToday, AdminOrderDateRange7Days, AdminOrderDateRange30Days:
		return true
	default:
		return false
	}
}

func IsAdminOrderDispute(value string) bool {
	switch strings.TrimSpace(value) {
	case "", AdminOrderDisputeAll, AdminOrderDisputeActive, AdminOrderDisputeNone:
		return true
	default:
		return false
	}
}

func IsAdminOrderSort(value string) bool {
	switch strings.TrimSpace(value) {
	case "", AdminOrderSortUpdatedDesc, AdminOrderSortCreatedDesc, AdminOrderSortAmountDesc, AdminOrderSortAmountAsc:
		return true
	default:
		return false
	}
}

func NormalizeAdminOrderAmount(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", true
	}
	if !adminOrderAmountPattern.MatchString(value) {
		return "", false
	}
	parsed, ok := new(big.Rat).SetString(value)
	if !ok || parsed.Sign() < 0 {
		return "", false
	}
	return value, true
}

func CompareAdminOrderAmounts(left, right string) int {
	leftValue, leftOK := new(big.Rat).SetString(strings.TrimSpace(left))
	rightValue, rightOK := new(big.Rat).SetString(strings.TrimSpace(right))
	if !leftOK || !rightOK {
		return strings.Compare(left, right)
	}
	return leftValue.Cmp(rightValue)
}

func (filter AdminOrderFilter) NormalizedSort() string {
	if IsAdminOrderSort(filter.Sort) && strings.TrimSpace(filter.Sort) != "" {
		return strings.TrimSpace(filter.Sort)
	}
	return AdminOrderSortUpdatedDesc
}

func (filter AdminOrderFilter) CreatedAfter(now time.Time) (time.Time, bool) {
	var duration time.Duration
	switch strings.TrimSpace(filter.DateRange) {
	case AdminOrderDateRangeToday:
		businessNow := now.In(shanghaiTime)
		return time.Date(businessNow.Year(), businessNow.Month(), businessNow.Day(), 0, 0, 0, 0, shanghaiTime), true
	case AdminOrderDateRange7Days:
		duration = 7 * 24 * time.Hour
	case AdminOrderDateRange30Days:
		duration = 30 * 24 * time.Hour
	default:
		return time.Time{}, false
	}
	return now.Add(-duration), true
}

func FilterAdminOrders(orders []Order, filter AdminOrderFilter, now time.Time) []Order {
	query := strings.ToLower(strings.TrimSpace(filter.Query))
	normalizedQuery := normalizeOrderSearch(query)
	statuses := make(map[string]struct{}, len(filter.Statuses))
	for _, status := range filter.Statuses {
		statuses[strings.TrimSpace(status)] = struct{}{}
	}
	createdAfter, hasCreatedAfter := filter.CreatedAfter(now)
	minAmount, hasMinAmount := decimalFilterValue(filter.MinAmount)
	maxAmount, hasMaxAmount := decimalFilterValue(filter.MaxAmount)

	filtered := make([]Order, 0, len(orders))
	for _, order := range orders {
		if len(statuses) > 0 {
			if _, ok := statuses[order.Status]; !ok {
				continue
			}
		}
		if filter.BuyerUserID != "" && order.BuyerUserID != filter.BuyerUserID ||
			filter.SellerUserID != "" && order.SellerUserID != filter.SellerUserID ||
			filter.APIServiceID != "" && order.APIServiceID != filter.APIServiceID ||
			hasCreatedAfter && order.CreatedAt.Before(createdAfter) {
			continue
		}
		isActiveDispute := IsDisputeActive(order.DisputeStatus)
		if filter.Dispute == AdminOrderDisputeActive && !isActiveDispute || filter.Dispute == AdminOrderDisputeNone && isActiveDispute {
			continue
		}
		amount, amountOK := new(big.Rat).SetString(order.Amount)
		if hasMinAmount && (!amountOK || amount.Cmp(minAmount) < 0) || hasMaxAmount && (!amountOK || amount.Cmp(maxAmount) > 0) {
			continue
		}
		if query != "" && !containsOrderSearch(query, normalizedQuery, order) {
			continue
		}
		filtered = append(filtered, order)
	}

	sortMode := filter.NormalizedSort()
	sort.SliceStable(filtered, func(i, j int) bool {
		left, right := filtered[i], filtered[j]
		switch sortMode {
		case AdminOrderSortCreatedDesc:
			if !left.CreatedAt.Equal(right.CreatedAt) {
				return left.CreatedAt.After(right.CreatedAt)
			}
		case AdminOrderSortAmountDesc:
			if compared := CompareAdminOrderAmounts(left.Amount, right.Amount); compared != 0 {
				return compared > 0
			}
		case AdminOrderSortAmountAsc:
			if compared := CompareAdminOrderAmounts(left.Amount, right.Amount); compared != 0 {
				return compared < 0
			}
			return left.ID < right.ID
		default:
			if !left.UpdatedAt.Equal(right.UpdatedAt) {
				return left.UpdatedAt.After(right.UpdatedAt)
			}
		}
		return left.ID > right.ID
	})
	return filtered
}

func PageAdminOrders(orders []Order, filter AdminOrderFilter, page domain.PageRequest, now time.Time) (domain.Page[Order], *domain.AppError) {
	page = normalizeAdminOrderPageRequest(page)
	sortMode := filter.NormalizedSort()
	position, appErr := DecodeAdminOrderCursor(page.Cursor, sortMode)
	if appErr != nil {
		return domain.Page[Order]{}, appErr
	}
	filtered := FilterAdminOrders(orders, filter, now)
	if page.Cursor != "" {
		remaining := make([]Order, 0, len(filtered))
		for _, order := range filtered {
			if adminOrderIsAfterCursor(order, position, sortMode) {
				remaining = append(remaining, order)
			}
		}
		filtered = remaining
	}
	return PageAdminOrderItems(filtered, page, sortMode), nil
}

func DecodeAdminOrderCursor(value, sortMode string) (AdminOrderCursorPosition, *domain.AppError) {
	value = strings.TrimSpace(value)
	if value == "" {
		return AdminOrderCursorPosition{}, nil
	}
	body, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return AdminOrderCursorPosition{}, invalidAdminOrderCursorError()
	}
	var payload adminOrderCursor
	if err := json.Unmarshal(body, &payload); err != nil {
		return AdminOrderCursorPosition{}, invalidAdminOrderCursorError()
	}
	payload.Sort = strings.TrimSpace(payload.Sort)
	payload.Value = strings.TrimSpace(payload.Value)
	payload.ID = strings.TrimSpace(payload.ID)
	if payload.Version != adminOrderCursorVersion || payload.Sort != normalizedAdminOrderSort(sortMode) || payload.Value == "" || payload.ID == "" {
		return AdminOrderCursorPosition{}, invalidAdminOrderCursorError()
	}
	if _, err := uuid.Parse(payload.ID); err != nil {
		return AdminOrderCursorPosition{}, invalidAdminOrderCursorError()
	}
	switch payload.Sort {
	case AdminOrderSortAmountAsc, AdminOrderSortAmountDesc:
		if _, ok := NormalizeAdminOrderAmount(payload.Value); !ok {
			return AdminOrderCursorPosition{}, invalidAdminOrderCursorError()
		}
	default:
		if _, err := time.Parse(time.RFC3339Nano, payload.Value); err != nil {
			return AdminOrderCursorPosition{}, invalidAdminOrderCursorError()
		}
	}
	return AdminOrderCursorPosition{Value: payload.Value, ID: payload.ID}, nil
}

func PageAdminOrderItems(items []Order, page domain.PageRequest, sortMode string) domain.Page[Order] {
	page = normalizeAdminOrderPageRequest(page)
	result := domain.Page[Order]{Items: items}
	if len(items) <= page.Limit {
		return result
	}
	visible := append([]Order(nil), items[:page.Limit]...)
	last := visible[len(visible)-1]
	next := encodeAdminOrderCursor(normalizedAdminOrderSort(sortMode), adminOrderCursorValue(last, sortMode), last.ID)
	result.Items = visible
	result.NextCursor = &next
	return result
}

func encodeAdminOrderCursor(sortMode, value, id string) string {
	body, _ := json.Marshal(adminOrderCursor{
		Version: adminOrderCursorVersion,
		Sort:    normalizedAdminOrderSort(sortMode),
		Value:   strings.TrimSpace(value),
		ID:      strings.TrimSpace(id),
	})
	return base64.RawURLEncoding.EncodeToString(body)
}

func adminOrderCursorValue(order Order, sortMode string) string {
	switch normalizedAdminOrderSort(sortMode) {
	case AdminOrderSortCreatedDesc:
		return order.CreatedAt.UTC().Format(time.RFC3339Nano)
	case AdminOrderSortAmountAsc, AdminOrderSortAmountDesc:
		return order.Amount
	default:
		return order.UpdatedAt.UTC().Format(time.RFC3339Nano)
	}
}

func adminOrderIsAfterCursor(order Order, position AdminOrderCursorPosition, sortMode string) bool {
	switch normalizedAdminOrderSort(sortMode) {
	case AdminOrderSortAmountAsc, AdminOrderSortAmountDesc:
		comparison := CompareAdminOrderAmounts(order.Amount, position.Value)
		if comparison != 0 {
			if normalizedAdminOrderSort(sortMode) == AdminOrderSortAmountAsc {
				return comparison > 0
			}
			return comparison < 0
		}
		if normalizedAdminOrderSort(sortMode) == AdminOrderSortAmountAsc {
			return order.ID > position.ID
		}
		return order.ID < position.ID
	case AdminOrderSortCreatedDesc:
		cursorTime, _ := time.Parse(time.RFC3339Nano, position.Value)
		return order.CreatedAt.Before(cursorTime) || order.CreatedAt.Equal(cursorTime) && order.ID < position.ID
	default:
		cursorTime, _ := time.Parse(time.RFC3339Nano, position.Value)
		return order.UpdatedAt.Before(cursorTime) || order.UpdatedAt.Equal(cursorTime) && order.ID < position.ID
	}
}

func normalizedAdminOrderSort(value string) string {
	return (AdminOrderFilter{Sort: value}).NormalizedSort()
}

func normalizeAdminOrderPageRequest(page domain.PageRequest) domain.PageRequest {
	if page.Limit < 1 {
		page.Limit = 20
	}
	if page.Limit > 100 {
		page.Limit = 100
	}
	page.Cursor = strings.TrimSpace(page.Cursor)
	return page
}

func invalidAdminOrderCursorError() *domain.AppError {
	return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Invalid cursor", "分页 cursor 无效。", "cursor", "invalid", "cursor 无效或已过期。")
}

func decimalFilterValue(value string) (*big.Rat, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, false
	}
	parsed, ok := new(big.Rat).SetString(value)
	return parsed, ok
}

func normalizeOrderSearch(value string) string {
	var builder strings.Builder
	for _, character := range strings.ToLower(value) {
		if character >= 'a' && character <= 'z' || character >= '0' && character <= '9' {
			builder.WriteRune(character)
		}
	}
	return builder.String()
}

func containsOrderSearch(query, normalizedQuery string, order Order) bool {
	for _, value := range []string{order.ID, order.OrderNo, order.APIServiceID, order.ServiceTitleSnapshot, order.BuyerUserID, order.SellerUserID} {
		if strings.Contains(strings.ToLower(value), query) {
			return true
		}
	}
	return normalizedQuery != "" && strings.Contains(normalizeOrderSearch(order.OrderNo), normalizedQuery)
}
