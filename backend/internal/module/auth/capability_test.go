package auth

import (
	"context"
	"reflect"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
)

func TestProjectCapabilitiesExactMatrix(t *testing.T) {
	student := &StudentEmailClaim{ID: "claim"}
	linuxDo := &LinuxDoBinding{Bound: true}
	tests := []struct {
		name string
		user User
		want []string
	}{
		{name: "no identity", user: User{}, want: []string{}},
		{name: "student", user: User{StudentClaim: student}, want: []string{CapabilityAPIOrderCreate}},
		{name: "linuxdo", user: User{LinuxDoBinding: linuxDo}, want: []string{
			CapabilityAPIOrderCreate,
			CapabilityAPIProbeManage,
			CapabilityAPIQuotaPublish,
			CapabilityAPIServicePublish,
			CapabilityCarpoolApply,
			CapabilityCarpoolPublish,
		}},
		{name: "admin only", user: User{IsAdmin: true}, want: []string{CapabilityAdminAccess}},
		{name: "student admin grant stays ineffective", user: User{StudentClaim: student, IsAdmin: true}, want: []string{CapabilityAPIOrderCreate}},
		{name: "linked student admin", user: User{StudentClaim: student, LinuxDoBinding: linuxDo, IsAdmin: true}, want: AllCapabilities},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := ProjectCapabilities(test.user); !reflect.DeepEqual(got, test.want) {
				t.Fatalf("ProjectCapabilities() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestHydrateCapabilitiesSuppressesStudentOnlyAdminGrant(t *testing.T) {
	studentOnly := HydrateCapabilities(User{StudentClaim: &StudentEmailClaim{}, IsAdmin: true})
	if studentOnly.IsAdmin || HasCapability(studentOnly, CapabilityAdminAccess) {
		t.Fatalf("student-only account retained admin authority: %+v", studentOnly)
	}
	linked := HydrateCapabilities(User{
		StudentClaim: &StudentEmailClaim{}, LinuxDoBinding: &LinuxDoBinding{Bound: true}, IsAdmin: true,
	})
	if !linked.IsAdmin || !HasCapability(linked, CapabilityAdminAccess) {
		t.Fatalf("linux.do-linked student lost an explicit admin grant: %+v", linked)
	}
}

func TestRequireCapabilityReturnsStableSafeError(t *testing.T) {
	err := RequireCapability(User{StudentClaim: &StudentEmailClaim{}}, CapabilityAPIProbeManage)
	if err == nil || err.Code != domain.CodeCapabilityRequired || len(err.FieldErrors) != 1 || err.FieldErrors[0].Message != CapabilityAPIProbeManage {
		t.Fatalf("unexpected capability error: %+v", err)
	}
	if err := RequireCapability(User{StudentClaim: &StudentEmailClaim{}}, CapabilityAPIOrderCreate); err != nil {
		t.Fatalf("student buyer capability denied: %v", err)
	}
}

func TestCreateDevSessionRetainsSyntheticLinuxDoCapabilities(t *testing.T) {
	now := time.Date(2026, 8, 12, 9, 0, 0, 0, time.UTC)
	service := NewService(nil, func() time.Time { return now })
	user, session, appErr := service.CreateDevSession(context.Background(), "dev-buyer", false)
	if appErr != nil {
		t.Fatalf("CreateDevSession() error = %v", appErr)
	}
	if user.LinuxDoBinding == nil || !user.LinuxDoBinding.Bound || !HasCapability(user, CapabilityAPIProbeManage) {
		t.Fatalf("dev session identity was not projected as LinuxDo-bound: %+v", user)
	}
	readUser, _, appErr := service.GetSession(context.Background(), session.ID)
	if appErr != nil {
		t.Fatalf("GetSession() error = %v", appErr)
	}
	if readUser.LinuxDoBinding == nil || !readUser.LinuxDoBinding.Bound || !HasCapability(readUser, CapabilityCarpoolPublish) {
		t.Fatalf("subsequent session read lost dev identity capabilities: %+v", readUser)
	}
}
