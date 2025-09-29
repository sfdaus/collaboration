package domain

import (
	"context"
	"prakarsa-app/entity"
	"prakarsa-app/transport/request"
	"prakarsa-app/transport/response"
)

// // CollaborationRepository represent the collaboration repository contract
type CollaborationRepository interface {
	ThreadCollaborationApply(ctx context.Context, threadCollabApplicationPayload *entity.ThreadPartnerApplication,
		initiatorNotificationOutbox *entity.NotificationOutboxInsert, collabNotificationOutbox *entity.NotificationOutboxInsert,
	) (res response.ThreadCollaborationApplyRes, initID string, err error)
	RevertThreadCollaborationApply(ctx context.Context, threadCollabApplicationPayload *entity.ThreadPartnerApplication) (err error)
	RejectThreadCollaboration(ctx context.Context, threadCollabApplicationPayload *entity.ThreadPartnerApplication,
		applicantNotificationOutbox *entity.NotificationOutboxInsert, pendingStatus string) (err error)
	RevertThreadCollaborationReject(ctx context.Context, threadCollabApplicationPayload *entity.ThreadPartnerApplication,
		pendingStatus string) (err error)
	ApproveThreadCollaboration(ctx context.Context, threadCollabApplicationPayload *entity.ThreadPartnerApplication,
		threadCollaborator *entity.ThreadCollaborator, applicantNotificationOutbox *entity.NotificationOutboxInsert,
		pendingStatus string) (err error)
	RevertThreadCollaborationApprove(ctx context.Context, threadCollabApplicationPayload *entity.ThreadPartnerApplication,
		threadCollaborator *entity.ThreadCollaborator, pendingStatus, partnerTypeID string) (err error)
	MyThreadCollaboration(ctx context.Context, request *request.MyThreadCollaborationReq) ([]response.MyThreadCollaborationRes, response.MetaRes, error)
	MyThreadCollaborationRequests(ctx context.Context, request *request.MyThreadCollaborationRequestsReq) ([]response.MyThreadCollaborationRequestsRes, response.MetaRes, error)
	AcceptedThreadCollaborationRequests(ctx context.Context, request *request.AcceptedThreadCollaborationRequestsReq) ([]response.AcceptedThreadCollaborationRequestsRes, response.MetaRes, error)
	CancelThreadCollaboration(ctx context.Context, request *request.CancelThreadCollaborationReq, threadCollabApplicationPayload *entity.ThreadPartnerApplication,
		threadCollaboratorPayload *entity.ThreadCollaborator) error
}

// CollaborationUsecase represent the collaboration usecase contract
type CollaborationUsecase interface {
	ThreadCollaborationApply(ctx context.Context, request *request.ThreadCollaborationApplyReq) (response.ThreadCollaborationApplyRes, error)
	RejectThreadCollaboration(ctx context.Context, request *request.RejectThreadCollaborationReq) error
	ApproveThreadCollaboration(ctx context.Context, request *request.ApproveThreadCollaborationReq) error
	MyThreadCollaboration(ctx context.Context, request *request.MyThreadCollaborationReq) ([]response.MyThreadCollaborationRes, response.MetaRes, error)
	MyThreadCollaborationRequests(ctx context.Context, request *request.MyThreadCollaborationRequestsReq) ([]response.MyThreadCollaborationRequestsRes, response.MetaRes, error)
	AcceptedThreadCollaborationRequests(ctx context.Context, request *request.AcceptedThreadCollaborationRequestsReq) ([]response.AcceptedThreadCollaborationRequestsRes, response.MetaRes, error)
	CancelThreadCollaboration(ctx context.Context, request *request.CancelThreadCollaborationReq) error
}
