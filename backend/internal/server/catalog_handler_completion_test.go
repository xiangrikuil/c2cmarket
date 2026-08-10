package server

import (
	"encoding/json"
	"net/http"
	"testing"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/catalog"
)

func TestAPIModelBulkMutationCompletionBuilder(t *testing.T) {
	result := catalog.APIModelBulkMutationResult{
		Created: 2,
		Updated: 1,
		Changed: 3,
		IDs: []string{
			"11111111-1111-4111-8111-111111111111",
			"22222222-2222-4222-8222-222222222222",
		},
	}

	completion, appErr := apiModelBulkMutationCompletionBuilder(result)
	if appErr != nil {
		t.Fatalf("build completion: %v", appErr)
	}
	if completion.Status != http.StatusOK || completion.ResourceType != "api_model_catalog" || completion.ResourceID != result.IDs[0] {
		t.Fatalf("unexpected completion metadata: %+v", completion)
	}

	var response apiModelBulkMutationResponse
	if err := json.Unmarshal(completion.Body, &response); err != nil {
		t.Fatalf("decode completion body: %v", err)
	}
	if response.Created != result.Created || response.Updated != result.Updated || response.Changed != result.Changed {
		t.Fatalf("unexpected completion counts: %+v", response)
	}
	if len(response.IDs) != len(result.IDs) || response.IDs[0] != result.IDs[0] || response.IDs[1] != result.IDs[1] {
		t.Fatalf("completion body must retain every affected ID: %+v", response.IDs)
	}
}

func TestAPIModelBulkMutationCompletionBuilderRejectsEmptyIDs(t *testing.T) {
	completion, appErr := apiModelBulkMutationCompletionBuilder(catalog.APIModelBulkMutationResult{})
	if appErr == nil || appErr.Status != http.StatusInternalServerError || appErr.Code != domain.CodeInternalError {
		t.Fatalf("expected internal error for empty IDs, got completion=%+v error=%+v", completion, appErr)
	}
}
