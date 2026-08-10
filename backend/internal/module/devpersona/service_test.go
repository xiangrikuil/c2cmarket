package devpersona

import (
	"context"
	"testing"
	"time"

	"c2c-market/backend/internal/module/apimarket"
	"c2c-market/backend/internal/module/auth"
	"c2c-market/backend/internal/module/contact"
	"c2c-market/backend/internal/module/profile"
)

type testServices struct {
	personas *Service
	identity *auth.Service
	profiles *profile.Service
	contacts *contact.Service
	payments *apimarket.Manager
}

func newTestServices() testServices {
	now := func() time.Time { return time.Date(2026, 8, 10, 8, 0, 0, 0, time.UTC) }
	identity := auth.NewService(nil, now)
	profiles := profile.NewService(nil, now)
	contacts := contact.NewService(nil, now)
	payments := apimarket.NewManager(nil, nil, contacts, now)
	return testServices{
		personas: NewService(identity, profiles, contacts, payments),
		identity: identity,
		profiles: profiles,
		contacts: contacts,
		payments: payments,
	}
}

func TestPrepareSessionCreatesReadyPersonasIdempotently(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	for _, persona := range Values() {
		persona := persona
		t.Run(string(persona), func(t *testing.T) {
			services := newTestServices()
			first, appErr := services.personas.PrepareSession(ctx, string(persona))
			if appErr != nil {
				t.Fatalf("prepare %s: %v", persona, appErr)
			}
			second, appErr := services.personas.PrepareSession(ctx, string(persona))
			if appErr != nil {
				t.Fatalf("prepare %s again: %v", persona, appErr)
			}
			if first.User.ID != second.User.ID || first.User.Username != "dev-"+string(persona) {
				t.Fatalf("persona identity changed: first=%+v second=%+v", first.User, second.User)
			}
			if second.User.IsAdmin != (persona == Admin) {
				t.Fatalf("unexpected admin permission for %s: %+v", persona, second.User)
			}
			userProfile, appErr := services.profiles.MyProfile(ctx, second.User)
			if appErr != nil || userProfile.EmailVerifiedAt == nil {
				t.Fatalf("persona email not verified: profile=%+v err=%v", userProfile, appErr)
			}
			passwordConfigured, appErr := services.identity.PasswordConfigured(ctx, second.User.ID)
			if appErr != nil || !passwordConfigured {
				t.Fatalf("persona password not configured: configured=%v err=%v", passwordConfigured, appErr)
			}

			methods, appErr := services.contacts.ListMethods(ctx, second.User.ID)
			if appErr != nil {
				t.Fatalf("list contacts: %v", appErr)
			}
			if persona == Admin && len(methods) != 0 {
				t.Fatalf("admin should not receive business contacts: %+v", methods)
			}
			if persona != Admin {
				assertReadyContacts(t, methods)
			}
			if persona == Seller {
				assertReadySeller(t, services, second.User)
			}
		})
	}
}

