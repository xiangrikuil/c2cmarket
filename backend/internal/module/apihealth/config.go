package apihealth

import (
	"errors"
	"strings"
	"time"
)

var (
	ErrInvalidModel           = errors.New("probe model is required")
	ErrCredentialRequired     = errors.New("probe credential is required before enabling")
	ErrCredentialInvalid      = errors.New("probe credential is invalid")
	ErrInvalidExpectedVersion = errors.New("probe config version is invalid")
)

type ConfigMutation struct {
	Config                   Config
	MeasurementInvalidated   bool
	AuthorizationInvalidated bool
}

func BuildConfigMutation(existing *Config, serviceID, ownerID string, input ConfigInput, now time.Time) (ConfigMutation, error) {
	target, err := NormalizeTarget(input.BaseURL)
	if err != nil {
		return ConfigMutation{}, err
	}
	model := strings.TrimSpace(input.Model)
	if model == "" {
		return ConfigMutation{}, ErrInvalidModel
	}
	if input.Credential != nil && strings.TrimSpace(*input.Credential) == "" {
		return ConfigMutation{}, ErrCredentialInvalid
	}
	credentialConfigured := input.Credential != nil
	if existing != nil && existing.CredentialConfigured {
		credentialConfigured = true
	}
	if input.Enabled && !credentialConfigured {
		return ConfigMutation{}, ErrCredentialRequired
	}
	if existing == nil {
		return ConfigMutation{Config: Config{
			APIServiceID: serviceID, OwnerUserID: ownerID,
			Protocol: ProtocolOpenAIChatCompletionsV1, BaseURL: target.BaseURL,
			NormalizedOrigin: target.Origin, Model: model,
			CredentialConfigured: credentialConfigured, Enabled: input.Enabled,
			AuthorizationStatus: AuthorizationPending,
			MeasurementVersion:  1, Version: 1, CreatedAt: now, UpdatedAt: now,
		}}, nil
	}
	updated := *existing
	measurementChanged := MeasurementIdentityChanged(*existing, target, model)
	updated.Protocol = ProtocolOpenAIChatCompletionsV1
	updated.BaseURL = target.BaseURL
	updated.NormalizedOrigin = target.Origin
	updated.Model = model
	updated.CredentialConfigured = credentialConfigured
	updated.Enabled = input.Enabled
	updated.UpdatedAt = now
	updated.Version++
	if measurementChanged {
		updated.MeasurementVersion++
		updated.AuthorizationStatus = AuthorizationPending
		updated.AuthorizationMethod = ""
		updated.VerifiedOrigin = ""
		updated.VerifiedAt = nil
		updated.ApprovedByAdminID = ""
		updated.ApprovedAt = nil
		updated.RejectionReason = ""
		updated.ChallengeExpiresAt = nil
		updated.LastConfigErrorCode = ""
	}
	return ConfigMutation{
		Config: updated, MeasurementInvalidated: measurementChanged,
		AuthorizationInvalidated: measurementChanged,
	}, nil
}

func IsAuthorized(config Config) bool {
	return (config.AuthorizationStatus == AuthorizationVerified || config.AuthorizationStatus == AuthorizationApproved) &&
		config.VerifiedOrigin == config.NormalizedOrigin && config.VerifiedAt != nil
}
