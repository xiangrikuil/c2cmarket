package server

import (
	"net/http"
	"time"

	"c2c-market/backend/internal/module/apimarket"
)

type apiAccountPaymentSettingsRequest struct {
	PaymentWindowMinutes int                              `json:"paymentWindowMinutes"`
	PaymentOptions       []apiServicePaymentOptionRequest `json:"paymentOptions"`
}

type apiAccountPaymentSettingsResponse struct {
	PaymentWindowMinutes int                               `json:"paymentWindowMinutes"`
	PaymentOptions       []apiAccountPaymentOptionResponse `json:"paymentOptions"`
	UpdatedAt            string                            `json:"updatedAt"`
}

type apiAccountPaymentOptionResponse struct {
	PaymentMethod        string `json:"paymentMethod"`
	Enabled              bool   `json:"enabled"`
	PaymentInstructions  string `json:"paymentInstructions"`
	PaymentQRCodeDataURL string `json:"paymentQrCodeDataUrl,omitempty"`
}

func (s *Server) handleMyAPIAccountPaymentSettings(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	settings, appErr := s.apiPayment.GetAPIAccountPaymentSettings(r.Context(), user)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, toAPIAccountPaymentSettingsResponse(settings))
}

func (s *Server) handleUpdateMyAPIAccountPaymentSettings(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	req, appErr := decodeStrictJSONOnly[apiAccountPaymentSettingsRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	settings, appErr := s.apiPayment.UpdateAPIAccountPaymentSettings(r.Context(), user, apimarket.UpdateAccountPaymentSettingsInput{
		PaymentWindowMinutes: req.PaymentWindowMinutes,
		PaymentOptions:       toAppPaymentOptionInputs(req.PaymentOptions),
	})
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, toAPIAccountPaymentSettingsResponse(settings))
}

func toAPIAccountPaymentSettingsResponse(settings apimarket.AccountPaymentSettings) apiAccountPaymentSettingsResponse {
	options := make([]apiAccountPaymentOptionResponse, 0, len(settings.PaymentOptions))
	for _, option := range settings.PaymentOptions {
		options = append(options, apiAccountPaymentOptionResponse{
			PaymentMethod:        option.PaymentMethod,
			Enabled:              option.Enabled,
			PaymentInstructions:  option.PaymentInstructions,
			PaymentQRCodeDataURL: option.PaymentQRCodeDataURL,
		})
	}
	updatedAt := ""
	if !settings.UpdatedAt.IsZero() {
		updatedAt = settings.UpdatedAt.UTC().Format(time.RFC3339Nano)
	}
	return apiAccountPaymentSettingsResponse{
		PaymentWindowMinutes: apimarket.DefaultPaymentWindowMinutes,
		PaymentOptions:       options,
		UpdatedAt:            updatedAt,
	}
}
