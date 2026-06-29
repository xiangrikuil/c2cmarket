package server

import (
	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/contact"
	"github.com/go-chi/chi/v5"
	"net/http"
	"time"
)

type createContactMethodRequest struct {
	Type         string   `json:"type"`
	Label        string   `json:"label"`
	Value        string   `json:"value"`
	DisplayValue string   `json:"displayValue"`
	UsageScopes  []string `json:"usageScopes"`
	IsDefault    bool     `json:"isDefault"`
	Enabled      *bool    `json:"enabled"`
}

type contactMethodResponse struct {
	ID           string   `json:"id"`
	UserID       string   `json:"userId"`
	Type         string   `json:"type"`
	Label        string   `json:"label"`
	MaskedValue  string   `json:"maskedValue"`
	DisplayValue string   `json:"displayValue,omitempty"`
	UsageScopes  []string `json:"usageScopes"`
	IsDefault    bool     `json:"isDefault"`
	Enabled      bool     `json:"enabled"`
	Verified     bool     `json:"verified"`
	CreatedAt    string   `json:"createdAt"`
	UpdatedAt    string   `json:"updatedAt"`
	Version      int64    `json:"version"`
}

type contactMethodListResponse = listResponse[contactMethodResponse]

func (s *Server) handleCreateContactMethod(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	body, req, appErr := decodeStrictJSON[createContactMethodRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}

	s.withIdempotency(w, r, user.ID, "POST /api/v1/contact-methods", body, func() (int, any, string, string, *domain.AppError) {
		value := req.Value
		if value == "" {
			value = req.DisplayValue
		}
		enabled := true
		if req.Enabled != nil {
			enabled = *req.Enabled
		}
		method, errApp := s.app.CreateContactMethod(r.Context(), contact.ContactMethodInput{
			UserID:    user.ID,
			Type:      req.Type,
			Label:     req.Label,
			Value:     value,
			IsDefault: req.IsDefault,
			Enabled:   enabled,
		})
		if errApp != nil {
			return 0, nil, "", "", errApp
		}
		return http.StatusCreated, toContactMethodResponse(method), "contact_method", method.ID, nil
	})
}

func (s *Server) handleMyContactMethods(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	methods, appErr := s.app.ListContactMethods(r.Context(), user.ID)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	items := make([]contactMethodResponse, 0, len(methods))
	for _, method := range methods {
		items = append(items, toContactMethodResponse(method))
	}
	writeJSON(w, http.StatusOK, contactMethodListResponse{Items: items})
}

func (s *Server) handleUpdateContactMethod(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	req, appErr := decodeStrictJSONOnly[createContactMethodRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	value := req.Value
	if value == "" {
		value = req.DisplayValue
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	method, appErr := s.app.UpdateContactMethod(r.Context(), contact.UpdateContactMethodInput{
		UserID:    user.ID,
		MethodID:  chi.URLParam(r, "id"),
		Type:      req.Type,
		Label:     req.Label,
		Value:     value,
		IsDefault: req.IsDefault,
		Enabled:   enabled,
	})
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, toContactMethodResponse(method))
}

func (s *Server) handleDeleteContactMethod(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	method, appErr := s.app.DeleteContactMethod(r.Context(), user.ID, chi.URLParam(r, "id"))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, toContactMethodResponse(method))
}

func (s *Server) handleSetDefaultContactMethod(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	method, appErr := s.app.SetDefaultContactMethod(r.Context(), user.ID, chi.URLParam(r, "id"))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, toContactMethodResponse(method))
}

func (s *Server) handleVerifyContactMethod(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	method, appErr := s.app.VerifyContactMethod(r.Context(), user.ID, chi.URLParam(r, "id"))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, toContactMethodResponse(method))
}

type createContactSessionRequest struct {
	SellerUsername        string `json:"sellerUsername"`
	BuyerContactMethodID  string `json:"buyerContactMethodId"`
	SellerContactMethodID string `json:"sellerContactMethodId"`
	DurationSeconds       int    `json:"durationSeconds"`
}

type contactSessionResponse struct {
	ID       string `json:"id"`
	BuyerID  string `json:"buyerId"`
	SellerID string `json:"sellerId"`
	OpensAt  string `json:"opensAt"`
	EndsAt   string `json:"endsAt"`
}

func (s *Server) handleCreateDevContactSession(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	body, req, appErr := decodeStrictJSON[createContactSessionRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}

	s.withIdempotency(w, r, user.ID, "POST /api/v1/dev/contact-sessions", body, func() (int, any, string, string, *domain.AppError) {
		seller, _, appErr := s.app.CreateDevSession(r.Context(), req.SellerUsername, false)
		if appErr != nil {
			return 0, nil, "", "", appErr
		}
		duration := time.Duration(req.DurationSeconds) * time.Second
		session, errApp := s.app.CreateContactSession(r.Context(), contact.CreateContactSessionInput{
			BuyerUserID:           user.ID,
			SellerUserID:          seller.ID,
			BuyerContactMethodID:  req.BuyerContactMethodID,
			SellerContactMethodID: req.SellerContactMethodID,
			Duration:              duration,
		})
		if errApp != nil {
			return 0, nil, "", "", errApp
		}
		return http.StatusCreated, contactSessionResponse{
			ID:       session.ID,
			BuyerID:  session.BuyerUserID,
			SellerID: session.SellerUserID,
			OpensAt:  session.OpensAt.UTC().Format(time.RFC3339),
			EndsAt:   session.EndsAt.UTC().Format(time.RFC3339),
		}, "contact_session", session.ID, nil
	})
}

type contactSessionContactsResponse struct {
	SessionID string           `json:"sessionId"`
	EndsAt    string           `json:"endsAt"`
	Items     []contactItemDTO `json:"items"`
}

type contactItemDTO struct {
	Side        string `json:"side"`
	SubjectID   string `json:"subjectId"`
	Type        string `json:"type"`
	Label       string `json:"label"`
	Value       string `json:"value"`
	MaskedValue string `json:"maskedValue"`
}

func (s *Server) handleReadContactSession(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	view, appErr := s.app.ReadContactSession(r.Context(), chi.URLParam(r, "id"), user.ID, requestIDFrom(r))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	items := make([]contactItemDTO, 0, len(view.Items))
	for _, item := range view.Items {
		items = append(items, *toContactItemDTO(item))
	}
	writeJSON(w, http.StatusOK, contactSessionContactsResponse{
		SessionID: view.SessionID,
		EndsAt:    view.EndsAt.UTC().Format(time.RFC3339),
		Items:     items,
	})
}
func toContactItemDTO(item contact.ContactItemView) *contactItemDTO {
	return &contactItemDTO{
		Side:        item.Side,
		SubjectID:   item.SubjectID,
		Type:        item.Type,
		Label:       item.Label,
		Value:       item.Value,
		MaskedValue: item.MaskedValue,
	}
}

func toContactMethodResponse(method contact.ContactMethod) contactMethodResponse {
	return contactMethodResponse{
		ID:           method.ID,
		UserID:       method.UserID,
		Type:         method.Type,
		Label:        method.Label,
		MaskedValue:  method.MaskedValue,
		DisplayValue: method.DisplayValue,
		UsageScopes:  []string{"carpool_owner", "api_merchant", "buyer", "dispute"},
		IsDefault:    method.IsDefault,
		Enabled:      method.Enabled,
		Verified:     method.VerifiedAt != nil,
		CreatedAt:    method.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:    method.UpdatedAt.UTC().Format(time.RFC3339),
		Version:      method.Version,
	}
}
