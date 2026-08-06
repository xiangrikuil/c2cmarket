package apiquota

import (
	"strings"
	"testing"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/apiorder"
)

func TestParseCredentialCSVAcceptsSeparateTemplates(t *testing.T) {
	tests := []struct {
		name         string
		deliveryKind string
		body         string
		assert       func(t *testing.T, row CredentialImportRow)
	}{
		{
			name:         "api key endpoint",
			deliveryKind: apiorder.DeliveryKindAPIKeyEndpoint,
			body:         "api_base_url,api_key,instructions\nhttps://api.example.com/v1,sk-buyer-one,买家专属接入信息\n",
			assert: func(t *testing.T, row CredentialImportRow) {
				t.Helper()
				if row.APIBaseURL != "https://api.example.com/v1" || row.APIKey != "sk-buyer-one" || row.Username != "" {
					t.Fatalf("unexpected API key row: %+v", row)
				}
			},
		},
		{
			name:         "login account",
			deliveryKind: apiorder.DeliveryKindLoginAccount,
			body:         "panel_login_url,username,password,instructions\nhttps://panel.example.com/login,buyer-one,initial-password,首次登录后修改密码\n",
			assert: func(t *testing.T, row CredentialImportRow) {
				t.Helper()
				if row.PanelLoginURL != "https://panel.example.com/login" || row.Username != "buyer-one" || row.Password != "initial-password" || row.APIKey != "" {
					t.Fatalf("unexpected login row: %+v", row)
				}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rows, appErr := ParseCredentialCSV(strings.NewReader(test.body), test.deliveryKind)
			if appErr != nil {
				t.Fatalf("parse credential CSV: %v", appErr)
			}
			if len(rows) != 1 {
				t.Fatalf("expected one row, got %d", len(rows))
			}
			test.assert(t, rows[0])
		})
	}
}

func TestParseCredentialCSVRejectsWrongTemplateDuplicateAndSecretLeak(t *testing.T) {
	_, appErr := ParseCredentialCSV(strings.NewReader("panel_login_url,username,password,instructions\nhttps://panel.example.com,user,password,note\n"), apiorder.DeliveryKindAPIKeyEndpoint)
	if appErr == nil || appErr.Code != domain.CodeValidationFailed {
		t.Fatalf("expected wrong template validation error, got %v", appErr)
	}

	secret := "sk-duplicate-sensitive-value"
	_, appErr = ParseCredentialCSV(strings.NewReader("api_base_url,api_key,instructions\nhttps://api.example.com/v1,"+secret+",first\nhttps://api.example.com/v1,"+secret+",second\n"), apiorder.DeliveryKindAPIKeyEndpoint)
	if appErr == nil || appErr.Code != domain.CodeValidationFailed {
		t.Fatalf("expected duplicate validation error, got %v", appErr)
	}
	if strings.Contains(appErr.Detail, secret) {
		t.Fatalf("credential error must not echo the raw secret: %q", appErr.Detail)
	}
}

func TestParseCredentialCSVRejectsMoreThanFiveThousandRows(t *testing.T) {
	var body strings.Builder
	body.WriteString("api_base_url,api_key,instructions\n")
	for index := 0; index < maxCredentialCSVRows+1; index++ {
		body.WriteString("https://api.example.com/v1,sk-buyer-")
		body.WriteString(integerText(index))
		body.WriteString(",note\n")
	}
	_, appErr := ParseCredentialCSV(strings.NewReader(body.String()), apiorder.DeliveryKindAPIKeyEndpoint)
	if appErr == nil || !strings.Contains(appErr.Detail, "5000") {
		t.Fatalf("expected row limit validation error, got %v", appErr)
	}
}
