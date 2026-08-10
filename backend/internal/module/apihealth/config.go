package apihealth

import (
	"errors"
	"strings"
	"time"

	"c2c-market/backend/internal/platform/openaiapi"
)

var (
	ErrInvalidName                 = errors.New("probe connection name is required")
	ErrCredentialRequired          = errors.New("probe credential is required")
	ErrCredentialInvalid           = errors.New("probe credential is invalid")
	ErrProbeModelRequired          = errors.New("probe model is required")
	ErrProbeModelUnavailable       = errors.New("probe model is unavailable")
	ErrPreflightRequired           = errors.New("probe preflight token is required")
	ErrPreflightInvalid            = errors.New("probe preflight token is invalid or expired")
	ErrInsecureHTTPNotAcknowledged = errors.New("insecure HTTP probe risk must be acknowledged")
)

func NewConnection(ownerID string, input ConnectionInput, target openaiapi.BaseURL, result VerificationResult, price PriceSnapshot, now time.Time) (Connection, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" || len([]rune(name)) > 80 {
		return Connection{}, ErrInvalidName
	}
	if input.Credential == nil || strings.TrimSpace(*input.Credential) == "" {
		return Connection{}, ErrCredentialRequired
	}
	connection := Connection{
		OwnerUserID: ownerID, Name: name, BaseURL: target.Raw, NormalizedBaseURL: target.Canonical,
		CredentialConfigured: true, ProbeEnvironment: ProbeEnvironmentUSWestV1, Price: price,
		MeasurementVersion: 1, Version: 1, CreatedAt: now, UpdatedAt: now,
	}
	applyVerification(&connection, input.Enabled, result, now)
	return connection, nil
}

func UpdateConnection(existing Connection, input ConnectionInput, target openaiapi.BaseURL, result *VerificationResult, price *PriceSnapshot, now time.Time) (Connection, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" || len([]rune(name)) > 80 {
		return Connection{}, ErrInvalidName
	}
	if input.Credential != nil && strings.TrimSpace(*input.Credential) == "" {
		return Connection{}, ErrCredentialInvalid
	}
	updated := existing
	updated.Name = name
	updated.BaseURL = target.Raw
	updated.NormalizedBaseURL = target.Canonical
	updated.Enabled = input.Enabled
	updated.UpdatedAt = now
	updated.Version++
	if result != nil {
		oldModel := updated.ProbeModel
		oldProtocol := updated.ProbeProtocol
		measurementChanged := target.Canonical != existing.NormalizedBaseURL || input.Credential != nil ||
			result.ProbeModel != existing.ProbeModel || result.ProbeProtocol != existing.ProbeProtocol ||
			existing.ProbeEnvironment != ProbeEnvironmentUSWestV1
		if measurementChanged {
			updated.MeasurementVersion++
		}
		updated.CredentialConfigured = true
		if price != nil {
			updated.Price = *price
		}
		applyVerification(&updated, input.Enabled, *result, now)
		if oldModel != "" && (oldModel != updated.ProbeModel || oldProtocol != updated.ProbeProtocol) {
			updated.ProbeModelChangedAt = timePointer(now)
		}
	}
	return updated, nil
}

func applyVerification(connection *Connection, enableRequested bool, result VerificationResult, now time.Time) {
	connection.Enabled = false
	connection.VerifiedAt = nil
	connection.LastVerificationErrorCode = result.ErrorCode
	connection.AvailableModels = append([]string(nil), result.AvailableModels...)
	if result.ErrorCode == "" {
		connection.VerificationStatus = VerificationVerified
		connection.VerifiedAt = timePointer(now)
		connection.ProbeModel = result.ProbeModel
		connection.ProbeProtocol = result.ProbeProtocol
		connection.Enabled = enableRequested
		return
	}
	connection.VerificationStatus = VerificationFailed
}

func timePointer(value time.Time) *time.Time {
	copy := value
	return &copy
}
