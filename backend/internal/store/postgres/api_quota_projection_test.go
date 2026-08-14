package postgres

import (
	"strings"
	"testing"
)

func TestPublicAPIQuotaOffersQueryProjectsMerchantIdentityAvatar(t *testing.T) {
	t.Parallel()

	for _, fragment := range []string{
		"s.merchant_identity_mode = 'store_alias'",
		"THEN mp.avatar_url",
		"WHEN u.avatar_mode = 'custom_url'",
		"LEFT JOIN linux_do_bindings l ON l.user_id = s.owner_user_id",
	} {
		if !strings.Contains(publicAPIQuotaOffersQuery, fragment) {
			t.Fatalf("expected public quota projection to contain %q", fragment)
		}
	}
}
