package pgsql

import (
	"context"
	"database/sql"
	"errors"
	"prakarsa-app/entity"
	"prakarsa-app/transport/response"
	"prakarsa-app/utils"
	"strings"

	"github.com/lib/pq"
)

type pgsqlCollaborationRepository struct {
	db *sql.DB
}

// NewPgsqlCollaborationRepository will create new an todoRepository object representation of ReferenceRepository interface
func NewPgsqlCollaborationRepository(db *sql.DB) *pgsqlCollaborationRepository {
	return &pgsqlCollaborationRepository{
		db: db,
	}
}

func (r *pgsqlCollaborationRepository) ThreadCollaborationApply(ctx context.Context,
	threadCollabApplicationPayload *entity.ThreadPartnerApplication, initiatorNotificationOutbox *entity.NotificationOutboxInsert,
	collabNotificationOutbox *entity.NotificationOutboxInsert) (res response.ThreadCollaborationApplyRes, initID string, err error) {
	// Mulai transaction
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return
	}

	// Pastikan rollback kalau ada error
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// 1) Ambil owner thread & status aktif
	var title string
	var threadActive bool

	qThread := `SELECT t.title, t.user_id AS owner_id, t.is_active
				  FROM threads t
				  WHERE t.id = $1
				  FOR SHARE`

	if err = tx.QueryRowContext(ctx, qThread, threadCollabApplicationPayload.ThreadID).
		Scan(&title, &initID, &threadActive); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return res, initID, utils.NewNotFoundError("Thread or Role not found")
		}
		return
	}

	if !threadActive {
		return res, initID, utils.NewNotFoundError("Thread is not active")
	}

	if initID == threadCollabApplicationPayload.ApplicantUserID {
		return res, initID, utils.NewForbiddenError("Cannot apply to your own thread")
	}

	// Add initiator id
	threadCollabApplicationPayload.InitiatorUserID = initID

	// 2) Validasi role milik thread + cek kapasitas saat apply
	var needed, fulfilled *int
	var roleID, roleName string

	qRole := `SELECT tpt.amount_needed, tpt.amount_fulfilled, tpt.partner_type_id, pt.name
				  FROM thread_partner_types tpt
				  JOIN partner_types pt ON pt.id = tpt.partner_type_id
				  WHERE tpt.id = $1 AND tpt.thread_id = $2
				  FOR SHARE`

	if err = tx.QueryRowContext(ctx, qRole, threadCollabApplicationPayload.ThreadPartnerTypeID,
		threadCollabApplicationPayload.ThreadID).Scan(&needed, &fulfilled, &roleID, &roleName); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return res, initID, utils.NewNotFoundError("Role isn't available on this thread")
		}
		return
	}

	// Blokir application saat penuh
	if needed == nil {
		return res, initID, utils.NewBadRequestError("Capacity for this role is full")
	} else if fulfilled != nil && *fulfilled >= *needed {
		return res, initID, utils.NewBadRequestError("Capacity for this role is full")
	}

	// 3) Insert thread application
	qInsert := `INSERT INTO thread_partner_applications
					(id, thread_id, thread_partner_type_id, applicant_user_id, initiator_user_id,
					 message, status, is_active, created_at, created_by, updated_at)
				  VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`

	if _, err = tx.ExecContext(ctx, qInsert, threadCollabApplicationPayload.ID, threadCollabApplicationPayload.ThreadID,
		threadCollabApplicationPayload.ThreadPartnerTypeID, threadCollabApplicationPayload.ApplicantUserID, threadCollabApplicationPayload.InitiatorUserID,
		threadCollabApplicationPayload.Message, threadCollabApplicationPayload.Status, threadCollabApplicationPayload.IsActive,
		threadCollabApplicationPayload.CreatedAt, threadCollabApplicationPayload.CreatedBy, threadCollabApplicationPayload.UpdatedAt); err != nil {

		var pqe *pq.Error
		if errors.As(err, &pqe) && pqe.Constraint == "uniq_active_application_per_role" {
			return res, initID, utils.NewForbiddenError("You cannot apply to the same role twice")
		}
		return
	}

	res = response.ThreadCollaborationApplyRes{
		ID:         threadCollabApplicationPayload.ID,
		ThreadID:   threadCollabApplicationPayload.ThreadID,
		ThreadName: title,
		RoleID:     roleID,
		RoleName:   roleName,
		Status:     threadCollabApplicationPayload.Status,
	}

	// Initiator Notification
	initiatorNotificationOutbox.UserID = initID
	initiatorNotificationOutbox.IdempotencyKey = strings.Replace(initiatorNotificationOutbox.IdempotencyKey, "[INIT_ID]", initID, 1)

	appTitle := strings.Replace(utils.CollaborationInitiatorNotificationTitle["THREAD_APPLICATION_TITLE"], "<role>", res.RoleName, 1)
	appTitle = strings.Replace(appTitle, "<thread_title>", res.ThreadName, 1)
	initiatorNotificationOutbox.Title = appTitle

	qInitNotifOutbox := `
						 INSERT INTO notification_outbox
								(id, user_id, type, reference_type, reference_id, headers_json,
								 title, message, action_url, priority, status, attempt_count,
								 next_attempt_at, idempotency_key, created_at, updated_at)
							VALUES
								($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'pending',0,NOW(),$11,$12,$13)
						`
	if _, err = tx.ExecContext(ctx, qInitNotifOutbox,
		initiatorNotificationOutbox.ID, initiatorNotificationOutbox.UserID, initiatorNotificationOutbox.Type, initiatorNotificationOutbox.ReferenceType,
		initiatorNotificationOutbox.ReferenceID, initiatorNotificationOutbox.HeadersJSON, initiatorNotificationOutbox.Title, initiatorNotificationOutbox.Message,
		initiatorNotificationOutbox.ActionURL, initiatorNotificationOutbox.Priority, initiatorNotificationOutbox.IdempotencyKey, initiatorNotificationOutbox.CreatedAt,
		initiatorNotificationOutbox.UpdatedAt,
	); err != nil {
		// idempotency_key UNIQUE akan trigger duplicate error kalau kejadian enqueue ganda
		return
	}

	// Collaborator Notification
	selfTitle := strings.Replace(utils.CollaborationSelfNotificationTitle["THREAD_APPLICATION_TITLE"], "<role>", res.RoleName, 1)
	selfTitle = strings.Replace(selfTitle, "<thread_title>", res.ThreadName, 1)
	collabNotificationOutbox.Title = selfTitle

	qCollabNotifOutbox := `
						 INSERT INTO notification_outbox
								(id, user_id, type, reference_type, reference_id, headers_json,
								 title, message, action_url, priority, status, attempt_count,
								 next_attempt_at, idempotency_key, created_at, updated_at)
							VALUES
								($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'pending',0,NOW(),$11,$12,$13)
						`
	if _, err = tx.ExecContext(ctx, qCollabNotifOutbox,
		collabNotificationOutbox.ID, collabNotificationOutbox.UserID, collabNotificationOutbox.Type, collabNotificationOutbox.ReferenceType,
		collabNotificationOutbox.ReferenceID, collabNotificationOutbox.HeadersJSON, collabNotificationOutbox.Title, collabNotificationOutbox.Message,
		collabNotificationOutbox.ActionURL, collabNotificationOutbox.Priority, collabNotificationOutbox.IdempotencyKey, collabNotificationOutbox.CreatedAt,
		collabNotificationOutbox.UpdatedAt,
	); err != nil {
		// idempotency_key UNIQUE akan trigger duplicate error kalau kejadian enqueue ganda
		return
	}

	if err = tx.Commit(); err != nil {
		return
	}

	return
}

