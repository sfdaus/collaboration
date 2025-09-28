package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"prakarsa-app/config"
	"prakarsa-app/entity"
	"prakarsa-app/infrastructure/transport/clients/ports"
	"prakarsa-app/transport/response"
	"prakarsa-app/utils"
	"time"

	"github.com/google/uuid"

	"prakarsa-app/domain"
	"prakarsa-app/repository/redis"
	"prakarsa-app/repository/s3"
	"prakarsa-app/transport/request"
)

type CollaborationUsecase struct {
	collaborationRepo domain.CollaborationRepository
	redisRepo         redis.RedisRepository
	ctxTimeout        time.Duration
	notifClient       ports.Notification
	s3Repo            s3.S3Repository
}

// NewCollaborationUsecase will create new an notificationUsecase object representation of ThreadUsecase interface
func NewCollaborationUsecase(collaborationRepo domain.CollaborationRepository, redisRepo redis.RedisRepository,
	ctxTimeout time.Duration, notifClient ports.Notification, s3Repo s3.S3Repository) *CollaborationUsecase {
	return &CollaborationUsecase{
		collaborationRepo: collaborationRepo,
		redisRepo:         redisRepo,
		ctxTimeout:        ctxTimeout,
		notifClient:       notifClient,
		s3Repo:            s3Repo,
	}
}

func (u *CollaborationUsecase) ThreadCollaborationApply(c context.Context, request *request.ThreadCollaborationApplyReq) (res response.ThreadCollaborationApplyRes, err error) {
	ctx, cancel := context.WithTimeout(c, u.ctxTimeout)
	defer cancel()

	// Payload Thread Collaboration Application
	threadCollabApplicationPayload := &entity.ThreadPartnerApplication{
		ID:                  uuid.NewString(),
		ThreadID:            request.ThreadID,
		ThreadPartnerTypeID: request.PartnerTypeID,
		ApplicantUserID:     request.UserID,
		Message:             request.Message,
		Status:              utils.PENDING_APPLICATION_STATUS,
		IsActive:            true,
		CreatedAt:           time.Now().Unix(),
		CreatedBy:           request.UserID,
		UpdatedAt:           time.Now().Unix(),
	}

	/*
		Payload Notification Outbox
	*/
	var initiatorNotificationOutboxPayload, collabNotificationOutboxPayload *entity.NotificationOutboxInsert

	headers := map[string]string{"x-user-id": request.UserID}
	headersJSON, _ := json.Marshal(headers)

	initiatorNotificationOutboxPayload = &entity.NotificationOutboxInsert{
		ID:            uuid.NewString(),
		Type:          utils.THREAD_INITIATOR_APPLICATION_NOTIFICATION_TYPE,
		ReferenceType: utils.THREAD_APPLICATION_NOTIFICATION_REFERENCE_TYPE,
		ReferenceID:   threadCollabApplicationPayload.ID,
		HeadersJSON:   headersJSON,
		Message:       request.Message,
		Priority:      utils.CollaborationNotificationPriority[utils.THREAD_INITIATOR_APPLICATION_NOTIFICATION_TYPE],
		IdempotencyKey: fmt.Sprintf(
			"%s:%s:%s", utils.NotificationIdempotencyKey[utils.THREAD_INITIATOR_APPLICATION_NOTIFICATION_TYPE],
			threadCollabApplicationPayload.ID, "[INIT_ID]",
		),
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
	}

	collabNotificationOutboxPayload = &entity.NotificationOutboxInsert{
		ID:            uuid.NewString(),
		UserID:        request.UserID,
		Type:          utils.THREAD_SELF_APPLICATION_NOTIFICATION_TYPE,
		ReferenceType: utils.THREAD_APPLICATION_NOTIFICATION_REFERENCE_TYPE,
		ReferenceID:   threadCollabApplicationPayload.ID,
		Message:       "Aplikasimu berhasil dikirimkan ke author.",
		HeadersJSON:   headersJSON,
		Priority:      utils.CollaborationNotificationPriority[utils.THREAD_SELF_APPLICATION_NOTIFICATION_TYPE],
		IdempotencyKey: fmt.Sprintf(
			"%s:%s:%s", utils.NotificationIdempotencyKey[utils.THREAD_SELF_APPLICATION_NOTIFICATION_TYPE],
			threadCollabApplicationPayload.ID, request.UserID,
		),
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
	}

	//var initID string
	res, _, err = u.collaborationRepo.ThreadCollaborationApply(ctx, threadCollabApplicationPayload,
		initiatorNotificationOutboxPayload, collabNotificationOutboxPayload)
	if err != nil {
		return
	}

	return
}

