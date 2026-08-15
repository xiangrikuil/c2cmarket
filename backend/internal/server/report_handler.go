package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/auth"
	"c2c-market/backend/internal/module/evidence"
	"c2c-market/backend/internal/module/idempotency"
	"c2c-market/backend/internal/module/report"

	"github.com/go-chi/chi/v5"
)

type createReportRequest struct {
	TargetType       string `json:"targetType"`
	TargetID         string `json:"targetId"`
	TargetLabel      string `json:"targetLabel"`
	ReportedUsername string `json:"reportedUsername"`
	ReasonCode       string `json:"reasonCode"`
	Title            string `json:"title"`
	Description      string `json:"description"`
}

type createAppealRequest struct {
	ReportID         string   `json:"reportId"`
	DisputeID        string   `json:"disputeId"`
	TargetType       string   `json:"targetType"`
	TargetID         string   `json:"targetId"`
	Title            string   `json:"title"`
	Statement        string   `json:"statement"`
	EvidenceAssetIDs []string `json:"evidenceAssetIds"`
}

type reportActionRequest struct {
	Reason              string                `json:"reason"`
	PublicSummary       string                `json:"publicSummary"`
	PublicResultCode    string                `json:"publicResultCode"`
	PublicResult        string                `json:"publicResult"`
	RequestedFromUserID string                `json:"requestedFromUserId"`
	Remedy              *disputeRemedyRequest `json:"remedy"`
}

type disputeRemedyRequest struct {
	Action            string `json:"action"`
	AmountCNY         string `json:"amountCny"`
	ResponsibleUserID string `json:"responsibleUserId"`
	Instructions      string `json:"instructions"`
	DueAt             string `json:"dueAt"`
}

type infoSupplementRequest struct {
	OpenInfoRequestID string   `json:"openInfoRequestId"`
	Body              string   `json:"body"`
	EvidenceAssetIDs  []string `json:"evidenceAssetIds"`
}

type disputeMessageRequest struct {
	Body             string   `json:"body"`
	EvidenceAssetIDs []string `json:"evidenceAssetIds"`
}

type disputeSettlementProposalRequest struct {
	Resolution          string `json:"resolution"`
	AmountCNY           string `json:"amountCny"`
	Terms               string `json:"terms"`
	FulfillmentRequired bool   `json:"fulfillmentRequired"`
	ResponsibleUserID   string `json:"responsibleUserId"`
	BeneficiaryUserID   string `json:"beneficiaryUserId"`
	DueAt               string `json:"dueAt"`
}

type disputeParticipantReasonRequest struct {
	Reason           string   `json:"reason"`
	EvidenceAssetIDs []string `json:"evidenceAssetIds"`
}

type disputeEscalationRequest struct {
	NegotiationChannels       []string `json:"negotiationChannels"`
	NegotiationEndedConfirmed bool     `json:"negotiationEndedConfirmed"`
	NegotiationSummary        string   `json:"negotiationSummary"`
	RequestedPlatformAction   string   `json:"requestedPlatformAction"`
	EvidenceAssetIDs          []string `json:"evidenceAssetIds"`
}

type disputeRemedyClaimRequest struct {
	Note             string   `json:"note"`
	EvidenceAssetIDs []string `json:"evidenceAssetIds"`
}

type infoSupplementResponse struct {
	ID                  string `json:"id"`
	InfoRequestID       string `json:"infoRequestId"`
	SubmittedByUserID   string `json:"submittedByUserId"`
	SubmittedByUsername string `json:"submittedByUsername"`
	SubmittedByName     string `json:"submittedByName"`
	Body                string `json:"body"`
	CreatedAt           string `json:"createdAt"`
}

type reportResponse struct {
	ID                  string                   `json:"id"`
	ReporterUserID      string                   `json:"reporterUserId,omitempty"`
	ReporterUsername    string                   `json:"reporterUsername"`
	ReporterName        string                   `json:"reporterName"`
	TargetType          string                   `json:"targetType"`
	TargetID            string                   `json:"targetId"`
	CanonicalTargetType string                   `json:"canonicalTargetType"`
	CanonicalTargetID   string                   `json:"canonicalTargetId"`
	TargetLabel         string                   `json:"targetLabel"`
	TargetSnapshotJSON  string                   `json:"targetSnapshotJson,omitempty"`
	ReportedUsername    string                   `json:"reportedUsername"`
	ReasonCode          string                   `json:"reasonCode"`
	Title               string                   `json:"title"`
	Description         string                   `json:"description,omitempty"`
	Status              string                   `json:"status"`
	AdminReason         string                   `json:"adminReason,omitempty"`
	HandledByAdminID    string                   `json:"handledByAdminId,omitempty"`
	HandledAt           *string                  `json:"handledAt,omitempty"`
	DisputeID           string                   `json:"disputeId,omitempty"`
	CreatedAt           string                   `json:"createdAt"`
	UpdatedAt           string                   `json:"updatedAt"`
	Version             int64                    `json:"version"`
	CanSupplement       *bool                    `json:"canSupplement,omitempty"`
	OpenInfoRequestID   string                   `json:"openInfoRequestId,omitempty"`
	Supplements         []infoSupplementResponse `json:"supplements,omitempty"`
}

