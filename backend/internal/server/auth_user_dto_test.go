package server

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"c2c-market/backend/internal/module/auth"
)

func TestSessionUserDTOExposesOnlySafeNullableStudentClaim(t *testing.T) {
	withoutClaim, err := json.Marshal(toUserDTO(auth.User{ID: "user-without-claim"}))
	if err != nil {
		t.Fatalf("marshal user without student claim: %v", err)
	}
	if !strings.Contains(string(withoutClaim), `"studentClaim":null`) {
		t.Fatalf("session user must expose an explicit nullable studentClaim: %s", withoutClaim)
	}

	claimedAt := time.Date(2026, 8, 13, 9, 30, 0, 0, time.FixedZone("CST", 8*60*60))
	withClaim, err := json.Marshal(toUserDTO(auth.User{
		ID: "student-user",
		StudentClaim: &auth.StudentEmailClaim{
			ID:                  "internal-claim-id",
			UserID:              "student-user",
			NormalizedEmail:     "student@example.edu",
			InstitutionDomainID: "internal-domain-id",
			InstitutionDomain:   "example.edu",
			InstitutionName:     "Example University",
			ClaimedAt:           claimedAt,
		},
	}))
	if err != nil {
		t.Fatalf("marshal student user: %v", err)
	}

	serialized := string(withClaim)
	for _, forbidden := range []string{"internal-claim-id", "student@example.edu", "internal-domain-id", `"userId"`, `"normalizedEmail"`} {
		if strings.Contains(serialized, forbidden) {
			t.Fatalf("session studentClaim leaked %q: %s", forbidden, serialized)
		}
	}
	for _, required := range []string{`"institutionDomain":"example.edu"`, `"institutionName":"Example University"`, `"claimedAt":"2026-08-13T01:30:00Z"`} {
		if !strings.Contains(serialized, required) {
			t.Fatalf("session studentClaim missing %s: %s", required, serialized)
		}
	}
}
