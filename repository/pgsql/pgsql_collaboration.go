package pgsql

import (
	"context"
	"database/sql"
	"errors"
	"github.com/lib/pq"
	"prakarsa-app/entity"
	"prakarsa-app/transport/response"
	"prakarsa-app/utils"
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
	threadCollabApplicationPayload *entity.ThreadPartnerApplication) (res response.ThreadCollaborationApplyRes, err error) {
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
	var ownerID string
	var threadActive bool

	qThread := `SELECT t.user_id AS owner_id, t.is_active
				  FROM threads t
				  WHERE t.id = $1
				  FOR SHARE`

	if err = tx.QueryRowContext(ctx, qThread, threadCollabApplicationPayload.ThreadID).
		Scan(&ownerID, &threadActive); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return res, utils.NewNotFoundError("Thread or Role not found")
		}
		return
	}

	if !threadActive {
		return res, utils.NewNotFoundError("Thread is not active")
	}

	if ownerID == threadCollabApplicationPayload.ApplicantUserID {
		return res, utils.NewForbiddenError("Cannot apply to your own thread")
	}

	// Add owner id to initiator id
	threadCollabApplicationPayload.InitiatorUserID = ownerID

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
			return res, utils.NewNotFoundError("Role isn't available on this thread")
		}
		return
	}

	// Blokir application saat penuh
	if needed == nil {
		return res, utils.NewBadRequestError("Capacity for this role is full")
	} else if fulfilled != nil && *fulfilled >= *needed {
		return res, utils.NewBadRequestError("Capacity for this role is full")
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
			return res, utils.NewForbiddenError("You cannot apply to the same role twice")
		}
		return
	}

	if err = tx.Commit(); err != nil {
		return
	}

	res = response.ThreadCollaborationApplyRes{
		ID:       threadCollabApplicationPayload.ID,
		ThreadID: threadCollabApplicationPayload.ThreadID,
		RoleID:   roleID,
		RoleName: roleName,
		Status:   threadCollabApplicationPayload.Status,
	}

	return
}
