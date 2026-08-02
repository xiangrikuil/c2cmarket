package server

import (
	"testing"

	"c2c-market/backend/internal/module/idempotency"
)

func TestRestorePromotionRewardETagFromCachedResponse(t *testing.T) {
	completion := idempotency.Completion{Body: []byte(`{"version":7}`)}
	restorePromotionRewardETag(&completion)
	if completion.Headers["ETag"] != `"7"` {
		t.Fatalf("expected replay ETag to be restored, got %#v", completion.Headers)
	}
}
