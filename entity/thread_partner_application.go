package entity

type ThreadPartnerApplication struct {
	ID                  string  `json:"id"`
	ThreadID            string  `json:"thread_id"`
	ThreadPartnerTypeID string  `json:"thread_partner_type_id"`
	ApplicantUserID     string  `json:"applicant_user_id"`
	InitiatorUserID     string  `json:"initiator_user_id"`
	ConversationID      *string `json:"conversation_id"`
	Message             string  `json:"message"`
	RejectReason        string  `json:"reject_reason"`
	Status              string  `json:"status"`
	IsActive            bool    `json:"is_active"`
	CreatedAt           int64   `json:"created_at"`
	CreatedBy           string  `json:"created_by"`
	UpdatedAt           int64   `json:"updated_at"`
	UpdatedBy           string  `json:"updated_by"`
	DeletedAt           int64   `json:"deleted_at"`
}
