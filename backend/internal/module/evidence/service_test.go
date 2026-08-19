package evidence

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"io"
	"net/http"
	"sync"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/idempotency"
)

type repositoryStub struct {
	mu            sync.Mutex
	createErr     *domain.AppError
	authorized    Asset
	authorizedErr *domain.AppError
	candidates    []DestroyCandidate
	claimErr      *domain.AppError
	markErr       *domain.AppError
	created       []Asset
	marked        []string
	claimCalls    int
	claimNotify   chan struct{}
}

func (r *repositoryStub) CreateReadyAssets(_ context.Context, assets []Asset) *domain.AppError {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.created = append(r.created, assets...)
	return r.createErr
}

func (r *repositoryStub) AuthorizedAsset(context.Context, string, string, bool) (Asset, *domain.AppError) {
	return r.authorized, r.authorizedErr
}

func (*repositoryStub) QuarantineAssetWithIdempotency(context.Context, idempotency.Entry, AdminQuarantineInput, time.Time, AdminQuarantineCompletionBuilder) (AdminQuarantineResult, idempotency.Completion, *domain.AppError) {
	return AdminQuarantineResult{}, idempotency.Completion{}, nil
}

func (r *repositoryStub) ClaimDestroyCandidates(context.Context, time.Time, int) ([]DestroyCandidate, *domain.AppError) {
	r.mu.Lock()
	r.claimCalls++
	notify := r.claimNotify
	items := append([]DestroyCandidate(nil), r.candidates...)
	err := r.claimErr
	r.mu.Unlock()
	if notify != nil {
		select {
		case notify <- struct{}{}:
		default:
		}
	}
	return items, err
}

func (r *repositoryStub) MarkDestroyed(_ context.Context, assetID string, _ time.Time) *domain.AppError {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.markErr != nil {
		return r.markErr
	}
	r.marked = append(r.marked, assetID)
	return nil
}

type objectStoreStub struct {
	mu          sync.Mutex
	objects     map[string][]byte
	putErr      error
	getErr      error
	deleteErrs  []error
	deleteCalls []string
}

func (s *objectStoreStub) Put(_ context.Context, key, _ string, body []byte) error {
	if s.putErr != nil {
		return s.putErr
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.objects == nil {
		s.objects = make(map[string][]byte)
	}
	s.objects[key] = append([]byte(nil), body...)
	return nil
}

func (s *objectStoreStub) Get(_ context.Context, key string) (Object, error) {
	if s.getErr != nil {
		return Object{}, s.getErr
	}
	s.mu.Lock()
	body, ok := s.objects[key]
	s.mu.Unlock()
	if !ok {
		return Object{}, ErrObjectNotFound
	}
	return Object{Body: io.NopCloser(bytes.NewReader(body)), ContentType: "image/png", Size: int64(len(body))}, nil
}

func (s *objectStoreStub) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deleteCalls = append(s.deleteCalls, key)
	if len(s.deleteErrs) > 0 {
		err := s.deleteErrs[0]
		s.deleteErrs = s.deleteErrs[1:]
		if err != nil {
			return err
		}
	}
	delete(s.objects, key)
	return nil
}

