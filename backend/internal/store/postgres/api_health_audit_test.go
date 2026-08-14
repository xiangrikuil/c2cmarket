package postgres

import (
	"reflect"
	"testing"

	"c2c-market/backend/internal/module/apihealth"
)

func TestProbeConnectionAuditActionClassifiesRealMutations(t *testing.T) {
	tests := []struct {
		name                       string
		requested                  string
		modelChanged, old, current bool
		want                       string
	}{
		{name: "ordinary update", requested: apihealth.ProbeAuditUpdated, old: true, current: true, want: apihealth.ProbeAuditUpdated},
		{name: "model change", requested: apihealth.ProbeAuditUpdated, modelChanged: true, old: true, current: true, want: apihealth.ProbeAuditModelChanged},
		{name: "enabled", requested: apihealth.ProbeAuditUpdated, old: false, current: true, want: apihealth.ProbeAuditEnabled},
		{name: "disabled", requested: apihealth.ProbeAuditUpdated, old: true, current: false, want: apihealth.ProbeAuditDisabled},
		{name: "verification remains explicit", requested: apihealth.ProbeAuditVerifyFailed, old: true, current: false, want: apihealth.ProbeAuditVerifyFailed},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := probeConnectionAuditAction(test.requested, test.modelChanged, test.old, test.current); got != test.want {
				t.Fatalf("action=%q want=%q", got, test.want)
			}
		})
	}
}

func TestProbeConnectionChangedFieldsContainNamesOnly(t *testing.T) {
	connection := apihealth.Connection{
		Name: "new name", BaseURL: "https://new.example", ProbeModel: "new-model",
		ProbeProtocol: apihealth.ProtocolResponsesV1, ProbeEnvironment: apihealth.ProbeEnvironmentUSWestV1,
		Enabled: true,
	}
	got := probeConnectionChangedFields(
		"old name", "https://old.example", "old-model", apihealth.ProtocolChatCompletionsV1,
		"old-environment", false, connection, true,
	)
	want := []string{"name", "base_url", "credential", "probe_model", "probe_protocol", "environment", "enabled"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("changed fields=%v want=%v", got, want)
	}
}
