package operationaudit

import (
	"os"
	"strings"
	"testing"
)

func TestDomainRegistryOnlyDeclaresActionsPresentAtDurableWriters(t *testing.T) {
	writerFiles := []string{
		"../../store/postgres/auth_student.go",
		"../../store/postgres/admin_user.go",
		"../../store/postgres/carpool.go",
		"../../store/postgres/api_market.go",
		"../../store/postgres/api_quota.go",
		"../../store/postgres/contact.go",
	}
	var writerSource strings.Builder
	for _, name := range writerFiles {
		contents, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("read durable writer %s: %v", name, err)
		}
		writerSource.Write(contents)
	}
	for _, definition := range ActionRegistry() {
		if definition.SourceKind != SourceDomain {
			continue
		}
		if !strings.Contains(writerSource.String(), `"`+definition.Action+`"`) &&
			!strings.Contains(writerSource.String(), `'`+definition.Action+`'`) {
			t.Fatalf("domain action %q has no literal at an approved durable writer", definition.Action)
		}
	}
}

func TestRegistryExcludesAuthenticationAndRequestTelemetry(t *testing.T) {
	for _, definition := range ActionRegistry() {
		if definition.SourceKind != SourceDomain {
			continue
		}
		for _, prefix := range []string{
			"login.", "turnstile.", "csrf.", "request.", "capability.",
		} {
			if strings.HasPrefix(definition.Action, prefix) {
				t.Fatalf("non-business telemetry entered registry: %+v", definition)
			}
		}
	}
}

func TestRegistryUsesAdminAuthorityForDualWrittenAccountGovernance(t *testing.T) {
	for _, action := range []string{"user.account_status_changed", "user.admin_permission_changed"} {
		if _, ok := LookupAction(SourceAdmin, action, "user"); !ok {
			t.Fatalf("admin authority is missing %q", action)
		}
		if _, ok := LookupAction(SourceDomain, action, "user"); ok {
			t.Fatalf("dual-written action %q must not also be projected from domain_events", action)
		}
	}
}

func TestRegistryMatchesOfficialPriceAdminWriterActions(t *testing.T) {
	for _, item := range []struct {
		action     string
		targetType string
	}{
		{action: "official_price_record.create", targetType: "official_price_record"},
		{action: "official_price_record.update", targetType: "official_price_record"},
		{action: "official_price_record.take_down", targetType: "official_price_record"},
		{action: "official_price_lead.approve", targetType: "official_price_lead"},
	} {
		definition, ok := LookupAction(SourceAdmin, item.action, item.targetType)
		if !ok {
			t.Fatalf("admin official-price writer action is missing: %+v", item)
		}
		if kinds := AllowedActorKinds(definition); len(kinds) != 1 || kinds[0] != ActorAdmin {
			t.Fatalf("admin official-price action has invalid actor kinds: action=%s kinds=%v", item.action, kinds)
		}
	}
	for _, phantom := range []string{
		"official_price_record.created",
		"official_price_record.updated",
		"official_price_record.taken_down",
	} {
		if _, ok := LookupAction(SourceAdmin, phantom, "official_price_record"); ok {
			t.Fatalf("phantom official-price admin action remains registered: %s", phantom)
		}
	}
}

func TestRegistryIncludesOneStepCarpoolJoin(t *testing.T) {
	definition, ok := LookupAction(SourceDomain, "carpool_application.joined", "carpool_application")
	if !ok {
		t.Fatal("carpool joined writer is missing from registry")
	}
	kinds := AllowedActorKinds(definition)
	if len(kinds) != 1 || kinds[0] != ActorUser {
		t.Fatalf("carpool joined actor kinds = %v, want user", kinds)
	}
}