type disputeResponse struct {
	ViewerUserID              string                       `json:"viewerUserId,omitempty"`
	ID                        string                       `json:"id"`
	APIOrderID                string                       `json:"apiOrderId,omitempty"`
	Active                    bool                         `json:"active"`
	ReportID                  string                       `json:"reportId,omitempty"`
	TargetType                string                       `json:"targetType"`
	TargetID                  string                       `json:"targetId"`
	TargetLabel               string                       `json:"targetLabel"`
	PrimaryUserID             string                       `json:"primaryUserId,omitempty"`
	PrimaryUsername           string                       `json:"primaryUsername"`
	PrimaryDisplayName        string                       `json:"primaryDisplayName"`
	CounterpartyUserID        string                       `json:"counterpartyUserId,omitempty"`
	CounterpartyUsername      string                       `json:"counterpartyUsername"`
	CounterpartyName          string                       `json:"counterpartyName"`
	SubjectUserID             string                       `json:"subjectUserId,omitempty"`
	SubjectUsername           string                       `json:"subjectUsername,omitempty"`
	SubjectName               string                       `json:"subjectName,omitempty"`
	Status                    string                       `json:"status"`
	IssueCode                 string                       `json:"issueCode,omitempty"`
	RequestedResolution       string                       `json:"requestedResolution,omitempty"`
	RequestedAmountCNY        string                       `json:"requestedAmountCny,omitempty"`
	IssueOccurredAt           *string                      `json:"issueOccurredAt,omitempty"`
	PublicSummary             string                       `json:"publicSummary"`
	PublicResultCode          string                       `json:"publicResultCode"`
	PublicResult              string                       `json:"publicResult"`
	AdminReason               string                       `json:"adminReason,omitempty"`
	OpenedByAdminID           string                       `json:"openedByAdminId,omitempty"`
	OpenedAt                  string                       `json:"openedAt"`
	ResolvedAt                *string                      `json:"resolvedAt,omitempty"`
	ClosedAt                  *string                      `json:"closedAt,omitempty"`
	FinalReason               string                       `json:"finalReason,omitempty"`
	AppealExpiresAt           *string                      `json:"appealExpiresAt,omitempty"`
	AdverselyAffectedIDs      []string                     `json:"adverselyAffectedUserIds,omitempty"`
	NegotiationChannels       []string                     `json:"negotiationChannels,omitempty"`
	NegotiationEndedConfirmed bool                         `json:"negotiationEndedConfirmed"`
	NegotiationSummary        string                       `json:"negotiationSummary,omitempty"`
	RequestedPlatformAction   string                       `json:"requestedPlatformAction,omitempty"`
	EscalatedByUserID         string                       `json:"escalatedByUserId,omitempty"`
	EscalatedAt               *string                      `json:"escalatedAt,omitempty"`
	CreatedAt                 string                       `json:"createdAt"`
	UpdatedAt                 string                       `json:"updatedAt"`
	Version                   int64                        `json:"version"`
	CanAppeal                 *bool                        `json:"canAppeal,omitempty"`
	CanSupplement             *bool                        `json:"canSupplement,omitempty"`
	OpenInfoRequestID         string                       `json:"openInfoRequestId,omitempty"`
	Supplements               []infoSupplementResponse     `json:"supplements,omitempty"`
	Messages                  []disputeMessageResponse     `json:"messages,omitempty"`
	SettlementProposals       []settlementProposalResponse `json:"settlementProposals,omitempty"`
	Remedies                  []disputeRemedyResponse      `json:"remedies,omitempty"`
	Evidence                  []evidenceReferenceResponse  `json:"evidence,omitempty"`
}

type disputeMessageResponse struct {
	ID           string `json:"id"`
	SenderUserID string `json:"senderUserId"`
	Body         string `json:"body"`
	CreatedAt    string `json:"createdAt"`
}

type settlementProposalResponse struct {
	ID                  string  `json:"id"`
	ProposedByUserID    string  `json:"proposedByUserId"`
	Resolution          string  `json:"resolution"`
	AmountCNY           string  `json:"amountCny,omitempty"`
	Terms               string  `json:"terms"`
	FulfillmentRequired bool    `json:"fulfillmentRequired"`
	ResponsibleUserID   string  `json:"responsibleUserId,omitempty"`
	BeneficiaryUserID   string  `json:"beneficiaryUserId,omitempty"`
	DueAt               *string `json:"dueAt,omitempty"`
	Status              string  `json:"status"`
	AcceptedByUserID    string  `json:"acceptedByUserId,omitempty"`
	AcceptedAt          *string `json:"acceptedAt,omitempty"`
	RejectedByUserID    string  `json:"rejectedByUserId,omitempty"`
	RejectedAt          *string `json:"rejectedAt,omitempty"`
	SupersededReason    string  `json:"supersededReason,omitempty"`
	CreatedAt           string  `json:"createdAt"`
	UpdatedAt           string  `json:"updatedAt"`
	Version             int64   `json:"version"`
}

type disputeRemedyResponse struct {
	ID                    string  `json:"id"`
	Action                string  `json:"action"`
	AmountCNY             string  `json:"amountCny,omitempty"`
	Currency              string  `json:"currency"`
	ResponsibleUserID     string  `json:"responsibleUserId"`
	BeneficiaryUserID     string  `json:"beneficiaryUserId"`
	Instructions          string  `json:"instructions"`
	Status                string  `json:"status"`
	DueAt                 string  `json:"dueAt"`
	ClaimedAt             *string `json:"claimedAt,omitempty"`
	ConfirmationDueAt     *string `json:"confirmationDueAt,omitempty"`
	ConfirmedAt           *string `json:"confirmedAt,omitempty"`
	ContestedAt           *string `json:"contestedAt,omitempty"`
	ConfirmationExpiredAt *string `json:"confirmationExpiredAt,omitempty"`
	LatenessStatus        string  `json:"latenessStatus"`
	LateAt                *string `json:"lateAt,omitempty"`
	LatenessDecidedAt     *string `json:"latenessDecidedAt,omitempty"`
	LatenessReason        string  `json:"latenessReason,omitempty"`
	ClaimedLate           bool    `json:"claimedLate"`
	Source                string  `json:"source"`
	SettlementProposalID  string  `json:"settlementProposalId,omitempty"`
	ClaimNote             string  `json:"claimNote,omitempty"`
	ResponseNote          string  `json:"responseNote,omitempty"`
	CreatedAt             string  `json:"createdAt"`
	UpdatedAt             string  `json:"updatedAt"`
	Version               int64   `json:"version"`
}

type appealResponse struct {
	ID                string                      `json:"id"`
	AppellantUserID   string                      `json:"appellantUserId,omitempty"`
	AppellantUsername string                      `json:"appellantUsername"`
	AppellantName     string                      `json:"appellantName"`
	ReportID          string                      `json:"reportId,omitempty"`
	DisputeID         string                      `json:"disputeId,omitempty"`
	TargetType        string                      `json:"targetType"`
	TargetID          string                      `json:"targetId"`
	Title             string                      `json:"title"`
	Statement         string                      `json:"statement,omitempty"`
	Status            string                      `json:"status"`
	AdminReason       string                      `json:"adminReason,omitempty"`
	HandledByAdminID  string                      `json:"handledByAdminId,omitempty"`
	HandledAt         *string                     `json:"handledAt,omitempty"`
	CreatedAt         string                      `json:"createdAt"`
	UpdatedAt         string                      `json:"updatedAt"`
	Version           int64                       `json:"version"`
	Evidence          []evidenceReferenceResponse `json:"evidence,omitempty"`
}

