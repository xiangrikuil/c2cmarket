package devpersona

import (
	"context"
	"net/http"
	"strings"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/apimarket"
	"c2c-market/backend/internal/module/auth"
	"c2c-market/backend/internal/module/contact"
	"c2c-market/backend/internal/module/profile"
)

const developmentPaymentQRCode = "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII="

type IdentityService interface {
	LoginDevPersonaIdentity(ctx context.Context, profile auth.OAuthProfile, displayName string) (auth.User, *domain.AppError)
	CreateDevPersonaSession(ctx context.Context, userID string, isAdmin bool) (auth.User, auth.Session, *domain.AppError)
	PasswordConfigured(ctx context.Context, userID string) (bool, *domain.AppError)
	SetPassword(ctx context.Context, input auth.SetPasswordInput) *domain.AppError
}

type ProfileService interface {
	MyProfile(ctx context.Context, user auth.User) (profile.UserProfile, *domain.AppError)
	StartEmailVerification(ctx context.Context, user auth.User, input profile.EmailVerificationStartInput) (profile.EmailVerificationChallenge, *domain.AppError)
	ConfirmEmailVerification(ctx context.Context, user auth.User, input profile.EmailVerificationConfirmInput) (profile.UserProfile, *domain.AppError)
	MyMerchantProfile(ctx context.Context, user auth.User) (profile.MerchantProfile, *domain.AppError)
	UpsertMyMerchantProfile(ctx context.Context, user auth.User, input profile.UpsertMerchantProfileInput) (profile.MerchantProfile, *domain.AppError)
}

type ContactService interface {
	ListMethods(ctx context.Context, userID string) ([]contact.ContactMethod, *domain.AppError)
	CreateMethod(ctx context.Context, input contact.ContactMethodInput) (contact.ContactMethod, *domain.AppError)
}

type PaymentService interface {
	GetAccountPaymentSettings(ctx context.Context, user auth.User) (apimarket.AccountPaymentSettings, *domain.AppError)
	UpdateAccountPaymentSettings(ctx context.Context, user auth.User, input apimarket.UpdateAccountPaymentSettingsInput) (apimarket.AccountPaymentSettings, *domain.AppError)
}

type Service struct {
	identity IdentityService
	profiles ProfileService
	contacts ContactService
	payments PaymentService
}

func NewService(identity IdentityService, profiles ProfileService, contacts ContactService, payments PaymentService) *Service {
	return &Service{identity: identity, profiles: profiles, contacts: contacts, payments: payments}
}

func (s *Service) PrepareSession(ctx context.Context, value string) (Result, *domain.AppError) {
	definition, appErr := Parse(value)
	if appErr != nil {
		return Result{}, appErr
	}
	user, appErr := s.identity.LoginDevPersonaIdentity(ctx, oauthProfile(definition), definition.DisplayName)
	if appErr != nil {
		return Result{}, appErr
	}
	if appErr := s.ensureAccountRecovery(ctx, user, definition); appErr != nil {
		return Result{}, appErr
	}
	if definition.Persona != Admin {
		if appErr := s.ensureContacts(ctx, user); appErr != nil {
			return Result{}, appErr
		}
	}
	if definition.Persona == Seller {
		if appErr := s.ensureSellerReadiness(ctx, user, definition); appErr != nil {
			return Result{}, appErr
		}
	}
	user, session, appErr := s.identity.CreateDevPersonaSession(ctx, user.ID, definition.IsAdmin)
	if appErr != nil {
		return Result{}, appErr
	}
	return Result{Persona: definition.Persona, User: user, Session: session}, nil
}