func TestPrepareSessionPreservesUsableSellerData(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	services := newTestServices()
	prepared, appErr := services.personas.PrepareSession(ctx, string(Seller))
	if appErr != nil {
		t.Fatalf("prepare seller: %v", appErr)
	}

	methods, _ := services.contacts.ListMethods(ctx, prepared.User.ID)
	wechat := contactByType(methods, "wechat")
	if _, appErr := services.contacts.UpdateMethod(ctx, contact.UpdateContactMethodInput{
		UserID: prepared.User.ID, MethodID: wechat.ID, Type: "wechat", Label: "工作微信",
		Value: "seller-custom-wechat", Enabled: true, IsDefault: wechat.IsDefault,
	}); appErr != nil {
		t.Fatalf("customize seller contact: %v", appErr)
	}
	if _, appErr := services.profiles.UpsertMyMerchantProfile(ctx, prepared.User, profile.UpsertMerchantProfileInput{
		Slug: "custom-seller-shop", DisplayName: "本地自定义店铺",
	}); appErr != nil {
		t.Fatalf("customize merchant: %v", appErr)
	}
	if _, appErr := services.payments.UpdateAccountPaymentSettings(ctx, prepared.User, apimarket.UpdateAccountPaymentSettingsInput{
		PaymentWindowMinutes: apimarket.DefaultPaymentWindowMinutes,
		PaymentOptions: []apimarket.PaymentOptionInput{
			{PaymentMethod: apimarket.PaymentMethodWechat},
			{PaymentMethod: apimarket.PaymentMethodAlipay, Enabled: true, PaymentQRCodeDataURL: developmentPaymentQRCode},
		},
	}); appErr != nil {
		t.Fatalf("customize payment settings: %v", appErr)
	}
	if appErr := services.identity.SetPassword(ctx, auth.SetPasswordInput{
		UserID: prepared.User.ID, CurrentPassword: SharedPassword, NewPassword: "CustomDev#2027",
	}); appErr != nil {
		t.Fatalf("customize password: %v", appErr)
	}

	if _, appErr := services.personas.PrepareSession(ctx, string(Seller)); appErr != nil {
		t.Fatalf("prepare customized seller again: %v", appErr)
	}
	methods, _ = services.contacts.ListMethods(ctx, prepared.User.ID)
	if got := contactByType(methods, "wechat").DisplayValue; got != "seller-custom-wechat" {
		t.Fatalf("seller contact was overwritten: %q", got)
	}
	merchant, _ := services.profiles.MyMerchantProfile(ctx, prepared.User)
	if merchant.Slug != "custom-seller-shop" || merchant.DisplayName != "本地自定义店铺" {
		t.Fatalf("merchant was overwritten: %+v", merchant)
	}
	settings, _ := services.payments.GetAccountPaymentSettings(ctx, prepared.User)
	if !settings.PaymentOptions[1].Enabled || settings.PaymentOptions[0].Enabled {
		t.Fatalf("payment settings were overwritten: %+v", settings.PaymentOptions)
	}
	if _, _, appErr := services.identity.LoginWithPassword(ctx, prepared.User.Username, "CustomDev#2027"); appErr != nil {
		t.Fatalf("custom password was overwritten: %v", appErr)
	}
}

func TestParseRejectsUnknownPersona(t *testing.T) {
	t.Parallel()
	for _, value := range []string{"developer", "Buyer", " buyer "} {
		if _, appErr := Parse(value); appErr == nil || appErr.Status != 422 {
			t.Fatalf("expected strict persona validation for %q, got %+v", value, appErr)
		}
	}
}

func assertReadyContacts(t *testing.T, methods []contact.ContactMethod) {
	t.Helper()
	if len(methods) != 2 {
		t.Fatalf("expected two idempotent contacts, got %+v", methods)
	}
	for _, methodType := range []string{"linuxdo", "wechat"} {
		method := contactByType(methods, methodType)
		if method.ID == "" || !method.Enabled {
			t.Fatalf("contact %s is not ready: %+v", methodType, method)
		}
	}
}

func assertReadySeller(t *testing.T, services testServices, user auth.User) {
	t.Helper()
	merchant, appErr := services.profiles.MyMerchantProfile(context.Background(), user)
	if appErr != nil || merchant.ID == "" || merchant.Status != "active" {
		t.Fatalf("seller merchant not ready: merchant=%+v err=%v", merchant, appErr)
	}
	settings, appErr := services.payments.GetAccountPaymentSettings(context.Background(), user)
	if appErr != nil || !apimarket.HasSingleUsableAccountPaymentOption(settings.PaymentOptions) {
		t.Fatalf("seller payment not ready: settings=%+v err=%v", settings, appErr)
	}
}

func contactByType(methods []contact.ContactMethod, methodType string) contact.ContactMethod {
	for _, method := range methods {
		if method.Type == methodType {
			return method
		}
	}
	return contact.ContactMethod{}
}