func (r *pgsqlCollaborationRepository) RevertThreadCollaborationApply(ctx context.Context,
	threadCollabApplicationPayload *entity.ThreadPartnerApplication) (err error) {
	query := "DELETE FROM thread_partner_applications WHERE id = $1"
	_, err = r.db.ExecContext(ctx, query, threadCollabApplicationPayload.ID)
	return
}

func (r *pgsqlCollaborationRepository) RejectThreadCollaboration(ctx context.Context,
	threadCollabApplicationPayload *entity.ThreadPartnerApplication, pendingStatus string) (applicantID string, threadTitle string, err error) {
	// Mulai transaction
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return
	}

	// Pastikan rollback kalau ada error
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// Ambil applicant_id
	var threadID string
	qThreadApp := `SELECT applicant_user_id, thread_id
				  FROM thread_partner_applications
				  WHERE id = $1 AND status = $2 AND initiator_user_id = $3 AND is_active = true
				  FOR SHARE`

	if err = tx.QueryRowContext(ctx, qThreadApp, threadCollabApplicationPayload.ID, pendingStatus, threadCollabApplicationPayload.InitiatorUserID).
		Scan(&applicantID, &threadID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return applicantID, threadTitle, utils.NewNotFoundError("Thread collaboration application not found")
		}
		return
	}

	// Ambil thread title
	qThread := `SELECT title
				  FROM threads
				  WHERE id = $1 AND is_active = true
				  FOR SHARE`

	if err = tx.QueryRowContext(ctx, qThread, threadID).
		Scan(&threadTitle); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return applicantID, threadTitle, utils.NewNotFoundError("Thread not found")
		}
		return
	}

	// Update rejected
	query := `UPDATE thread_partner_applications 
				SET 
				status = $1,
				reject_reason = $2,
				updated_at = $3,
				updated_by = $4
				WHERE id = $5 AND initiator_user_id = $6 AND is_active = true`
	_, err = tx.ExecContext(ctx, query, threadCollabApplicationPayload.Status, threadCollabApplicationPayload.RejectReason,
		threadCollabApplicationPayload.UpdatedAt, threadCollabApplicationPayload.UpdatedBy,
		threadCollabApplicationPayload.ID, threadCollabApplicationPayload.InitiatorUserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return applicantID, threadTitle, utils.NewNotFoundError("Application not found")
		}
		return
	}

	if err = tx.Commit(); err != nil {
		return
	}

	return
}