func (u *CollaborationUsecase) RejectThreadCollaboration(c context.Context, request *request.RejectThreadCollaborationReq) (err error) {
	ctx, cancel := context.WithTimeout(c, u.ctxTimeout)
	defer cancel()

	threadCollabApplicationPayload := &entity.ThreadPartnerApplication{
		ID:              request.ApplicationCollaborationID,
		InitiatorUserID: request.UserID,
		RejectReason:    request.Message,
		Status:          utils.REJECTED_APPLICATION_STATUS,
		UpdatedAt:       time.Now().Unix(),
		UpdatedBy:       request.UserID,
	}

	// Payload Applicant Notification
	var applicantNotificationOutboxPayload *entity.NotificationOutboxInsert

	headers := map[string]string{"x-user-id": request.UserID}
	headersJSON, _ := json.Marshal(headers)

	applicantNotificationOutboxPayload = &entity.NotificationOutboxInsert{
		ID:            uuid.NewString(),
		Type:          utils.THREAD_APPLICATION_REJECT_NOTIFICATION,
		ReferenceType: utils.THREAD_APPLICATION_NOTIFICATION_REFERENCE_TYPE,
		ReferenceID:   threadCollabApplicationPayload.ID,
		Message:       request.Message,
		HeadersJSON:   headersJSON,
		Priority:      utils.CollaborationNotificationPriority[utils.THREAD_APPLICATION_REJECT_NOTIFICATION],
		IdempotencyKey: fmt.Sprintf(
			"%s:%s:%s", utils.NotificationIdempotencyKey[utils.THREAD_APPLICATION_REJECT_NOTIFICATION],
			threadCollabApplicationPayload.ID, "[APPLICANT_ID]",
		),
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
	}

	err = u.collaborationRepo.RejectThreadCollaboration(ctx, threadCollabApplicationPayload,
		applicantNotificationOutboxPayload, utils.PENDING_APPLICATION_STATUS)
	if err != nil {
		return
	}

	return
}

func (u *CollaborationUsecase) ApproveThreadCollaboration(c context.Context, request *request.ApproveThreadCollaborationReq) (err error) {
	ctx, cancel := context.WithTimeout(c, u.ctxTimeout)
	defer cancel()

	threadCollabApplicationPayload := &entity.ThreadPartnerApplication{
		ID:              request.ApplicationCollaborationID,
		InitiatorUserID: request.UserID,
		Status:          utils.ACCEPTED_APPLICATION_STATUS,
		UpdatedAt:       time.Now().Unix(),
		UpdatedBy:       request.UserID,
	}

	threadCollaboratorPayload := &entity.ThreadCollaborator{
		ID:        uuid.NewString(),
		Status:    utils.ACTIVE_COLLABORATION_STATUS,
		JoinedAt:  time.Now().Unix(),
		IsActive:  true,
		CreatedAt: time.Now().Unix(),
		CreatedBy: request.UserID,
		UpdatedAt: time.Now().Unix(),
	}

	// Payload Applicant Notification
	var applicantNotificationOutboxPayload *entity.NotificationOutboxInsert

	headers := map[string]string{"x-user-id": request.UserID}
	headersJSON, _ := json.Marshal(headers)

	applicantNotificationOutboxPayload = &entity.NotificationOutboxInsert{
		ID:            uuid.NewString(),
		Type:          utils.THREAD_APPLICATION_APPROVE_NOTIFICATION,
		ReferenceType: utils.THREAD_APPLICATION_NOTIFICATION_REFERENCE_TYPE,
		ReferenceID:   threadCollabApplicationPayload.ID,
		Message:       utils.CollaborationInitiatorNotificationMessage["THREAD_APPLICATION_APPROVAL_MESSAGE"],
		HeadersJSON:   headersJSON,
		Priority:      utils.CollaborationNotificationPriority[utils.THREAD_APPLICATION_APPROVE_NOTIFICATION],
		IdempotencyKey: fmt.Sprintf(
			"%s:%s:%s", utils.NotificationIdempotencyKey[utils.THREAD_APPLICATION_APPROVE_NOTIFICATION],
			threadCollabApplicationPayload.ID, "[APPLICANT_ID]",
		),
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
	}

	err = u.collaborationRepo.ApproveThreadCollaboration(ctx, threadCollabApplicationPayload,
		threadCollaboratorPayload, applicantNotificationOutboxPayload, utils.PENDING_APPLICATION_STATUS)
	if err != nil {
		return
	}

	return
}

func (u *CollaborationUsecase) MyThreadCollaboration(c context.Context, request *request.MyThreadCollaborationReq) (res []response.MyThreadCollaborationRes, meta response.MetaRes, err error) {
	ctx, cancel := context.WithTimeout(c, u.ctxTimeout)
	defer cancel()

	res, meta, err = u.collaborationRepo.MyThreadCollaboration(ctx, request)
	if err != nil {
		return
	}

	// Map response
	if len(res) > 0 {
		for i, item := range res {
			res[i].Profile.Avatar, err = u.s3Repo.GetPresignedURL(c, config.LoadConfig().S3Bucket, item.Profile.Avatar, false, time.Duration(24*time.Hour))
			if err != nil {
				return res, meta, err
			}
		}
	}

	return
}
