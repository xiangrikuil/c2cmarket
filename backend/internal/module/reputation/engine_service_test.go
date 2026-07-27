package reputation

import (
	"context"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
)

type fakeEngineRepository struct {
	*fakeRepository
	snapshots map[SnapshotKey]ReputationSnapshot
	saved     []ReputationSnapshot
	loadCalls int
	saveCalls int
	userIDs   []string
	history   []ReputationHistory
}

type fakeAuditEngineRepository struct {
	*fakeEngineRepository
	evidence AdminReputationEvidence
	auditID  string
	auditNow time.Time
}

func (f *fakeAuditEngineRepository) LoadAdminReputationEvidence(_ context.Context, userID string, now time.Time) (AdminReputationEvidence, *domain.AppError) {
	f.auditID = userID
	f.auditNow = now
	return f.evidence, nil
}

func (f *fakeEngineRepository) LoadReputationSnapshots(_ context.Context, keys []SnapshotKey) (map[SnapshotKey]ReputationSnapshot, *domain.AppError) {
	f.loadCalls++
	result := make(map[SnapshotKey]ReputationSnapshot, len(keys))
	for _, key := range keys {
		if snapshot, exists := f.snapshots[key]; exists {
			result[key] = snapshot
		}
	}
	return result, nil
}

func (f *fakeEngineRepository) SaveReputationSnapshots(_ context.Context, snapshots []ReputationSnapshot) *domain.AppError {
	f.saveCalls++
	f.saved = append(f.saved, snapshots...)
	if f.snapshots == nil {
		f.snapshots = make(map[SnapshotKey]ReputationSnapshot)
	}
	for _, snapshot := range snapshots {
		f.snapshots[snapshot.Key()] = snapshot
	}
	return nil
}

func (f *fakeEngineRepository) ListReputationUserIDs(context.Context) ([]string, *domain.AppError) {
	return append([]string(nil), f.userIDs...), nil
}

func (f *fakeEngineRepository) ListReputationHistory(context.Context, string, int) ([]ReputationHistory, *domain.AppError) {
	return append([]ReputationHistory(nil), f.history...), nil
}

