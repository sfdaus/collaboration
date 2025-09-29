package entity

type ThreadCollaborator struct {
	ID                         string `json:"id"`
	ThreadID                   string `json:"thread_id"`
	ThreadPartnerTypeID        string `json:"thread_partner_type_id"`
	ThreadPartnerApplicationID string `json:"thread_partner_application_id"`
	UserID                     string `json:"user_id"`
	Status                     string `json:"status"`
	JoinedAt                   int64  `json:"joined_at"`
	LeftAt                     int64  `json:"left_at"`
	IsActive                   bool   `json:"is_active"`
	CreatedAt                  int64  `json:"created_at"`
	CreatedBy                  string `json:"created_by"`
	UpdatedAt                  int64  `json:"updated_at"`
	UpdatedBy                  string `json:"updated_by"`
	DeletedAt                  int64  `json:"deleted_at"`
}
