package usecase

import (
	"context"
	"prakarsa-app/entity"
	"prakarsa-app/infrastructure/transport/clients/ports"
	"prakarsa-app/transport/response"
	"prakarsa-app/utils"
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

	res, err = u.collaborationRepo.ThreadCollaborationApply(ctx, threadCollabApplicationPayload)
	return
}
