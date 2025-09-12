package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"prakarsa-app/entity"
	"prakarsa-app/infrastructure/transport/clients/ports"
	"prakarsa-app/transport/response"
	"prakarsa-app/utils"
	"strings"
	"time"

	"github.com/google/uuid"

	"prakarsa-app/domain"
	"prakarsa-app/repository/redis"
	"prakarsa-app/transport/request"
)

type CollaborationUsecase struct {
	collaborationRepo domain.CollaborationRepository
	redisRepo         redis.RedisRepository
	ctxTimeout        time.Duration
	notifClient       ports.Notification
}

// NewCollaborationUsecase will create new an notificationUsecase object representation of ThreadUsecase interface
func NewCollaborationUsecase(collaborationRepo domain.CollaborationRepository, redisRepo redis.RedisRepository,
	ctxTimeout time.Duration, notifClient ports.Notification) *CollaborationUsecase {
	return &CollaborationUsecase{
		collaborationRepo: collaborationRepo,
		redisRepo:         redisRepo,
		ctxTimeout:        ctxTimeout,
		notifClient:       notifClient,
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

	appTitle := strings.Replace(utils.CollaborationInitiatorNotificationTitle["THREAD_APPLICATION_TITLE"], "<role>", res.RoleName, 1)
	appTitle = strings.Replace(appTitle, "<thread_title>", `"`+res.ThreadName, 1)
	initiatorNotificationOutboxPayload = &entity.NotificationOutboxInsert{
		ID:            uuid.NewString(),
		Type:          utils.THREAD_INITIATOR_APPLICATION_NOTIFICATION_TYPE,
		ReferenceType: utils.THREAD_APPLICATION_NOTIFICATION_REFERENCE_TYPE,
		ReferenceID:   threadCollabApplicationPayload.ID,
		HeadersJSON:   headersJSON,
		Title:         appTitle,
		Message:       request.Message,
		Priority:      utils.NotificationPriority[utils.CollaborationNotificationPriority[utils.THREAD_INITIATOR_APPLICATION_NOTIFICATION_TYPE]],
		IdempotencyKey: fmt.Sprintf(
			"notif:thread_apply:init:%s:%s", threadCollabApplicationPayload.ID, "[INIT_ID]",
		),
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
	}

	selfTitle := strings.Replace(utils.CollaborationSelfNotificationTitle["THREAD_APPLICATION_TITLE"], "<role>", res.RoleName, 1)
	selfTitle = strings.Replace(selfTitle, "<thread_title>", res.ThreadName, 1)
	collabNotificationOutboxPayload = &entity.NotificationOutboxInsert{
		ID:            uuid.NewString(),
		UserID:        request.UserID,
		Type:          utils.THREAD_INITIATOR_APPLICATION_NOTIFICATION_TYPE,
		ReferenceType: utils.THREAD_APPLICATION_NOTIFICATION_REFERENCE_TYPE,
		ReferenceID:   threadCollabApplicationPayload.ID,
		Title:         appTitle,
		Message:       request.Message,
		HeadersJSON:   headersJSON,
		Priority:      utils.NotificationPriority[utils.CollaborationNotificationPriority[utils.THREAD_SELF_APPLICATION_NOTIFICATION_TYPE]],
		IdempotencyKey: fmt.Sprintf(
			"notif:thread_apply:collab:%s:%s", threadCollabApplicationPayload.ID, request.UserID,
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

	///*
	//	Send Notifications
	//*/
	//// Initiator Notification
	//appTitle := strings.Replace(utils.CollaborationInitiatorNotificationTitle["THREAD_APPLICATION_TITLE"], "<role>", res.RoleName, 1)
	//appTitle = strings.Replace(appTitle, "<thread_title>", `"`+res.ThreadName, 1)
	//
	//initNotifPayload := ports.CreateNotification{
	//	UserID:        initID,
	//	Type:          utils.THREAD_INITIATOR_APPLICATION_NOTIFICATION_TYPE,
	//	ReferenceType: utils.THREAD_APPLICATION_NOTIFICATION_REFERENCE_TYPE,
	//	ReferenceID:   threadCollabApplicationPayload.ID,
	//	Title:         appTitle,
	//	Message:       request.Message,
	//	Priority:      utils.CollaborationNotificationPriority[utils.THREAD_INITIATOR_APPLICATION_NOTIFICATION_TYPE],
	//	Headers:       map[string]string{"x-user-id": request.UserID},
	//}
	//err = u.notifClient.SendNotification(c, initNotifPayload)
	//
	//if err != nil {
	//	err = u.collaborationRepo.RevertThreadCollaborationApply(ctx, threadCollabApplicationPayload)
	//	if err != nil {
	//		return
	//	}
	//
	//	return res, utils.NewInternalServerError(fmt.Errorf("there was an error sending notification to author"))
	//}
	//
	//// Self Notification
	//selfTitle := strings.Replace(utils.CollaborationSelfNotificationTitle["THREAD_APPLICATION_TITLE"], "<role>", res.RoleName, 1)
	//selfTitle = strings.Replace(selfTitle, "<thread_title>", res.ThreadName, 1)
	//
	//selfNotifPayload := ports.CreateNotification{
	//	UserID:        request.UserID,
	//	Type:          utils.THREAD_SELF_APPLICATION_NOTIFICATION_TYPE,
	//	ReferenceType: utils.THREAD_APPLICATION_NOTIFICATION_REFERENCE_TYPE,
	//	ReferenceID:   threadCollabApplicationPayload.ID,
	//	Title:         selfTitle,
	//	Message:       "Aplikasimu berhasil dikirimkan ke author.",
	//	Priority:      utils.CollaborationNotificationPriority[utils.THREAD_SELF_APPLICATION_NOTIFICATION_TYPE],
	//	Headers:       map[string]string{"x-user-id": request.UserID},
	//}
	//err = u.notifClient.SendNotification(c, selfNotifPayload)

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

	applicantID, threadTitle, err := u.collaborationRepo.RejectThreadCollaboration(ctx, threadCollabApplicationPayload,
		utils.PENDING_APPLICATION_STATUS)
	if err != nil {
		return
	}

	/*
		Send Notifications
	*/

	// Applicant notification
	newTitle := strings.Replace(utils.CollaborationInitiatorNotificationTitle["APPLICATION_REJECTED_TITLE"],
		"<thread_title>", threadTitle, 1)

	initNotifPayload := ports.CreateNotification{
		UserID:        applicantID,
		Type:          utils.THREAD_APPLICATION_REJECT_NOTIFICATION,
		ReferenceType: utils.THREAD_APPLICATION_NOTIFICATION_REFERENCE_TYPE,
		ReferenceID:   threadCollabApplicationPayload.ID,
		Title:         newTitle,
		Message:       request.Message,
		Priority:      utils.CollaborationNotificationPriority[utils.THREAD_APPLICATION_REJECT_NOTIFICATION],
		Headers:       map[string]string{"x-user-id": request.UserID},
	}
	err = u.notifClient.SendNotification(c, initNotifPayload)

	if err != nil {
		err = u.collaborationRepo.RevertThreadCollaborationReject(ctx, threadCollabApplicationPayload,
			utils.PENDING_APPLICATION_STATUS)
		if err != nil {
			return
		}

		return utils.NewInternalServerError(fmt.Errorf("there was an error sending notification to applicant"))
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

	applicantID, threadTitle, partnerTypeID, err := u.collaborationRepo.ApproveThreadCollaboration(ctx, threadCollabApplicationPayload,
		threadCollaboratorPayload, utils.PENDING_APPLICATION_STATUS)
	if err != nil {
		return
	}

	/*
		Send Notifications
	*/

	// Applicant notification
	newTitle := strings.Replace(utils.CollaborationInitiatorNotificationTitle["APPLICATION_APPROVED_TITLE"],
		"<thread_title>", threadTitle, 1)

	initNotifPayload := ports.CreateNotification{
		UserID:        applicantID,
		Type:          utils.THREAD_APPLICATION_APPROVE_NOTIFICATION,
		ReferenceType: utils.THREAD_APPLICATION_NOTIFICATION_REFERENCE_TYPE,
		ReferenceID:   threadCollabApplicationPayload.ID,
		Title:         newTitle,
		Message:       utils.CollaborationInitiatorNotificationMessage["THREAD_APPLICATION_MESSAGE"],
		Priority:      utils.CollaborationNotificationPriority[utils.THREAD_APPLICATION_APPROVE_NOTIFICATION],
		Headers:       map[string]string{"x-user-id": request.UserID},
	}
	err = u.notifClient.SendNotification(c, initNotifPayload)

	if err != nil {
		err = u.collaborationRepo.RevertThreadCollaborationApprove(ctx, threadCollabApplicationPayload,
			threadCollaboratorPayload, utils.PENDING_APPLICATION_STATUS, partnerTypeID)
		if err != nil {
			return
		}

		return utils.NewInternalServerError(fmt.Errorf("there was an error sending notification to applicant"))
	}

	return
}