func (r *pgsqlCollaborationRepository) RevertThreadCollaborationReject(ctx context.Context,
	threadCollabApplicationPayload *entity.ThreadPartnerApplication, pendingStatus string) (err error) {
	query := `UPDATE thread_partner_applications 
		SET 
		status = $1,
		updated_at = $2,
		updated_by = $3
		WHERE id = $4 AND initiator_user_id = $5 AND is_active = true`
	_, err = r.db.ExecContext(ctx, query, pendingStatus,
		threadCollabApplicationPayload.UpdatedAt, threadCollabApplicationPayload.UpdatedBy,
		threadCollabApplicationPayload.ID, threadCollabApplicationPayload.InitiatorUserID)
	if err != nil {
		return
	}
	return
}

func (r *pgsqlCollaborationRepository) ApproveThreadCollaboration(ctx context.Context,
	threadCollabApplicationPayload *entity.ThreadPartnerApplication, threadCollaborator *entity.ThreadCollaborator,
	pendingStatus string) (applicantID string, threadTitle string, partnerTypeID string, err error) {

	// Mulai transaction
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return
	}

	// Pastikan rollback kalau ada error
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// Ambil applicant_id
	var threadID string
	qThreadApp := `SELECT applicant_user_id, thread_id, thread_partner_type_id
				  FROM thread_partner_applications
				  WHERE id = $1 AND status = $2 AND initiator_user_id = $3 AND is_active = true
				  FOR SHARE`

	if err = tx.QueryRowContext(ctx, qThreadApp, threadCollabApplicationPayload.ID, pendingStatus, threadCollabApplicationPayload.InitiatorUserID).
		Scan(&applicantID, &threadID, &partnerTypeID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return applicantID, threadTitle, partnerTypeID, utils.NewNotFoundError("Thread collaboration application not found")
		}
		return
	}

	// LOCK role & cek kapasitas
	var need, full int
	qRole := `SELECT amount_needed, amount_fulfilled
			FROM thread_partner_types
			WHERE id=$1 AND thread_id=$2
			FOR UPDATE`
	if err = tx.QueryRowContext(ctx, qRole, partnerTypeID, threadID).Scan(&need, &full); err != nil {
		return
	}
	if full >= need {
		err = utils.NewForbiddenError("Role capacity is full")
		return
	}

	// Add missing values on collaborator payload
	threadCollaborator.ThreadPartnerTypeID = partnerTypeID
	threadCollaborator.ThreadID = threadID
	threadCollaborator.UserID = applicantID

	// Ambil thread title
	qThread := `SELECT title
				  FROM threads
				  WHERE id = $1 AND is_active = true
				  FOR SHARE`

	if err = tx.QueryRowContext(ctx, qThread, threadID).
		Scan(&threadTitle); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return applicantID, threadTitle, partnerTypeID, utils.NewNotFoundError("Thread not found")
		}
		return
	}

	// Update application status + cek RowsAffected()
	query := `UPDATE thread_partner_applications
				SET status = $1,
					updated_at = $2,
					updated_by = $3
				WHERE id = $4 AND initiator_user_id = $5 AND status = $6 AND is_active = true`

	res, err := tx.ExecContext(ctx, query, threadCollabApplicationPayload.Status, threadCollabApplicationPayload.UpdatedAt,
		threadCollabApplicationPayload.UpdatedBy, threadCollabApplicationPayload.ID, threadCollabApplicationPayload.InitiatorUserID, pendingStatus)

	if err != nil {
		return
	}

	if n, _ := res.RowsAffected(); n == 0 {
		err = utils.NewNotFoundError("Application not found or already processed")
		return
	}

	// Insert thread collaborator
	query = `INSERT INTO thread_collaborators
			(id, thread_id, thread_partner_type_id, user_id, is_active, joined_at, status, 
				created_at, created_by, updated_at)
			VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9, $10);`
	_, err = tx.ExecContext(ctx, query, threadCollaborator.ID, threadCollaborator.ThreadID, threadCollaborator.ThreadPartnerTypeID,
		threadCollaborator.UserID, threadCollaborator.IsActive, threadCollaborator.JoinedAt, threadCollaborator.Status,
		threadCollaborator.CreatedAt, threadCollaborator.CreatedBy, threadCollaborator.UpdatedAt)
	if err != nil {
		return
	}

	// Update amount fulfilled
	qPartnerType := `UPDATE thread_partner_types
					SET amount_fulfilled = amount_fulfilled + 1
					WHERE id = $1 AND is_active = true`

	if _, err = tx.ExecContext(ctx, qPartnerType, partnerTypeID); err != nil {
		return
	}

	if err = tx.Commit(); err != nil {
		return
	}

	return
}