func TestGetManyUsesOneBatchAndReturnsCachedSnapshots(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	key := SnapshotKey{UserID: "user-1", Role: RoleSeller, Scope: ScopeOverall}
	sourceAt := now.Add(-time.Hour)
	repo := &fakeEngineRepository{
		fakeRepository: &fakeRepository{facts: map[string]RawFacts{
			"user-1": {
				UserID: "user-1",
				Seller: RoleFacts{API: ScopeFacts{CompletedCount: 10, SourceDataUpdatedAt: &sourceAt}},
			},
			"user-2": {UserID: "user-2"},
		}},
		snapshots: map[SnapshotKey]ReputationSnapshot{
			key: {
				UserID:              key.UserID,
				Role:                key.Role,
				Scope:               key.Scope,
				Tier:                TierReliable,
				State:               StateActive,
				Confidence:          ConfidenceHigh,
				RuleVersion:         RuleVersion,
				SourceDataUpdatedAt: &sourceAt,
			},
		},
	}
	service := NewService(repo, func() time.Time { return now })

	result, appErr := service.GetMany(context.Background(), []string{"user-1", "user-2"}, RoleSeller, ScopeOverall)
	if appErr != nil {
		t.Fatalf("get many: %v", appErr)
	}
	if len(repo.aggregateIDs) != 2 || repo.loadCalls != 1 || repo.saveCalls != 1 {
		t.Fatalf("expected one aggregate/load/save batch: ids=%#v load=%d save=%d", repo.aggregateIDs, repo.loadCalls, repo.saveCalls)
	}
	if len(repo.saved) != 1 || repo.saved[0].UserID != "user-2" {
		t.Fatalf("only stale/missing snapshot should be saved: %#v", repo.saved)
	}
	if result["user-1"].Tier != TierReliable || result["user-2"].Tier != TierInsufficient {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestGetUserReputationBuildsSixIndependentSnapshotsInOneBatch(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	repo := &fakeEngineRepository{
		fakeRepository: &fakeRepository{facts: map[string]RawFacts{
			"user-1": {
				UserID: "user-1",
				Buyer: RoleFacts{
					Carpool: ScopeFacts{CompletedCount: 3},
					API:     ScopeFacts{CompletedCount: 10},
				},
				Seller: RoleFacts{
					Carpool: ScopeFacts{CompletedCount: 30},
				},
			},
		}},
	}
	service := NewService(repo, func() time.Time { return now })
	snapshots, appErr := service.GetUserReputation(context.Background(), "user-1")
	if appErr != nil {
		t.Fatalf("get reputation: %v", appErr)
	}
	if len(snapshots) != 6 || repo.loadCalls != 1 || len(repo.aggregateIDs) != 1 {
		t.Fatalf("expected six snapshots in one batch: snapshots=%d aggregate=%#v load=%d", len(snapshots), repo.aggregateIDs, repo.loadCalls)
	}
	byKey := make(map[SnapshotKey]ReputationSnapshot, len(snapshots))
	for _, snapshot := range snapshots {
		byKey[snapshot.Key()] = snapshot
	}
	if byKey[SnapshotKey{UserID: "user-1", Role: RoleBuyer, Scope: ScopeCarpool}].Tier != TierNormal {
		t.Fatal("buyer carpool scope did not stay independent")
	}
	if byKey[SnapshotKey{UserID: "user-1", Role: RoleBuyer, Scope: ScopeAPI}].Tier != TierReliable {
		t.Fatal("buyer API scope did not stay independent")
	}
	if byKey[SnapshotKey{UserID: "user-1", Role: RoleSeller, Scope: ScopeAPI}].Tier != TierInsufficient {
		t.Fatal("seller API scope leaked from another role or scope")
	}
}

func TestRecalculateAllUsesBoundedBatches(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	userIDs := make([]string, 0, recalculationBatchSize+1)
	facts := make(map[string]RawFacts, recalculationBatchSize+1)
	for index := 0; index < recalculationBatchSize+1; index++ {
		userID := "user-" + time.Unix(int64(index), 0).UTC().Format("150405")
		userIDs = append(userIDs, userID)
		facts[userID] = RawFacts{UserID: userID}
	}
	repo := &fakeEngineRepository{
		fakeRepository: &fakeRepository{facts: facts},
		userIDs:        userIDs,
	}
	service := NewService(repo, func() time.Time { return now })
	result, appErr := service.RecalculateAll(context.Background())
	if appErr != nil {
		t.Fatalf("recalculate all: %v", appErr)
	}
	if result.RequestedUsers != recalculationBatchSize+1 || result.RebuiltStates != (recalculationBatchSize+1)*6 {
		t.Fatalf("unexpected result: %#v", result)
	}
	if repo.loadCalls != 2 || repo.saveCalls != 2 {
		t.Fatalf("expected two bounded batches, got load=%d save=%d", repo.loadCalls, repo.saveCalls)
	}
}

func TestAdminEvidenceUsesOptionalAuditRepository(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	repo := &fakeAuditEngineRepository{
		fakeEngineRepository: &fakeEngineRepository{fakeRepository: &fakeRepository{}},
		evidence: AdminReputationEvidence{
			Restrictions: []UserRestriction{{ID: "restriction-1"}},
			Outcomes:     []DisputeOutcome{{ID: "outcome-1"}},
			Appeals:      []ReputationAppeal{{ID: "appeal-1"}},
		},
	}
	service := NewService(repo, func() time.Time { return now })

	evidence, appErr := service.AdminEvidence(context.Background(), "user-1")
	if appErr != nil {
		t.Fatalf("admin evidence: %v", appErr)
	}
	if repo.auditID != "user-1" || !repo.auditNow.Equal(now) {
		t.Fatalf("unexpected audit request: id=%q now=%s", repo.auditID, repo.auditNow)
	}
	if len(evidence.Restrictions) != 1 || len(evidence.Outcomes) != 1 || len(evidence.Appeals) != 1 {
		t.Fatalf("unexpected evidence: %#v", evidence)
	}
}

func TestAdminEvidenceWithoutAuditRepositoryReturnsExplicitEmptyCollections(t *testing.T) {
	t.Parallel()

	service := NewService(&fakeEngineRepository{fakeRepository: &fakeRepository{}}, time.Now)
	evidence, appErr := service.AdminEvidence(context.Background(), "user-1")
	if appErr != nil {
		t.Fatalf("admin evidence: %v", appErr)
	}
	if evidence.Restrictions == nil || evidence.Outcomes == nil || evidence.Appeals == nil || evidence.SourceAuthorVerifications == nil {
		t.Fatalf("expected explicit empty evidence collections: %#v", evidence)
	}
}