func (s *Service) ensureAccountRecovery(ctx context.Context, user auth.User, definition Definition) *domain.AppError {
	userProfile, appErr := s.profiles.MyProfile(ctx, user)
	if appErr != nil {
		return appErr
	}
	if userProfile.EmailVerifiedAt == nil {
		challenge, appErr := s.profiles.StartEmailVerification(ctx, user, profile.EmailVerificationStartInput{Email: definition.Email})
		if appErr != nil {
			return appErr
		}
		if strings.TrimSpace(challenge.DevCode) == "" {
			return domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Development email code unavailable", "开发邮箱验证码不可用。")
		}
		if _, appErr := s.profiles.ConfirmEmailVerification(ctx, user, profile.EmailVerificationConfirmInput{
			Email: definition.Email,
			Code:  challenge.DevCode,
		}); appErr != nil {
			return appErr
		}
	}

	configured, appErr := s.identity.PasswordConfigured(ctx, user.ID)
	if appErr != nil {
		return appErr
	}
	if configured {
		return nil
	}
	return s.identity.SetPassword(ctx, auth.SetPasswordInput{UserID: user.ID, NewPassword: SharedPassword})
}

func (s *Service) ensureContacts(ctx context.Context, user auth.User) *domain.AppError {
	methods, appErr := s.contacts.ListMethods(ctx, user.ID)
	if appErr != nil {
		return appErr
	}
	enabledTypes := make(map[string]bool, len(methods))
	hasDefault := false
	for _, method := range methods {
		if !method.Enabled {
			continue
		}
		enabledTypes[method.Type] = true
		hasDefault = hasDefault || method.IsDefault
	}
	fixtures := []contact.ContactMethodInput{
		{UserID: user.ID, Type: "linuxdo", Label: "linux.do 私信", Value: "@" + user.Username, Enabled: true, IsDefault: !hasDefault},
		{UserID: user.ID, Type: "wechat", Label: "微信", Value: user.Username + "-wechat", Enabled: true},
	}
	for _, fixture := range fixtures {
		if enabledTypes[fixture.Type] {
			continue
		}
		if _, appErr := s.contacts.CreateMethod(ctx, fixture); appErr != nil {
			return appErr
		}
		hasDefault = hasDefault || fixture.IsDefault
	}
	return nil
}

func (s *Service) ensureSellerReadiness(ctx context.Context, user auth.User, definition Definition) *domain.AppError {
	merchant, appErr := s.profiles.MyMerchantProfile(ctx, user)
	if appErr != nil {
		if appErr.Code != domain.CodeObjectNotFound {
			return appErr
		}
		if _, appErr := s.profiles.UpsertMyMerchantProfile(ctx, user, profile.UpsertMerchantProfileInput{
			Slug:        user.Username,
			DisplayName: definition.DisplayName + "店铺",
		}); appErr != nil {
			return appErr
		}
	} else if merchant.Status != "active" {
		if _, appErr := s.profiles.UpsertMyMerchantProfile(ctx, user, profile.UpsertMerchantProfileInput{
			Slug: merchant.Slug, DisplayName: merchant.DisplayName, AvatarURL: merchant.AvatarURL,
		}); appErr != nil {
			return appErr
		}
	}

	settings, appErr := s.payments.GetAccountPaymentSettings(ctx, user)
	if appErr != nil {
		return appErr
	}
	if apimarket.HasSingleUsableAccountPaymentOption(settings.PaymentOptions) {
		return nil
	}
	_, appErr = s.payments.UpdateAccountPaymentSettings(ctx, user, apimarket.UpdateAccountPaymentSettingsInput{
		PaymentWindowMinutes: apimarket.DefaultPaymentWindowMinutes,
		PaymentOptions: []apimarket.PaymentOptionInput{
			{PaymentMethod: apimarket.PaymentMethodWechat, Enabled: true, PaymentQRCodeDataURL: developmentPaymentQRCode},
			{PaymentMethod: apimarket.PaymentMethodAlipay},
		},
	})
	return appErr
}

func oauthProfile(definition Definition) auth.OAuthProfile {
	subject := "c2cmarket-dev-persona-" + string(definition.Persona) + "-v1"
	return auth.OAuthProfile{
		Provider:        "linux_do",
		Subject:         subject,
		Username:        definition.Username,
		DisplayName:     definition.DisplayName,
		TrustLevel:      3,
		LinuxDoUserID:   subject,
		LinuxDoUsername: definition.Username,
	}
}