func (r *pgsqlCollaborationRepository) RevertThreadCollaborationApprove(ctx context.Context,
	threadCollabApplicationPayload *entity.ThreadPartnerApplication, threadCollaborator *entity.ThreadCollaborator, pendingStatus, partnerTypeID string) (err error) {
	// Mulai transaction
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return
	}

	// Pastikan rollback kalau ada error
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// Update approved
	query := `UPDATE thread_partner_applications 
				SET 
				status = $1,
				updated_at = $2
				WHERE id = $3 AND initiator_user_id = $4 AND is_active = true`
	_, err = tx.ExecContext(ctx, query, pendingStatus, threadCollabApplicationPayload.UpdatedAt,
		threadCollabApplicationPayload.ID, threadCollabApplicationPayload.InitiatorUserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return utils.NewNotFoundError("Revert : Application not found")
		}
		return
	}

	// Revert thread collaborator
	query = `DELETE FROM thread_collaborators
				WHERE id = $1`
	_, err = tx.ExecContext(ctx, query, threadCollaborator.ID)
	if err != nil {
		return
	}

	// Revert amount fulfilled
	qPartnerType := `UPDATE thread_partner_types
					SET amount_fulfilled = amount_fulfilled - 1
					WHERE id = $1 AND is_active = true`

	if _, err = tx.ExecContext(ctx, qPartnerType, partnerTypeID); err != nil {
		return
	}

	if err = tx.Commit(); err != nil {
		return
	}
	return
}
