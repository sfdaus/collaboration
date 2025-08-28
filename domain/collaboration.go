package domain

import (
	"context"
	"prakarsa-app/entity"
	"prakarsa-app/transport/request"
	"prakarsa-app/transport/response"
)

// // CollaborationRepository represent the collaboration repository contract
type CollaborationRepository interface {
	ThreadCollaborationApply(ctx context.Context, threadCollabApplicationPayload *entity.ThreadPartnerApplication) (res response.ThreadCollaborationApplyRes,
		initID string, err error)
	RevertThreadCollaborationApply(ctx context.Context, threadCollabApplicationPayload *entity.ThreadPartnerApplication) (err error)
	RejectThreadCollaboration(ctx context.Context, threadCollabApplicationPayload *entity.ThreadPartnerApplication,
		pendingStatus string) (applicantID string, threadTitle string, err error)
}

// CollaborationUsecase represent the collaboration usecase contract
type CollaborationUsecase interface {
	ThreadCollaborationApply(ctx context.Context, request *request.ThreadCollaborationApplyReq) (response.ThreadCollaborationApplyRes, error)
	RejectThreadCollaboration(ctx context.Context, request *request.RejectThreadCollaborationReq) error
}