type evidenceReferenceResponse struct {
	ID          string `json:"id"`
	Version     int64  `json:"version"`
	Kind        string `json:"kind"`
	MIME        string `json:"mime"`
	ByteSize    int64  `json:"byteSize"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	CreatedAt   string `json:"createdAt"`
	ContentPath string `json:"contentPath"`
	Visibility  string `json:"visibility"`
	Usage       string `json:"usage"`
	SourceType  string `json:"sourceType"`
	SourceID    string `json:"sourceId"`
}

type publicDisputeResponse struct {
	ID         string `json:"id"`
	Username   string `json:"username"`
	Type       string `json:"type"`
	Result     string `json:"result"`
	HandledAt  string `json:"handledAt"`
	Unresolved bool   `json:"unresolved"`
}

type adminMutationResponse struct {
	Report  *reportResponse  `json:"report,omitempty"`
	Dispute *disputeResponse `json:"dispute,omitempty"`
	Appeal  *appealResponse  `json:"appeal,omitempty"`
}

func (s *Server) handleCreateReport(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	body, req, appErr := decodeStrictJSON[createReportRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	completion, appErr := s.app.CreateReportWithIdempotency(
		r.Context(),
		user,
		"POST /api/v1/reports",
		r.Header.Get("Idempotency-Key"),
		requestHash(r.Method, "POST /api/v1/reports", body),
		report.CreateReportInput{
			TargetType:       req.TargetType,
			TargetID:         req.TargetID,
			TargetLabel:      req.TargetLabel,
			ReportedUsername: req.ReportedUsername,
			ReasonCode:       req.ReasonCode,
			Title:            req.Title,
			Description:      req.Description,
		},
		reportCompletionBuilder(http.StatusCreated, false),
	)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeIdempotencyCompletion(w, completion)
}

func (s *Server) handleMyReports(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	items, appErr := s.app.MyReports(r.Context(), user)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writePaginatedJSON(w, r, toMyReportResponses(items, user.ID))
}

func (s *Server) handleSubmitReportSupplement(w http.ResponseWriter, r *http.Request) {
	s.handleSubmitInfoSupplement(w, r, report.InfoRequestEntityReport)
}

func (s *Server) handleSubmitDisputeSupplement(w http.ResponseWriter, r *http.Request) {
	s.handleSubmitInfoSupplement(w, r, report.InfoRequestEntityDispute)
}

func (s *Server) handleSubmitInfoSupplement(w http.ResponseWriter, r *http.Request, entityType string) {
	if entityType == report.InfoRequestEntityReport {
		s.handleSubmitReportInfoSupplement(w, r)
		return
	}
	actor, appErr := s.requireBusinessActor(r, true, true)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	body, req, appErr := decodeStrictJSON[infoSupplementRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	entityID := chi.URLParam(r, "id")
	routeKey := "POST /api/v1/me/" + entityType + "s/{id}/supplements:" + entityID
	completion, appErr := s.disputeContinuity.SubmitInfoSupplementForActorWithIdempotency(r.Context(), actor, routeKey, r.Header.Get("Idempotency-Key"), requestHash(r.Method, routeKey, body), report.SupplementInput{
		EntityType: entityType, EntityID: entityID, InfoRequestID: req.OpenInfoRequestID, Body: req.Body, RequestID: requestIDFrom(r), EvidenceAssetIDs: append([]string(nil), req.EvidenceAssetIDs...),
	}, supplementCompletionBuilder(actor.UserID))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeIdempotencyCompletion(w, completion)
}

func (s *Server) handleSubmitReportInfoSupplement(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	body, req, appErr := decodeStrictJSON[infoSupplementRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	entityID := chi.URLParam(r, "id")
	routeKey := "POST /api/v1/me/reports/{id}/supplements:" + entityID
	completion, appErr := s.app.SubmitInfoSupplementWithIdempotency(r.Context(), user, routeKey, r.Header.Get("Idempotency-Key"), requestHash(r.Method, routeKey, body), report.SupplementInput{
		EntityType: report.InfoRequestEntityReport, EntityID: entityID, InfoRequestID: req.OpenInfoRequestID, Body: req.Body, RequestID: requestIDFrom(r),
	}, supplementCompletionBuilder(user.ID))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeIdempotencyCompletion(w, completion)
}

func (s *Server) handleCreateAppeal(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	body, req, appErr := decodeStrictJSON[createAppealRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	routeKey := "POST /api/v1/me/appeals"
	completion, appErr := s.app.CreateAppealWithIdempotency(
		r.Context(),
		user,
		routeKey,
		r.Header.Get("Idempotency-Key"),
		requestHash(r.Method, routeKey, body),
		report.CreateAppealInput{
			ReportID:         req.ReportID,
			DisputeID:        req.DisputeID,
			Title:            req.Title,
			Statement:        req.Statement,
			EvidenceAssetIDs: append([]string(nil), req.EvidenceAssetIDs...),
		},
		appealCompletionBuilder(http.StatusCreated, false),
	)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeIdempotencyCompletion(w, completion)
}

func (s *Server) handleMyDisputes(w http.ResponseWriter, r *http.Request) {
	actor, appErr := s.requireBusinessActor(r, true, false)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	items, appErr := s.disputeContinuity.DisputesForActor(r.Context(), actor)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writePaginatedJSON(w, r, toMyDisputeResponses(items, actor.UserID))
}

func (s *Server) handleMyDispute(w http.ResponseWriter, r *http.Request) {
	actor, appErr := s.requireBusinessActor(r, true, false)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	item, appErr := s.disputeContinuity.DisputeForActor(r.Context(), actor, chi.URLParam(r, "id"))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	setETag(w, item.Version)
	w.Header().Set("Cache-Control", "private, no-store")
	writeJSON(w, http.StatusOK, toMyDisputeDetailResponse(item, actor.UserID))
}

func (s *Server) handleAppendDisputeMessage(w http.ResponseWriter, r *http.Request) {
	actor, appErr := s.requireBusinessActor(r, true, true)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	body, req, appErr := decodeStrictJSON[disputeMessageRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	s.handleDisputeParticipantAction(w, r, actor, body, report.DisputeParticipantActionInput{
		Action:           report.DisputeMessageActionAppend,
		Body:             req.Body,
		EvidenceAssetIDs: append([]string(nil), req.EvidenceAssetIDs...),
	})
}

func (s *Server) handleCreateDisputeSettlementProposal(w http.ResponseWriter, r *http.Request) {
	actor, appErr := s.requireBusinessActor(r, true, true)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	body, req, appErr := decodeStrictJSON[disputeSettlementProposalRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	var dueAt time.Time
	if req.FulfillmentRequired {
		dueAt, appErr = parseRequiredTime(req.DueAt, "dueAt")
		if appErr != nil {
			writeProblem(w, r, appErr)
			return
		}
	}
	s.handleDisputeParticipantAction(w, r, actor, body, report.DisputeParticipantActionInput{
		Action:              report.DisputeMessageActionPropose,
		Resolution:          req.Resolution,
		AmountCNY:           req.AmountCNY,
		Terms:               req.Terms,
		FulfillmentRequired: req.FulfillmentRequired,
		ResponsibleUserID:   req.ResponsibleUserID,
		BeneficiaryUserID:   req.BeneficiaryUserID,
		DueAt:               dueAt,
	})
}

func (s *Server) handleConfirmDisputeSettlementProposal(w http.ResponseWriter, r *http.Request) {
	actor, appErr := s.requireBusinessActor(r, true, true)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	body, _, appErr := decodeStrictJSON[emptyRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	s.handleDisputeParticipantAction(w, r, actor, body, report.DisputeParticipantActionInput{
		Action:     report.DisputeMessageActionConfirm,
		ProposalID: chi.URLParam(r, "proposalId"),
	})
}

func (s *Server) handleRejectDisputeSettlementProposal(w http.ResponseWriter, r *http.Request) {
	actor, appErr := s.requireBusinessActor(r, true, true)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	body, req, appErr := decodeStrictJSON[disputeParticipantReasonRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	s.handleDisputeParticipantAction(w, r, actor, body, report.DisputeParticipantActionInput{
		Action:     report.DisputeMessageActionReject,
		ProposalID: chi.URLParam(r, "proposalId"),
		Reason:     req.Reason,
	})
}

func (s *Server) handleEscalateDispute(w http.ResponseWriter, r *http.Request) {
	actor, appErr := s.requireBusinessActor(r, true, true)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	body, req, appErr := decodeStrictJSON[disputeEscalationRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	s.handleDisputeParticipantAction(w, r, actor, body, report.DisputeParticipantActionInput{
		Action:                    report.DisputeMessageActionEscalate,
		NegotiationChannels:       append([]string(nil), req.NegotiationChannels...),
		NegotiationEndedConfirmed: req.NegotiationEndedConfirmed,
		NegotiationSummary:        req.NegotiationSummary,
		RequestedPlatformAction:   req.RequestedPlatformAction,
		EvidenceAssetIDs:          append([]string(nil), req.EvidenceAssetIDs...),
	})
}

func (s *Server) handleClaimDisputeRemedy(w http.ResponseWriter, r *http.Request) {
	actor, appErr := s.requireBusinessActor(r, true, true)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	body, req, appErr := decodeStrictJSON[disputeRemedyClaimRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	s.handleDisputeParticipantAction(w, r, actor, body, report.DisputeParticipantActionInput{
		Action:           report.DisputeRemedyActionClaim,
		Note:             req.Note,
		EvidenceAssetIDs: append([]string(nil), req.EvidenceAssetIDs...),
	})
}

func (s *Server) handleConfirmDisputeRemedy(w http.ResponseWriter, r *http.Request) {
	actor, appErr := s.requireBusinessActor(r, true, true)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	body, req, appErr := decodeStrictJSON[disputeParticipantReasonRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	s.handleDisputeParticipantAction(w, r, actor, body, report.DisputeParticipantActionInput{
		Action: report.DisputeRemedyActionConfirm,
		Reason: req.Reason,
	})
}

func (s *Server) handleContestDisputeRemedy(w http.ResponseWriter, r *http.Request) {
	actor, appErr := s.requireBusinessActor(r, true, true)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	body, req, appErr := decodeStrictJSON[disputeParticipantReasonRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	s.handleDisputeParticipantAction(w, r, actor, body, report.DisputeParticipantActionInput{
		Action:           report.DisputeRemedyActionContest,
		Reason:           req.Reason,
		EvidenceAssetIDs: append([]string(nil), req.EvidenceAssetIDs...),
	})
}

func (s *Server) handleDisputeParticipantAction(w http.ResponseWriter, r *http.Request, actor auth.BusinessActor, body []byte, input report.DisputeParticipantActionInput) {
	input.DisputeID = chi.URLParam(r, "id")
	input.RequestID = requestIDFrom(r)
	routeKey := r.Method + " /api/v1/me/disputes/{id}/" + input.Action
	completion, appErr := s.disputeContinuity.DisputeParticipantActionForActorWithIdempotency(
		r.Context(), actor, routeKey, r.Header.Get("Idempotency-Key"),
		requestHash(r.Method, routeKey+":"+input.DisputeID+":"+input.ProposalID, body),
		input, disputeParticipantCompletionBuilder(actor.UserID),
	)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeNoStoreIdempotencyCompletion(w, completion)
}

func (s *Server) handleMyAppeals(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	items, appErr := s.app.MyAppeals(r.Context(), user)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writePaginatedJSON(w, r, toAppealResponses(items, false))
}

func (s *Server) handlePublicUserDisputes(w http.ResponseWriter, r *http.Request) {
	items, appErr := s.app.PublicUserDisputes(r.Context(), chi.URLParam(r, "username"))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, listResponse[publicDisputeResponse]{Items: toPublicDisputeResponses(items)})
}

func (s *Server) handleAdminReports(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	pageRequest, appErr := parsePageRequest(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	items, appErr := s.app.AdminReports(r.Context(), user, pageRequest)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writePageJSON(w, domain.Page[reportResponse]{
		Items:      toReportResponses(items.Items, true),
		NextCursor: items.NextCursor,
	})
}

func (s *Server) handleAdminReport(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	item, appErr := s.app.AdminReport(r.Context(), user, chi.URLParam(r, "id"))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	setETag(w, item.Version)
	writeJSON(w, http.StatusOK, toReportResponse(item, true))
}

func (s *Server) handleTriageReport(w http.ResponseWriter, r *http.Request) {
	s.handleAdminReportAction(w, r, "triage")
}

func (s *Server) handleRequestReportInfo(w http.ResponseWriter, r *http.Request) {
	s.handleAdminReportAction(w, r, "request_info")
}

func (s *Server) handleRejectReport(w http.ResponseWriter, r *http.Request) {
	s.handleAdminReportAction(w, r, "reject")
}

func (s *Server) handleOpenReportDispute(w http.ResponseWriter, r *http.Request) {
	s.handleAdminReportAction(w, r, "open_dispute")
}

func (s *Server) handleCloseReport(w http.ResponseWriter, r *http.Request) {
	s.handleAdminReportAction(w, r, "close")
}

func (s *Server) handleAdminReportAction(w http.ResponseWriter, r *http.Request, action string) {
	user, _, appErr := s.requireSessionAndCSRF(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	body, req, appErr := decodeStrictJSON[reportActionRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	version, appErr := requireIfMatchVersion(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	id := chi.URLParam(r, "id")
	routeKey := "POST /api/v1/admin/reports/{id}/" + action + ":" + id
	completion, appErr := s.app.AdminReportActionWithIdempotency(r.Context(), user, routeKey, r.Header.Get("Idempotency-Key"), requestHash(r.Method, routeKey, body), report.AdminActionInput{
		ID:               id,
		Action:           action,
		Reason:           req.Reason,
		PublicSummary:    req.PublicSummary,
		PublicResultCode: req.PublicResultCode,
		PublicResult:     req.PublicResult,
		ExpectedVersion:  version,
		RequestID:        requestIDFrom(r),
		RequestedFromID:  req.RequestedFromUserID,
	}, adminMutationCompletionBuilder)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeIdempotencyCompletion(w, completion)
}

func (s *Server) handleAdminDisputes(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	items, appErr := s.app.AdminDisputes(r.Context(), user)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writePaginatedJSON(w, r, toDisputeResponses(items, true))
}

func (s *Server) handleAdminDispute(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	item, appErr := s.app.AdminDispute(r.Context(), user, chi.URLParam(r, "id"))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	setETag(w, item.Version)
	writeJSON(w, http.StatusOK, toDisputeResponse(item, true))
}

func (s *Server) handleRequestDisputeInfo(w http.ResponseWriter, r *http.Request) {
	s.handleAdminDisputeAction(w, r, "request_info")
}

func (s *Server) handleResolveDispute(w http.ResponseWriter, r *http.Request) {
	s.handleAdminDisputeAction(w, r, "resolve")
}

func (s *Server) handleCloseDispute(w http.ResponseWriter, r *http.Request) {
	s.handleAdminDisputeAction(w, r, "close")
}

func (s *Server) handleConfirmDisputeRemedyLateness(w http.ResponseWriter, r *http.Request) {
	s.handleAdminDisputeAction(w, r, "confirm_lateness")
}

func (s *Server) handleExcuseDisputeRemedyLateness(w http.ResponseWriter, r *http.Request) {
	s.handleAdminDisputeAction(w, r, "excuse_lateness")
}

func (s *Server) handleAdminDisputeAction(w http.ResponseWriter, r *http.Request, action string) {
	user, _, appErr := s.requireSessionAndCSRF(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	body, req, appErr := decodeStrictJSON[reportActionRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	var remedyInput *report.DisputeRemedyInput
	if req.Remedy != nil {
		dueAt, dueErr := parseRequiredTime(req.Remedy.DueAt, "remedy.dueAt")
		if dueErr != nil {
			writeProblem(w, r, dueErr)
			return
		}
		remedyInput = &report.DisputeRemedyInput{
			Action: req.Remedy.Action, AmountCNY: req.Remedy.AmountCNY,
			ResponsibleUserID: req.Remedy.ResponsibleUserID,
			Instructions:      req.Remedy.Instructions, DueAt: dueAt,
		}
	}
	version, appErr := requireIfMatchVersion(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	id := chi.URLParam(r, "id")
	routeKey := "POST /api/v1/admin/disputes/{id}/" + action + ":" + id
	completion, appErr := s.app.AdminDisputeActionWithIdempotency(r.Context(), user, routeKey, r.Header.Get("Idempotency-Key"), requestHash(r.Method, routeKey, body), report.AdminActionInput{
		ID:               id,
		Action:           action,
		Reason:           req.Reason,
		PublicSummary:    req.PublicSummary,
		PublicResultCode: req.PublicResultCode,
		PublicResult:     req.PublicResult,
		ExpectedVersion:  version,
		RequestID:        requestIDFrom(r),
		RequestedFromID:  req.RequestedFromUserID,
		Remedy:           remedyInput,
	}, adminMutationCompletionBuilder)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeIdempotencyCompletion(w, completion)
}

func (s *Server) handleAdminAppeals(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	items, appErr := s.app.AdminAppeals(r.Context(), user)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writePaginatedJSON(w, r, toAppealResponses(items, true))
}

func (s *Server) handleAdminAppeal(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	item, appErr := s.app.AdminAppeal(r.Context(), user, chi.URLParam(r, "id"))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	setETag(w, item.Version)
	writeJSON(w, http.StatusOK, toAppealResponse(item, true))
}

func (s *Server) handleApproveAppeal(w http.ResponseWriter, r *http.Request) {
	s.handleAdminAppealAction(w, r, "approve")
}

func (s *Server) handleRejectAppeal(w http.ResponseWriter, r *http.Request) {
	s.handleAdminAppealAction(w, r, "reject")
}

func (s *Server) handleAdminAppealAction(w http.ResponseWriter, r *http.Request, action string) {
	user, _, appErr := s.requireSessionAndCSRF(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	body, req, appErr := decodeStrictJSON[reportActionRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	version, appErr := requireIfMatchVersion(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	id := chi.URLParam(r, "id")
	routeKey := "POST /api/v1/admin/appeals/{id}/" + action + ":" + id
	completion, appErr := s.app.AdminAppealActionWithIdempotency(r.Context(), user, routeKey, r.Header.Get("Idempotency-Key"), requestHash(r.Method, routeKey, body), report.AdminActionInput{
		ID:              id,
		Action:          action,
		Reason:          req.Reason,
		ExpectedVersion: version,
		RequestID:       requestIDFrom(r),
	}, adminMutationCompletionBuilder)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeIdempotencyCompletion(w, completion)
}

func reportCompletionBuilder(status int, includeAdmin bool) report.ReportCompletionBuilder {
	return func(item report.Report) (idempotency.Completion, *domain.AppError) {
		body, err := json.Marshal(toReportResponse(item, includeAdmin))
		if err != nil {
			return idempotency.Completion{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "响应编码失败。")
		}
		return idempotency.Completion{
			Status:       status,
			ContentType:  "application/json; charset=utf-8",
			Body:         body,
			ResourceType: "report",
			ResourceID:   item.ID,
			Headers:      map[string]string{"ETag": `"` + strconv.FormatInt(item.Version, 10) + `"`},
		}, nil
	}
}

func disputeParticipantCompletionBuilder(userID string) report.DisputeParticipantCompletionBuilder {
	return func(item report.DisputeCase) (idempotency.Completion, *domain.AppError) {
		body, err := json.Marshal(toMyDisputeDetailResponse(item, userID))
		if err != nil {
			return idempotency.Completion{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "响应编码失败。")
		}
		return idempotency.Completion{
			Status:        http.StatusOK,
			ContentType:   "application/json; charset=utf-8",
			Body:          body,
			SkipBodyCache: true,
			ResourceType:  "dispute",
			ResourceID:    item.ID,
			Headers: map[string]string{
				"ETag": `"` + strconv.FormatInt(item.Version, 10) + `"`,
			},
		}, nil
	}
}

func appealCompletionBuilder(status int, includeAdmin bool) report.AppealCompletionBuilder {
	return func(item report.Appeal) (idempotency.Completion, *domain.AppError) {
		body, err := json.Marshal(toAppealResponse(item, includeAdmin))
		if err != nil {
			return idempotency.Completion{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "响应编码失败。")
		}
		return idempotency.Completion{
			Status:       status,
			ContentType:  "application/json; charset=utf-8",
			Body:         body,
			ResourceType: "appeal",
			ResourceID:   item.ID,
			Headers:      map[string]string{"ETag": `"` + strconv.FormatInt(item.Version, 10) + `"`},
		}, nil
	}
}

func adminMutationCompletionBuilder(result report.MutationResult) (idempotency.Completion, *domain.AppError) {
	payload := toAdminMutationResponse(result)
	body, err := json.Marshal(payload)
	if err != nil {
		return idempotency.Completion{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "响应编码失败。")
	}
	headers := map[string]string{}
	resourceType := "moderation_action"
	resourceID := ""
	if result.Report != nil {
		resourceType = "report"
		resourceID = result.Report.ID
		headers["ETag"] = `"` + strconv.FormatInt(result.Report.Version, 10) + `"`
	}
	if result.Dispute != nil {
		resourceType = "dispute"
		resourceID = result.Dispute.ID
		headers["ETag"] = `"` + strconv.FormatInt(result.Dispute.Version, 10) + `"`
	}
	if result.Appeal != nil {
		resourceType = "appeal"
		resourceID = result.Appeal.ID
		headers["ETag"] = `"` + strconv.FormatInt(result.Appeal.Version, 10) + `"`
	}
	return idempotency.Completion{
		Status:       http.StatusOK,
		ContentType:  "application/json; charset=utf-8",
		Body:         body,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Headers:      headers,
	}, nil
}

func supplementCompletionBuilder(userID string) report.SupplementCompletionBuilder {
	return func(result report.MutationResult) (idempotency.Completion, *domain.AppError) {
		payload := adminMutationResponse{}
		resourceType := ""
		resourceID := ""
		version := int64(0)
		if result.Report != nil {
			response := toMyReportResponse(*result.Report, userID)
			payload.Report = &response
			resourceType, resourceID, version = "report", result.Report.ID, result.Report.Version
		}
		if result.Dispute != nil {
			response := toMyDisputeResponse(*result.Dispute, userID)
			payload.Dispute = &response
			resourceType, resourceID, version = "dispute", result.Dispute.ID, result.Dispute.Version
		}
		body, err := json.Marshal(payload)
		if err != nil {
			return idempotency.Completion{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "响应序列化失败。")
		}
		return idempotency.Completion{
			Status: http.StatusOK, ContentType: "application/json; charset=utf-8", Body: body,
			ResourceType: resourceType, ResourceID: resourceID, Headers: map[string]string{"ETag": `"` + strconv.FormatInt(version, 10) + `"`},
		}, nil
	}
}

func toReportResponses(items []report.Report, includeAdmin bool) []reportResponse {
	result := make([]reportResponse, 0, len(items))
	for _, item := range items {
		result = append(result, toReportResponse(item, includeAdmin))
	}
	return result
}

func toMyReportResponses(items []report.Report, userID string) []reportResponse {
	result := make([]reportResponse, 0, len(items))
	for _, item := range items {
		result = append(result, toMyReportResponse(item, userID))
	}
	return result
}

func toMyReportResponse(item report.Report, userID string) reportResponse {
	response := toReportResponse(item, false)
	canSupplement := item.Status == report.ReportStatusNeedsInfo && item.InfoRequestedFromID == userID && item.OpenInfoRequestID != ""
	response.CanSupplement = &canSupplement
	if canSupplement {
		response.OpenInfoRequestID = item.OpenInfoRequestID
	}
	return response
}

func toReportResponse(item report.Report, includeAdmin bool) reportResponse {
	response := reportResponse{
		ID:                  item.ID,
		ReporterUsername:    item.ReporterUsername,
		ReporterName:        item.ReporterName,
		TargetType:          item.TargetType,
		TargetID:            item.TargetID,
		CanonicalTargetType: item.CanonicalTargetType,
		CanonicalTargetID:   item.CanonicalTargetID,
		TargetLabel:         item.TargetLabel,
		ReportedUsername:    item.ReportedUsername,
		ReasonCode:          item.ReasonCode,
		Title:               item.Title,
		Status:              item.Status,
		HandledAt:           formatOptionalTime(item.HandledAt),
		DisputeID:           item.DisputeID,
		CreatedAt:           item.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:           item.UpdatedAt.UTC().Format(time.RFC3339),
		Version:             item.Version,
	}
	if includeAdmin {
		response.ReporterUserID = item.ReporterUserID
		response.Description = item.Description
		response.TargetSnapshotJSON = item.TargetSnapshotJSON
		response.AdminReason = item.AdminReason
		response.HandledByAdminID = item.HandledByAdminID
		response.Supplements = toInfoSupplementResponses(item.Supplements)
	}
	return response
}

func toDisputeResponses(items []report.DisputeCase, includeAdmin bool) []disputeResponse {
	result := make([]disputeResponse, 0, len(items))
	for _, item := range items {
		result = append(result, toDisputeResponse(item, includeAdmin))
	}
	return result
}

func toMyDisputeResponses(items []report.DisputeCase, userID string) []disputeResponse {
	result := make([]disputeResponse, 0, len(items))
	for _, item := range items {
		result = append(result, toMyDisputeResponse(item, userID))
	}
	return result
}

func toMyDisputeResponse(item report.DisputeCase, userID string) disputeResponse {
	response := toDisputeResponse(item, false)
	response.ViewerUserID = userID
	response.Evidence = toEvidenceReferenceResponses(filterEvidenceForUser(item.Evidence, userID), false)
	canAppeal := report.CanAppealDispute(item, userID)
	response.CanAppeal = &canAppeal
	canSupplement := item.Status == report.DisputeStatusWaitingInfo && item.InfoRequestedFromID == userID && item.OpenInfoRequestID != ""
	response.CanSupplement = &canSupplement
	if canSupplement {
		response.OpenInfoRequestID = item.OpenInfoRequestID
	}
	return response
}

func toMyDisputeDetailResponse(item report.DisputeCase, userID string) disputeResponse {
	response := toMyDisputeResponse(item, userID)
	response.PrimaryUserID = item.PrimaryUserID
	response.CounterpartyUserID = item.CounterpartyUserID
	return response
}

func toDisputeResponse(item report.DisputeCase, includeAdmin bool) disputeResponse {
	response := disputeResponse{
		ID:                        item.ID,
		APIOrderID:                item.APIOrderID,
		Active:                    item.Active,
		ReportID:                  item.ReportID,
		TargetType:                item.TargetType,
		TargetID:                  item.TargetID,
		TargetLabel:               item.TargetLabel,
		PrimaryUsername:           item.PrimaryUsername,
		PrimaryDisplayName:        item.PrimaryDisplayName,
		CounterpartyUsername:      item.CounterpartyUsername,
		CounterpartyName:          item.CounterpartyName,
		Status:                    item.Status,
		IssueCode:                 item.IssueCode,
		RequestedResolution:       item.RequestedResolution,
		RequestedAmountCNY:        item.RequestedAmountCNY,
		IssueOccurredAt:           formatOptionalTime(item.IssueOccurredAt),
		PublicSummary:             item.PublicSummary,
		PublicResultCode:          item.PublicResultCode,
		PublicResult:              item.PublicResult,
		OpenedAt:                  item.OpenedAt.UTC().Format(time.RFC3339),
		ResolvedAt:                formatOptionalTime(item.ResolvedAt),
		ClosedAt:                  formatOptionalTime(item.ClosedAt),
		FinalReason:               item.FinalReason,
		AppealExpiresAt:           formatOptionalTime(item.AppealExpiresAt),
		AdverselyAffectedIDs:      append([]string(nil), item.AdverselyAffectedIDs...),
		NegotiationChannels:       append([]string(nil), item.NegotiationChannels...),
		NegotiationEndedConfirmed: item.NegotiationEndedConfirmed,
		NegotiationSummary:        item.NegotiationSummary,
		RequestedPlatformAction:   item.RequestedPlatformAction,
		EscalatedByUserID:         item.EscalatedByUserID,
		EscalatedAt:               formatOptionalTime(item.EscalatedAt),
		CreatedAt:                 item.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:                 item.UpdatedAt.UTC().Format(time.RFC3339),
		Version:                   item.Version,
		Messages:                  toDisputeMessageResponses(item.Messages),
		SettlementProposals:       toSettlementProposalResponses(item.SettlementProposals),
		Remedies:                  toDisputeRemedyResponses(item.Remedies),
		Evidence:                  toEvidenceReferenceResponses(item.Evidence, includeAdmin),
	}
	if includeAdmin {
		response.PrimaryUserID = item.PrimaryUserID
		response.CounterpartyUserID = item.CounterpartyUserID
		response.SubjectUserID = item.SubjectUserID
		response.SubjectUsername = item.SubjectUsername
		response.SubjectName = item.SubjectName
		response.AdminReason = item.AdminReason
		response.OpenedByAdminID = item.OpenedByAdminID
		response.Supplements = toInfoSupplementResponses(item.Supplements)
	}
	return response
}

func toDisputeMessageResponses(items []report.DisputeMessage) []disputeMessageResponse {
	if len(items) == 0 {
		return nil
	}
	result := make([]disputeMessageResponse, 0, len(items))
	for _, item := range items {
		result = append(result, disputeMessageResponse{
			ID: item.ID, SenderUserID: item.SenderUserID, Body: item.Body,
			CreatedAt: item.CreatedAt.UTC().Format(time.RFC3339),
		})
	}
	return result
}

func toSettlementProposalResponses(items []report.SettlementProposal) []settlementProposalResponse {
	if len(items) == 0 {
		return nil
	}
	result := make([]settlementProposalResponse, 0, len(items))
	for _, item := range items {
		result = append(result, settlementProposalResponse{
			ID: item.ID, ProposedByUserID: item.ProposedByUserID, Resolution: item.Resolution,
			AmountCNY: item.AmountCNY, Terms: item.Terms, FulfillmentRequired: item.FulfillmentRequired,
			ResponsibleUserID: item.ResponsibleUserID, BeneficiaryUserID: item.BeneficiaryUserID, DueAt: formatOptionalTime(item.DueAt), Status: item.Status,
			AcceptedByUserID: item.AcceptedByUserID, AcceptedAt: formatOptionalTime(item.AcceptedAt),
			RejectedByUserID: item.RejectedByUserID, RejectedAt: formatOptionalTime(item.RejectedAt),
			SupersededReason: item.SupersededReason,
			CreatedAt:        item.CreatedAt.UTC().Format(time.RFC3339), UpdatedAt: item.UpdatedAt.UTC().Format(time.RFC3339),
			Version: item.Version,
		})
	}
	return result
}

func toDisputeRemedyResponses(items []report.DisputeRemedy) []disputeRemedyResponse {
	if len(items) == 0 {
		return nil
	}
	result := make([]disputeRemedyResponse, 0, len(items))
	for _, item := range items {
		result = append(result, disputeRemedyResponse{
			ID: item.ID, Action: item.Action, AmountCNY: item.AmountCNY, Currency: item.Currency,
			ResponsibleUserID: item.ResponsibleUserID, BeneficiaryUserID: item.BeneficiaryUserID,
			Instructions: item.Instructions, Status: item.Status, DueAt: item.DueAt.UTC().Format(time.RFC3339),
			ClaimedAt: formatOptionalTime(item.ClaimedAt), ConfirmationDueAt: formatOptionalTime(item.ConfirmationDueAt),
			ConfirmedAt: formatOptionalTime(item.ConfirmedAt), ContestedAt: formatOptionalTime(item.ContestedAt),
			ConfirmationExpiredAt: formatOptionalTime(item.ConfirmationExpiredAt),
			LatenessStatus:        item.LatenessStatus, LateAt: formatOptionalTime(item.LateAt),
			LatenessDecidedAt: formatOptionalTime(item.LatenessDecidedAt), LatenessReason: item.LatenessReason,
			ClaimedLate: item.ClaimedLate, Source: item.Source, SettlementProposalID: item.SettlementProposalID,
			ClaimNote: item.ClaimNote, ResponseNote: item.ResponseNote,
			CreatedAt: item.CreatedAt.UTC().Format(time.RFC3339), UpdatedAt: item.UpdatedAt.UTC().Format(time.RFC3339), Version: item.Version,
		})
	}
	return result
}

func toInfoSupplementResponses(items []report.InfoSupplement) []infoSupplementResponse {
	if len(items) == 0 {
		return nil
	}
	result := make([]infoSupplementResponse, 0, len(items))
	for _, item := range items {
		result = append(result, infoSupplementResponse{
			ID:                  item.ID,
			InfoRequestID:       item.InfoRequestID,
			SubmittedByUserID:   item.SubmittedByUserID,
			SubmittedByUsername: item.SubmittedByUsername,
			SubmittedByName:     item.SubmittedByName,
			Body:                item.Body,
			CreatedAt:           item.CreatedAt.UTC().Format(time.RFC3339),
		})
	}
	return result
}

func toAppealResponses(items []report.Appeal, includeAdmin bool) []appealResponse {
	result := make([]appealResponse, 0, len(items))
	for _, item := range items {
		result = append(result, toAppealResponse(item, includeAdmin))
	}
	return result
}

func toAppealResponse(item report.Appeal, includeAdmin bool) appealResponse {
	response := appealResponse{
		ID:                item.ID,
		AppellantUsername: item.AppellantUsername,
		AppellantName:     item.AppellantName,
		ReportID:          item.ReportID,
		DisputeID:         item.DisputeID,
		TargetType:        item.TargetType,
		TargetID:          item.TargetID,
		Title:             item.Title,
		Status:            item.Status,
		HandledAt:         formatOptionalTime(item.HandledAt),
		CreatedAt:         item.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:         item.UpdatedAt.UTC().Format(time.RFC3339),
		Version:           item.Version,
		Evidence:          toEvidenceReferenceResponses(filterEvidenceForUser(item.Evidence, item.AppellantUserID), false),
	}
	if includeAdmin {
		response.AppellantUserID = item.AppellantUserID
		response.Statement = item.Statement
		response.AdminReason = item.AdminReason
		response.HandledByAdminID = item.HandledByAdminID
		response.Evidence = toEvidenceReferenceResponses(item.Evidence, true)
	}
	return response
}

func filterEvidenceForUser(items []evidence.Reference, userID string) []evidence.Reference {
	result := make([]evidence.Reference, 0, len(items))
	for _, item := range items {
		if item.Visibility == evidence.VisibilityParticipantsAdmin || item.UploaderUserID == userID {
			result = append(result, item)
		}
	}
	return result
}

func toEvidenceReferenceResponses(items []evidence.Reference, admin bool) []evidenceReferenceResponse {
	if len(items) == 0 {
		return nil
	}
	result := make([]evidenceReferenceResponse, 0, len(items))
	for _, item := range items {
		contentPath := item.ContentPath
		if admin {
			contentPath = "/api/v1/admin/dispute-evidence/" + item.ID + "/content"
		}
		result = append(result, evidenceReferenceResponse{
			ID: item.ID, Version: item.Version, Kind: item.Kind, MIME: item.MIME, ByteSize: item.ByteSize,
			Width: item.Width, Height: item.Height, CreatedAt: item.CreatedAt.UTC().Format(time.RFC3339),
			ContentPath: contentPath, Visibility: item.Visibility, Usage: item.Usage,
			SourceType: item.SourceType, SourceID: item.SourceID,
		})
	}
	return result
}

func toPublicDisputeResponses(items []report.PublicDispute) []publicDisputeResponse {
	result := make([]publicDisputeResponse, 0, len(items))
	for _, item := range items {
		result = append(result, publicDisputeResponse{
			ID:         item.ID,
			Username:   item.Username,
			Type:       item.Type,
			Result:     item.Result,
			HandledAt:  item.HandledAt.UTC().Format("2006-01-02"),
			Unresolved: item.Unresolved,
		})
	}
	return result
}

func toAdminMutationResponse(result report.MutationResult) adminMutationResponse {
	response := adminMutationResponse{}
	if result.Report != nil {
		item := toReportResponse(*result.Report, true)
		response.Report = &item
	}
	if result.Dispute != nil {
		item := toDisputeResponse(*result.Dispute, true)
		response.Dispute = &item
	}
	if result.Appeal != nil {
		item := toAppealResponse(*result.Appeal, true)
		response.Appeal = &item
	}
	return response
}
