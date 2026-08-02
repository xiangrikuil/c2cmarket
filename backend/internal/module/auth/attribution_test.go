package auth

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestNormalizeRegistrationAttribution(t *testing.T) {
	tests := []struct {
		name  string
		input RegistrationAttribution
		want  RegistrationAttribution
	}{
		{
			name: "campaign",
			input: RegistrationAttribution{
				Source:       " linux.do ",
				Medium:       " community ",
				Campaign:     " launch ",
				ReferrerHost: "forum.example.com",
				LandingPath:  "/api-market/service-123?utm_source=linux.do",
			},
			want: RegistrationAttribution{
				SourceType:   RegistrationSourceCampaign,
				Source:       "linux.do",
				Medium:       "community",
				Campaign:     "launch",
				ReferrerHost: "forum.example.com",
				LandingPath:  "/api-market/:id",
			},
		},
		{
			name: "referral",
			input: RegistrationAttribution{
				ReferrerHost: "Example.COM.",
				LandingPath:  "/carpools/abc",
			},
			want: RegistrationAttribution{
				SourceType:   RegistrationSourceReferral,
				Source:       "example.com",
				ReferrerHost: "example.com",
				LandingPath:  "/carpools/:id",
			},
		},
		{
			name:  "direct",
			input: RegistrationAttribution{LandingPath: "/"},
			want: RegistrationAttribution{
				SourceType:  RegistrationSourceDirect,
				Source:      RegistrationSourceDirect,
				LandingPath: "/",
			},
		},
		{
			name: "invalid referrer",
			input: RegistrationAttribution{
				ReferrerHost: "https://example.com/path",
				LandingPath:  "//example.com/redirect",
			},
			want: RegistrationAttribution{
				SourceType:  RegistrationSourceDirect,
				Source:      RegistrationSourceDirect,
				LandingPath: "/",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := NormalizeRegistrationAttribution(test.input)
			if got != test.want {
				t.Fatalf("NormalizeRegistrationAttribution() = %+v, want %+v", got, test.want)
			}
		})
	}
}

func TestNormalizeRegistrationAttributionBoundsAndStripsControlCharacters(t *testing.T) {
	got := NormalizeRegistrationAttribution(RegistrationAttribution{
		Source:      "\n" + strings.Repeat("界", 120) + "\x00",
		Medium:      " social\tmedia ",
		Campaign:    strings.Repeat("x", 101),
		LandingPath: "/unknown/page",
	})

	if utf8.RuneCountInString(got.Source) != 100 {
		t.Fatalf("source length = %d, want 100", utf8.RuneCountInString(got.Source))
	}
	if strings.ContainsAny(got.Source+got.Medium, "\n\t\x00") {
		t.Fatalf("control characters were not removed: %+v", got)
	}
	if got.Medium != "socialmedia" {
		t.Fatalf("medium = %q, want socialmedia", got.Medium)
	}
	if len(got.Campaign) != 100 {
		t.Fatalf("campaign length = %d, want 100", len(got.Campaign))
	}
	if got.LandingPath != "/other" {
		t.Fatalf("landing path = %q, want /other", got.LandingPath)
	}
}

func TestNormalizeAttributionPathKeepsOnlyLowCardinalityRoutes(t *testing.T) {
	tests := map[string]string{
		"/carpools/new":              "/carpools",
		"/carpools/123":              "/carpools/:id",
		"/api-market/new":            "/api-market",
		"/api-market/service-123":    "/api-market/:id",
		"/official-prices?region=cn": "/official-prices",
		"/my/orders/123":             "/my",
		"/not-public/anything":       "/other",
		"not-a-path":                 "/",
	}
	for input, want := range tests {
		if got := NormalizeAttributionPath(input); got != want {
			t.Errorf("NormalizeAttributionPath(%q) = %q, want %q", input, got, want)
		}
	}
}
