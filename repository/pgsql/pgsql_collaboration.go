package pgsql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"prakarsa-app/entity"
	"prakarsa-app/transport/request"
	"prakarsa-app/transport/response"
	"prakarsa-app/utils"
	"strings"
	"time"

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
	var deadline *time.Time

	qThread := `SELECT t.title, t.user_id AS owner_id, t.is_active, t.deadline
				  FROM threads t
				  WHERE t.id = $1
				  FOR SHARE`

	if err = tx.QueryRowContext(ctx, qThread, threadCollabApplicationPayload.ThreadID).
		Scan(&title, &initID, &threadActive, &deadline); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return res, initID, utils.NewNotFoundError("Thread or Role not found")
		}
		return
	}

	if deadline != nil && (!threadActive || deadline.Before(time.Now())) {
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

	// Pengecekan kalau dia sudah mendaftar pada projek atau thread tersebut
	var tAppID, tAppStatus string
	tApplication := `SELECT tpa.id, tpa.status
				  FROM thread_partner_applications tpa
				  WHERE tpa.thread_id = $1 AND tpa.applicant_user_id = $2
				  ORDER BY COALESCE(tpa.updated_at, tpa.created_at) DESC
				  LIMIT 1
				  FOR SHARE`

	if err = tx.QueryRowContext(ctx, tApplication, threadCollabApplicationPayload.ThreadID, threadCollabApplicationPayload.ApplicantUserID).
		Scan(&tAppID, &tAppStatus); err != nil {
		return
	}

	if tAppStatus == utils.PENDING_APPLICATION_STATUS || tAppStatus == utils.ACCEPTED_APPLICATION_STATUS {
		return res, initID, utils.NewBadRequestError("Your are already applying for this project or thread")
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

func (r *pgsqlCollaborationRepository) RejectThreadCollaboration(ctx context.Context, threadCollabApplicationPayload *entity.ThreadPartnerApplication,
	applicantNotificationOutbox *entity.NotificationOutboxInsert, pendingStatus string) (err error) {
	var threadTitle, applicantID string

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
			return utils.NewNotFoundError("Thread collaboration application not found")
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
			return utils.NewNotFoundError("Thread not found")
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
			return utils.NewNotFoundError("Application not found")
		}
		return
	}

	// Applicant Notification
	applicantNotificationOutbox.UserID = applicantID
	newTitle := strings.Replace(utils.CollaborationInitiatorNotificationTitle["APPLICATION_REJECTED_TITLE"],
		"<thread_title>", threadTitle, 1)
	applicantNotificationOutbox.Title = newTitle

	applicantNotificationOutbox.IdempotencyKey = strings.Replace(applicantNotificationOutbox.IdempotencyKey, "[APPLICANT_ID]", applicantID, 1)

	qCollabNotifOutbox := `
						 INSERT INTO notification_outbox
								(id, user_id, type, reference_type, reference_id, headers_json,
								 title, message, action_url, priority, status, attempt_count,
								 next_attempt_at, idempotency_key, created_at, updated_at)
							VALUES
								($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'pending',0,NOW(),$11,$12,$13)
						`
	if _, err = tx.ExecContext(ctx, qCollabNotifOutbox,
		applicantNotificationOutbox.ID, applicantNotificationOutbox.UserID, applicantNotificationOutbox.Type, applicantNotificationOutbox.ReferenceType,
		applicantNotificationOutbox.ReferenceID, applicantNotificationOutbox.HeadersJSON, applicantNotificationOutbox.Title, applicantNotificationOutbox.Message,
		applicantNotificationOutbox.ActionURL, applicantNotificationOutbox.Priority, applicantNotificationOutbox.IdempotencyKey, applicantNotificationOutbox.CreatedAt,
		applicantNotificationOutbox.UpdatedAt,
	); err != nil {
		// idempotency_key UNIQUE akan trigger duplicate error kalau kejadian enqueue ganda
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
	applicantNotificationOutbox *entity.NotificationOutboxInsert, pendingStatus string) (err error) {
	var applicantID, threadTitle, partnerTypeID string

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
			return utils.NewNotFoundError("Thread collaboration application not found")
		}
		return
	}

	// LOCK role & cek kapasitas
	var need, full int64
	qRole := `SELECT COALESCE(amount_needed,0) as amount_needed, COALESCE(amount_fulfilled,0) as amount_fulfilled
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
			return utils.NewNotFoundError("Thread not found")
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
				created_at, created_by, updated_at, thread_partner_application_id)
			VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11);`
	_, err = tx.ExecContext(ctx, query, threadCollaborator.ID, threadCollaborator.ThreadID, threadCollaborator.ThreadPartnerTypeID,
		threadCollaborator.UserID, threadCollaborator.IsActive, threadCollaborator.JoinedAt, threadCollaborator.Status,
		threadCollaborator.CreatedAt, threadCollaborator.CreatedBy, threadCollaborator.UpdatedAt, threadCollaborator.ThreadPartnerApplicationID)
	if err != nil {
		return
	}

	// Update amount fulfilled
	qPartnerType := `UPDATE thread_partner_types
					SET amount_fulfilled = COALESCE(amount_fulfilled, 0) + 1
					WHERE id = $1 AND is_active = true`

	if _, err = tx.ExecContext(ctx, qPartnerType, partnerTypeID); err != nil {
		return
	}

	// Applicant Notification
	applicantNotificationOutbox.UserID = applicantID
	newTitle := strings.Replace(utils.CollaborationInitiatorNotificationTitle["APPLICATION_APPROVED_TITLE"],
		"<thread_title>", threadTitle, 1)
	applicantNotificationOutbox.Title = newTitle

	applicantNotificationOutbox.IdempotencyKey = strings.Replace(applicantNotificationOutbox.IdempotencyKey, "[APPLICANT_ID]", applicantID, 1)

	qCollabNotifOutbox := `
						 INSERT INTO notification_outbox
								(id, user_id, type, reference_type, reference_id, headers_json,
								 title, message, action_url, priority, status, attempt_count,
								 next_attempt_at, idempotency_key, created_at, updated_at)
							VALUES
								($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'pending',0,NOW(),$11,$12,$13)
						`
	if _, err = tx.ExecContext(ctx, qCollabNotifOutbox,
		applicantNotificationOutbox.ID, applicantNotificationOutbox.UserID, applicantNotificationOutbox.Type, applicantNotificationOutbox.ReferenceType,
		applicantNotificationOutbox.ReferenceID, applicantNotificationOutbox.HeadersJSON, applicantNotificationOutbox.Title, applicantNotificationOutbox.Message,
		applicantNotificationOutbox.ActionURL, applicantNotificationOutbox.Priority, applicantNotificationOutbox.IdempotencyKey, applicantNotificationOutbox.CreatedAt,
		applicantNotificationOutbox.UpdatedAt,
	); err != nil {
		// idempotency_key UNIQUE akan trigger duplicate error kalau kejadian enqueue ganda
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

func (r *pgsqlCollaborationRepository) MyThreadCollaboration(ctx context.Context, request *request.MyThreadCollaborationReq) (res []response.MyThreadCollaborationRes, meta response.MetaRes, err error) {
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

	// --- pagination ---
	perPage := request.PerPage
	if perPage <= 0 {
		perPage = 10
	}
	page := request.Page
	if page <= 0 {
		page = 1
	}
	meta.Page, meta.PerPage = page, perPage
	offset := (page - 1) * perPage

	// --- WHERE builder (mirip template-mu) ---
	wheres := []string{}
	args := []any{}
	idx := 1

	// wajib: aplikasi yang diajukan oleh user ini
	wheres = append(wheres, fmt.Sprintf("taa.applicant_user_id = $%d", idx))
	args = append(args, request.UserID)
	idx++

	// optional: status
	if s := strings.TrimSpace(request.Status); s != "" {
		wheres = append(wheres, fmt.Sprintf("taa.status = $%d", idx))
		args = append(args, s)
		idx++
	}

	// contoh: hanya active (hapus baris ini kalau tidak pakai is_active)
	wheres = append(wheres, "COALESCE(taa.is_active, TRUE)")
	whereSQL := ""
	if len(wheres) > 0 {
		whereSQL = "WHERE " + strings.Join(wheres, " AND ")
	}

	// --- COUNT ---
	countSQL := `
        SELECT COUNT(*)
        FROM thread_partner_applications taa
        JOIN thread_partner_types tpt ON tpt.id = taa.thread_partner_type_id
        JOIN threads t               ON t.id  = tpt.thread_id
    ` + whereSQL
	if err = tx.QueryRowContext(ctx, countSQL, args...).Scan(&meta.TotalData); err != nil {
		return
	}
	meta.TotalPages = (meta.TotalData + perPage - 1) / perPage

	// --- DATA ---
	// tambahkan limit+offset ke args
	args = append(args, perPage, offset)
	limitPos, offsetPos := idx, idx+1

	dataSQL := fmt.Sprintf(`
        SELECT
            t.id                          AS thread_id,
            t.title                       AS thread_name,
            taa.id                        AS application_id,
            pt.name                       AS partner_type_name,
            p.name                        AS profile_name,
            p.name_alias                  AS profile_name_alias,
            p.avatar                      AS profile_avatar,
            taa.status                    AS status,
            COALESCE(taa.updated_at, taa.created_at) AS created_at
        FROM thread_partner_applications taa
        JOIN thread_partner_types tpt ON tpt.id      = taa.thread_partner_type_id
        JOIN partner_types pt        ON pt.id       = tpt.partner_type_id
        JOIN threads t               ON t.id        = tpt.thread_id
        JOIN profiles p              ON p.user_id   = t.user_id  -- author thread
        %s
        ORDER BY COALESCE(taa.updated_at, taa.created_at) DESC
        LIMIT $%d OFFSET $%d
    `, whereSQL, limitPos, offsetPos)

	rows, err := tx.QueryContext(ctx, dataSQL, args...)
	if err != nil {
		return
	}
	defer rows.Close()

	out := make([]response.MyThreadCollaborationRes, 0, perPage)
	for rows.Next() {
		var item response.MyThreadCollaborationRes
		var prof entity.SimpleProfile
		if err = rows.Scan(
			&item.ThreadID,
			&item.ThreadName,
			&item.ApplicationID,
			&item.PartnerTypeName,
			&prof.Name,
			&prof.NameAlias,
			&prof.Avatar,
			&item.Status,
			&item.CreatedAt,
		); err != nil {
			return
		}
		item.Profile = prof
		out = append(out, item)
	}
	if err = rows.Err(); err != nil {
		return
	}

	res = out
	err = tx.Commit()
	return
}

func (r *pgsqlCollaborationRepository) MyThreadCollaborationRequests(ctx context.Context, request *request.MyThreadCollaborationRequestsReq) (res []response.MyThreadCollaborationRequestsRes, meta response.MetaRes, err error) {

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	// --- pagination ---
	perPage := request.PerPage
	if perPage <= 0 {
		perPage = 10
	}
	page := request.Page
	if page <= 0 {
		page = 1
	}
	meta.Page, meta.PerPage = page, perPage
	offset := (page - 1) * perPage

	// --- WHERE builder ---
	wheres := []string{}
	args := []any{}
	idx := 1

	// hanya request yang KAMU kirim (initiator)
	wheres = append(wheres, fmt.Sprintf("taa.initiator_user_id = '%s'", request.UserID))

	// status = PENDING
	if s := strings.TrimSpace(request.Status); s != "" {
		wheres = append(wheres, fmt.Sprintf("taa.status = $%d", idx))
		args = append(args, s)
		idx++
	}

	// aktif saja (hapus jika tidak punya kolom is_active)
	wheres = append(wheres, "COALESCE(taa.is_active, TRUE)")

	whereSQL := "WHERE " + strings.Join(wheres, " AND ")

	// --- COUNT ---
	countSQL := `
		SELECT COUNT(*)
		FROM thread_partner_applications taa
		JOIN thread_partner_types tpt ON tpt.id = taa.thread_partner_type_id
		JOIN threads t               ON t.id  = tpt.thread_id
		` + whereSQL

	if err = tx.QueryRowContext(ctx, countSQL, args...).Scan(&meta.TotalData); err != nil {
		return
	}
	meta.TotalPages = (meta.TotalData + perPage - 1) / perPage

	// --- DATA ---
	args = append(args, perPage, offset)
	limitPos, offsetPos := idx, idx+1

	dataSQL := fmt.Sprintf(`
		SELECT
			taa.id                                                 AS application_id,
			taa.thread_id                                        AS thread_id,
			t.title                                               AS thread_name,
			pt.name                                               AS partner_type_name,
			COALESCE(taa.message, '')                             AS message,
			papp.name                                             AS profile_name,
			papp.name_alias                                       AS profile_name_alias,
			papp.avatar                                           AS profile_avatar,
			COALESCE(taa.updated_at, taa.created_at) AS created_at
		FROM thread_partner_applications taa
		JOIN thread_partner_types tpt ON tpt.id     = taa.thread_partner_type_id
		JOIN partner_types pt        ON pt.id      = tpt.partner_type_id
		JOIN threads t               ON t.id       = tpt.thread_id
		LEFT JOIN profiles papp      ON papp.user_id = taa.applicant_user_id   -- profil target/recipient
		%s
		ORDER BY COALESCE(taa.updated_at, taa.created_at) DESC
		LIMIT $%d OFFSET $%d
	`, whereSQL, limitPos, offsetPos)

	rows, err := tx.QueryContext(ctx, dataSQL, args...)
	if err != nil {
		return
	}
	defer rows.Close()

	out := make([]response.MyThreadCollaborationRequestsRes, 0, perPage)
	for rows.Next() {
		var (
			item response.MyThreadCollaborationRequestsRes
			prof entity.SimpleProfile
		)
		if err = rows.Scan(
			&item.ApplicationID,
			&item.ThreadID,
			&item.ThreadName,
			&item.PartnerTypeName,
			&item.Message,
			&prof.Name,
			&prof.NameAlias,
			&prof.Avatar,
			&item.CreatedAt,
		); err != nil {
			return
		}
		item.Profile = prof
		out = append(out, item)
	}
	if err = rows.Err(); err != nil {
		return
	}

	res = out
	err = tx.Commit()
	return
}

func (r *pgsqlCollaborationRepository) AcceptedThreadCollaborationRequests(ctx context.Context, request *request.AcceptedThreadCollaborationRequestsReq) (res []response.AcceptedThreadCollaborationRequestsRes, meta response.MetaRes, err error) {

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	// --- pagination ---
	perPage := request.PerPage
	if perPage <= 0 {
		perPage = 10
	}
	page := request.Page
	if page <= 0 {
		page = 1
	}
	meta.Page, meta.PerPage = page, perPage
	offset := (page - 1) * perPage

	// --- WHERE builder ---
	wheres := []string{}
	args := []any{}
	idx := 1

	// hanya request yang KAMU inisiasi
	wheres = append(wheres, fmt.Sprintf("taa.initiator_user_id = $%d", idx))
	args = append(args, request.UserID)
	idx++

	// hanya yang sudah diterima
	wheres = append(wheres, "taa.status = 'ACCEPTED'")

	// aktif (hapus jika kolom ini tidak ada)
	wheres = append(wheres, "COALESCE(taa.is_active, TRUE)")

	whereSQL := "WHERE " + strings.Join(wheres, " AND ")

	// --- COUNT ---
	countSQL := `
		SELECT COUNT(*)
		FROM thread_partner_applications taa
		JOIN thread_partner_types tpt ON tpt.id = taa.thread_partner_type_id
		JOIN threads t               ON t.id  = taa.thread_id
		` + whereSQL

	if err = tx.QueryRowContext(ctx, countSQL, args...).Scan(&meta.TotalData); err != nil {
		return
	}
	meta.TotalPages = (meta.TotalData + perPage - 1) / perPage

	// --- DATA ---
	args = append(args, perPage, offset)
	limitPos, offsetPos := idx, idx+1

	dataSQL := fmt.Sprintf(`
		SELECT
			taa.thread_id                                        AS thread_id,
			t.title                                              AS thread_name,
			pt.name                                              AS partner_type_name,
			taa.id                                               AS application_id,
			papp.name                                            AS profile_name,
			papp.name_alias                                      AS profile_name_alias,
			papp.avatar                                          AS profile_avatar,
			COALESCE(taa.updated_at, taa.created_at)             AS created_at
		FROM thread_partner_applications taa
		JOIN thread_partner_types tpt ON tpt.id   = taa.thread_partner_type_id
		JOIN partner_types pt        ON pt.id    = tpt.partner_type_id
		JOIN threads t               ON t.id     = taa.thread_id
		LEFT JOIN profiles papp      ON papp.user_id = taa.initiator_user_id
		%s
		ORDER BY COALESCE(taa.updated_at, taa.created_at) DESC
		LIMIT $%d OFFSET $%d
	`, whereSQL, limitPos, offsetPos)

	rows, err := tx.QueryContext(ctx, dataSQL, args...)
	if err != nil {
		return
	}
	defer rows.Close()

	out := make([]response.AcceptedThreadCollaborationRequestsRes, 0, perPage)
	for rows.Next() {
		var (
			item response.AcceptedThreadCollaborationRequestsRes
			prof entity.SimpleProfile
		)
		if err = rows.Scan(
			&item.ThreadID,
			&item.ThreadName,
			&item.PartnerTypeName,
			&item.ApplicationID,
			&prof.Name,
			&prof.NameAlias,
			&prof.Avatar,
			&item.CreatedAt,
		); err != nil {
			return
		}
		item.Profile = prof
		out = append(out, item)
	}
	if err = rows.Err(); err != nil {
		return
	}

	res = out
	err = tx.Commit()
	return
}

func (r *pgsqlCollaborationRepository) CancelThreadCollaboration(ctx context.Context, request *request.CancelThreadCollaborationReq, threadCollabApplicationPayload *entity.ThreadPartnerApplication,
	threadCollaboratorPayload *entity.ThreadCollaborator) (err error) {

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

	// Check user relation to app id
	var userQ, threadID, threadPartnerTypeID, applicationStatus string
	if request.UserRelation == utils.THREAD_COLLABORATOR {
		userQ = "applicant_user_id"
	} else {
		userQ = "initiator_user_id"
	}

	// Checker collaboration application
	qThreadApp := fmt.Sprintf(`SELECT thread_id, thread_partner_type_id, status
				  FROM thread_partner_applications
				  WHERE is_active = true AND %s = $1 AND id = $2 AND status IN ($3, $4)
				  FOR SHARE`, userQ)

	if err = tx.QueryRowContext(ctx, qThreadApp, request.UserID, threadCollabApplicationPayload.ID, utils.ACCEPTED_APPLICATION_STATUS,
		utils.PENDING_APPLICATION_STATUS).Scan(&threadID, &threadPartnerTypeID, &applicationStatus); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return utils.NewNotFoundError("Thread collaboration application not found")
		}
		return
	}

	// Update application status + cek RowsAffected()
	query := fmt.Sprintf(`UPDATE thread_partner_applications
				SET status = $1,
					updated_at = $2,
					updated_by = $3,
					cancel_reason = $4
				WHERE id = $5 AND is_active = true AND %s = $6`, userQ)

	res, err := tx.ExecContext(ctx, query, threadCollabApplicationPayload.Status, threadCollabApplicationPayload.UpdatedAt,
		threadCollabApplicationPayload.UpdatedBy, threadCollabApplicationPayload.CancelReason, threadCollabApplicationPayload.ID, request.UserID)

	if err != nil {
		return
	}

	if n, _ := res.RowsAffected(); n == 0 {
		err = utils.NewNotFoundError("Application not found or already processed")
		return
	}

	if applicationStatus == utils.ACCEPTED_APPLICATION_STATUS {
		// Update thread collaborator status + cek RowsAffected()
		query = `UPDATE thread_collaborators
				SET status = $1,
					updated_at = $2,
					updated_by = $3,
					left_at = $4
				WHERE thread_partner_application_id = $5 AND is_active = true`

		res, err = tx.ExecContext(ctx, query, threadCollaboratorPayload.Status, threadCollaboratorPayload.UpdatedAt,
			threadCollaboratorPayload.UpdatedBy, threadCollaboratorPayload.LeftAt, request.ApplicationCollaborationID)

		if err != nil {
			return
		}

		if n, _ := res.RowsAffected(); n == 0 {
			err = utils.NewNotFoundError("Thread Collaborator not found or already processed")
			return
		}

		// Revert amount fulfilled
		qPartnerType := `UPDATE thread_partner_types
					SET amount_fulfilled = amount_fulfilled - 1
					WHERE id = $1 AND is_active = true`

		if _, err = tx.ExecContext(ctx, qPartnerType, threadPartnerTypeID); err != nil {
			return
		}
	}

	if err = tx.Commit(); err != nil {
		return
	}

	return
}
