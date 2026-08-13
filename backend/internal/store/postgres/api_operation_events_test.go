package postgres

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAPIQuotaCredentialImportMetadataIsStrictlyNonSensitive(t *testing.T) {
	metadata := apiQuotaCredentialImportMetadata(3, " api_key_endpoint ")
	if len(metadata) != 2 || metadata["importedCount"] != 3 || metadata["deliveryKind"] != "api_key_endpoint" {
		t.Fatalf("unexpected credential import metadata: %#v", metadata)
	}
	encoded, err := json.Marshal(metadata)
	if err != nil {
		t.Fatalf("marshal metadata: %v", err)
	}
	text := strings.ToLower(string(encoded))
	for _, forbidden := range []string{"csv", "credential", "fingerprint", "api_key", "password", "token", "instruction", "base_url"} {
		if strings.Contains(text, `"`+forbidden+`"`) {
			t.Fatalf("credential audit metadata contains forbidden key %q: %s", forbidden, encoded)
		}
	}
}