func TestServiceUploadDisabledAndRepositoryFailureCompensation(t *testing.T) {
	if _, appErr := NewService(nil, nil, nil).Upload(t.Context(), UploadInput{}); appErr == nil || appErr.Status != http.StatusServiceUnavailable || appErr.Code != domain.CodeCapabilityUnavailable {
		t.Fatalf("disabled upload returned %#v", appErr)
	}
	readOnly := NewServiceWithUploadCapability(&repositoryStub{}, &objectStoreStub{}, time.Now, false)
	if !readOnly.Enabled() || readOnly.UploadEnabled() {
		t.Fatalf("read-only evidence capability state is invalid: enabled=%t upload=%t", readOnly.Enabled(), readOnly.UploadEnabled())
	}
	if _, appErr := readOnly.Upload(t.Context(), UploadInput{}); appErr == nil || appErr.Status != http.StatusServiceUnavailable || appErr.Code != domain.CodeCapabilityUnavailable {
		t.Fatalf("read-only evidence accepted upload: %#v", appErr)
	}

	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	repo := &repositoryStub{createErr: domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "store failed", "store failed")}
	objects := &objectStoreStub{}
	service := NewService(repo, objects, func() time.Time { return now })
	_, appErr := service.Upload(t.Context(), UploadInput{
		APIOrderID: "order-1", UploaderUserID: "user-1", Kind: KindAPIError,
		Files: []io.Reader{bytes.NewReader(testPNG(t))}, RedactionConfirmed: true,
	})
	if appErr != repo.createErr {
		t.Fatalf("expected repository failure, got %#v", appErr)
	}
	if len(repo.created) != 1 || len(objects.deleteCalls) != 1 || len(objects.objects) != 0 {
		t.Fatalf("object compensation failed: created=%d deletes=%v objects=%v", len(repo.created), objects.deleteCalls, objects.objects)
	}
	asset := repo.created[0]
	if asset.UnboundExpiresAt == nil || !asset.UnboundExpiresAt.Equal(now.Add(24*time.Hour)) {
		t.Fatalf("unexpected unbound retention: %#v", asset.UnboundExpiresAt)
	}
}

func TestServiceOpenMapsMissingAndTransientObjectFailures(t *testing.T) {
	repo := &repositoryStub{authorized: Asset{ID: "asset-1", ObjectKey: "private/asset-1.png"}}
	objects := &objectStoreStub{}
	service := NewService(repo, objects, time.Now)

	if _, appErr := service.Open(t.Context(), "asset-1", "viewer-1", false); appErr == nil || appErr.Status != http.StatusGone || appErr.Code != domain.CodeObjectNotFound {
		t.Fatalf("missing object returned %#v", appErr)
	}
	objects.getErr = errors.New("temporary S3 failure")
	if _, appErr := service.Open(t.Context(), "asset-1", "viewer-1", false); appErr == nil || appErr.Status != http.StatusServiceUnavailable || appErr.Code != domain.CodeCapabilityUnavailable {
		t.Fatalf("transient object failure returned %#v", appErr)
	}
}

func TestServiceCleanupRetriesObjectAndMetadataFailures(t *testing.T) {
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	repo := &repositoryStub{candidates: []DestroyCandidate{{ID: "asset-1", ObjectKey: "private/asset-1.png"}}}
	objects := &objectStoreStub{objects: map[string][]byte{"private/asset-1.png": {1}}, deleteErrs: []error{errors.New("delete failed"), nil}}
	service := NewService(repo, objects, func() time.Time { return now })

	first, appErr := service.Cleanup(t.Context(), 10)
	if appErr != nil || first.Claimed != 1 || first.Failed != 1 || first.Destroyed != 0 || len(repo.marked) != 0 {
		t.Fatalf("unexpected failed delete result=%+v err=%v marked=%v", first, appErr, repo.marked)
	}
	second, appErr := service.Cleanup(t.Context(), 10)
	if appErr != nil || second.Destroyed != 1 || len(repo.marked) != 1 {
		t.Fatalf("delete retry did not converge result=%+v err=%v marked=%v", second, appErr, repo.marked)
	}

	repo.markErr = domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "mark failed", "mark failed")
	third, appErr := service.Cleanup(t.Context(), 10)
	if appErr != nil || third.Failed != 1 || third.Destroyed != 0 {
		t.Fatalf("metadata failure must remain retryable result=%+v err=%v", third, appErr)
	}
	repo.markErr = nil
	fourth, appErr := service.Cleanup(t.Context(), 10)
	if appErr != nil || fourth.Destroyed != 1 || len(repo.marked) != 2 {
		t.Fatalf("metadata retry did not converge result=%+v err=%v marked=%v", fourth, appErr, repo.marked)
	}
}

func testPNG(t *testing.T) []byte {
	t.Helper()
	var out bytes.Buffer
	if err := png.Encode(&out, image.NewRGBA(image.Rect(0, 0, 4, 4))); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}
